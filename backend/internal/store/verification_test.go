package store

import (
	"context"
	"energy/internal/domain"
	"errors"
	"testing"
)

func TestVerificationIsAuditedAndBoundToAnExistingRecommendation(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	if _, err := s.Enqueue(ctx); err != nil {
		t.Fatal(err)
	}
	run, err := s.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	report := finish(t, s, run)
	f := report.Findings[0]
	v := &domain.Verification{RunID: run.ID, StepCode: f.Explanation.Actions[0].Code, Status: "CONFIRMED"}
	if err = s.ActWithVerification(ctx, f.ID, "NOTE", "Resultado de prueba aislada", "test@local", 1, v); err != nil {
		t.Fatal(err)
	}
	actions, err := s.Actions(ctx, f.ID)
	if err != nil || len(actions) != 1 || actions[0].Verification == nil || *actions[0].Verification != *v {
		t.Fatal("verification lost", actions, err)
	}
	got, err := s.Finding(ctx, f.ID)
	if err != nil || got.Version != 2 || got.Status != "OPEN" {
		t.Fatal("verification incorrectly changed case", err)
	}
	if err = s.ActWithVerification(ctx, f.ID, "NOTE", "Concurrent stale note", "test@local", 1, v); !errors.Is(err, ErrConflict) {
		t.Fatal("stale version accepted", err)
	}
	for _, bad := range []domain.Verification{{RunID: run.ID, StepCode: "invented", Status: "CONFIRMED"}, {RunID: "other", StepCode: v.StepCode, Status: "PENDING"}, {RunID: run.ID, StepCode: v.StepCode, Status: "BAD"}} {
		if err = s.ActWithVerification(ctx, f.ID, "NOTE", "Invalid test recommendation", "test@local", 2, &bad); !errors.Is(err, ErrConflict) {
			t.Fatal("invalid verification accepted", err)
		}
	}
	v.Status = "PENDING"
	if err = s.ActWithVerification(ctx, f.ID, "NOTE", "Reopened test check", "test@local", 2, v); err != nil {
		t.Fatal(err)
	}
	actions, err = s.Actions(ctx, f.ID)
	if err != nil || len(actions) != 2 || actions[0].Verification.Status != "CONFIRMED" || actions[1].Verification.Status != "PENDING" {
		t.Fatal("history mutated", err)
	}
}
