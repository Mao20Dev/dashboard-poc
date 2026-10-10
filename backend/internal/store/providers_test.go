package store

import (
	"context"
	"testing"
)

func TestProviderJobsAreDistinctAndMigrationPreservesThem(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	a, err := s.EnqueueProvider(ctx, "claude", "sonnet-test")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EnqueueProvider(ctx, "openai", "gpt-test")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID || a.InputHash != b.InputHash {
		t.Fatal("provider must distinguish identical input")
	}
	retry, err := s.EnqueueProvider(ctx, "claude", "sonnet-test")
	if err != nil || retry.ID != a.ID {
		t.Fatal("same provider active job not coalesced", err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal("restart migration fails with two providers", err)
	}
	stored, err := s.Run(ctx, b.ID)
	if err != nil || stored.Provider != "openai" || stored.Model != "gpt-test" {
		t.Fatal("provider identity lost", stored, err)
	}
}
