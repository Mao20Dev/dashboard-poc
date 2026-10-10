package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
)

var ErrChatLimit = errors.New("chat_limit")
var ErrChatPending = errors.New("chat_pending")
var ErrChatFailed = errors.New("chat_failed")

// Reserve before calling the provider: duplicate clicks and restarts never repeat a paid attempt.
func (s *Store) ReserveChat(ctx context.Context, key, actor string) (json.RawMessage, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71932005)"); err != nil {
		return nil, err
	}
	var status string
	var response json.RawMessage
	err = tx.QueryRow(ctx, "SELECT status,response FROM chat_requests WHERE cache_key=$1 AND actor=$2", key, actor).Scan(&status, &response)
	if err == nil {
		if status == "COMPLETED" {
			return response, nil
		}
		if status == "PENDING" {
			return nil, ErrChatPending
		}
		return nil, ErrChatFailed
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if !s.ChatEnabled {
		return nil, ErrAIRequestsDisabled
	}
	var count, pending int
	err = tx.QueryRow(ctx, `SELECT count(*) FILTER (WHERE created_at >= date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),count(*) FILTER (WHERE status='PENDING' AND created_at>now()-interval '90 seconds') FROM chat_requests WHERE actor=$1`, actor).Scan(&count, &pending)
	if err != nil {
		return nil, err
	}
	if pending > 0 {
		return nil, ErrChatPending
	}
	limit := s.ChatDailyLimit
	if limit <= 0 {
		limit = 20
	}
	if count >= limit {
		return nil, ErrChatLimit
	}
	if _, err = tx.Exec(ctx, "INSERT INTO chat_requests(cache_key,actor,status) VALUES($1,$2,'PENDING')", key, actor); err != nil {
		return nil, err
	}
	return nil, tx.Commit(ctx)
}
func (s *Store) FinishChat(ctx context.Context, key string, response any, ok bool) error {
	status := "FAILED"
	var raw []byte
	var err error
	if ok {
		status = "COMPLETED"
		raw, err = json.Marshal(response)
		if err != nil {
			return err
		}
	}
	_, err = s.Pool.Exec(ctx, "UPDATE chat_requests SET status=$2,response=$3 WHERE cache_key=$1 AND status='PENDING'", key, status, raw)
	return err
}
