package store

import (
	"context"
	"encoding/json"
	"energy/internal/domain"
	"fmt"
	"strings"
)

func (s *Store) Findings(ctx context.Context) ([]domain.Finding, error) {
	rows, err := s.Pool.Query(ctx, `SELECT o.payload,i.status,i.version FROM observations o JOIN incidents i ON i.id=o.incident_id WHERE o.run_id=(SELECT id FROM analysis_runs WHERE status='COMPLETED' ORDER BY selected_at DESC,id DESC LIMIT 1) ORDER BY (o.payload->>'priority')::int DESC,i.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Finding{}
	for rows.Next() {
		var raw []byte
		var status string
		var version int
		if err = rows.Scan(&raw, &status, &version); err != nil {
			return nil, err
		}
		var f domain.Finding
		if err = json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		f.Status = status
		f.Version = version
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Store) Finding(ctx context.Context, id string) (domain.Finding, error) {
	var raw []byte
	var status string
	var version int
	var f domain.Finding
	err := s.Pool.QueryRow(ctx, "SELECT o.payload,i.status,i.version FROM observations o JOIN incidents i ON i.id=o.incident_id WHERE i.id=$1 AND o.run_id=(SELECT id FROM analysis_runs WHERE status='COMPLETED' ORDER BY selected_at DESC,id DESC LIMIT 1)", id).Scan(&raw, &status, &version)
	if err != nil {
		return f, err
	}
	err = json.Unmarshal(raw, &f)
	f.Status = status
	f.Version = version
	return f, err
}
func (s *Store) History(ctx context.Context, id string) ([]domain.Finding, error) {
	rows, err := s.Pool.Query(ctx, "SELECT payload FROM observations WHERE incident_id=$1 ORDER BY created_at DESC", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Finding{}
	for rows.Next() {
		var raw []byte
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var f domain.Finding
		if err = json.Unmarshal(raw, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (s *Store) Actions(ctx context.Context, id string) ([]domain.Action, error) {
	rows, err := s.Pool.Query(ctx, "SELECT id,action,note,actor,from_status,to_status,created_at,verification FROM actions WHERE incident_id=$1 ORDER BY created_at,id", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Action{}
	for rows.Next() {
		var a domain.Action
		if err = rows.Scan(&a.ID, &a.Action, &a.Note, &a.Actor, &a.From, &a.To, &a.CreatedAt, &a.Verification); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func NextStatus(from, action string) (string, error) {
	switch action {
	case "NOTE":
		return from, nil
	case "INVESTIGATE":
		if from == "OPEN" {
			return "INVESTIGATING", nil
		}
	case "RESOLVE":
		if from == "INVESTIGATING" {
			return "RESOLVED", nil
		}
	case "DISMISS":
		if from == "OPEN" || from == "INVESTIGATING" {
			return "DISMISSED", nil
		}
	case "REOPEN":
		if from == "RESOLVED" || from == "DISMISSED" {
			return "OPEN", nil
		}
	}
	return "", ErrConflict
}
func (s *Store) Act(ctx context.Context, id, action, note, actor string, version int) error {
	return s.ActWithVerification(ctx, id, action, note, actor, version, nil)
}
func (s *Store) ActWithVerification(ctx context.Context, id, action, note, actor string, version int, verification *domain.Verification) error {
	if len(strings.TrimSpace(note)) < 5 || len(note) > 2000 {
		return fmt.Errorf("note must have 5 to 2000 characters")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var from string
	var current int
	if err = tx.QueryRow(ctx, "SELECT status,version FROM incidents WHERE id=$1 FOR UPDATE", id).Scan(&from, &current); err != nil {
		return err
	}
	if current != version {
		return ErrConflict
	}
	if verification != nil {
		if action != "NOTE" || (verification.Status != "CONFIRMED" && verification.Status != "DISCARDED" && verification.Status != "PENDING") {
			return ErrConflict
		}
		var raw []byte
		if err = tx.QueryRow(ctx, "SELECT payload FROM observations WHERE run_id=$1 AND incident_id=$2", verification.RunID, id).Scan(&raw); err != nil {
			return ErrConflict
		}
		var finding domain.Finding
		if err = json.Unmarshal(raw, &finding); err != nil {
			return err
		}
		valid := false
		for _, step := range finding.Explanation.Actions {
			if step.Code == verification.StepCode {
				valid = true
				break
			}
		}
		if !valid {
			return ErrConflict
		}
	}
	to, err := NextStatus(from, action)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE incidents SET status=$2,version=version+1 WHERE id=$1", id, to); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO actions(id,incident_id,action,note,actor,from_status,to_status,verification) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", ID(), id, action, note, actor, from, to, verification); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
