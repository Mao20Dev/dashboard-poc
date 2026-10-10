package store

import (
	"context"
	"energy/internal/domain"
	"energy/internal/engine"
	"errors"
	"sync"
	"testing"
)

func TestCacheProviderSwitchAndDurableCostGate(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	ids := map[string]string{}
	for _, provider := range []string{"claude", "openai"} {
		queued, err := s.EnqueueProvider(ctx, provider, "model")
		if err != nil {
			t.Fatal(err)
		}
		r, err := s.Claim(ctx)
		if err != nil || queued.ID != r.ID {
			t.Fatal(err)
		}
		if err = s.ReserveProviderRequest(ctx, r); err != nil {
			t.Fatal(err)
		}
		if err = s.ReserveProviderRequest(ctx, r); !errors.Is(err, ErrConflict) {
			t.Fatal("duplicate paid reservation", err)
		}
		report, err := engine.Analyze(r.Input, engine.DefaultConfig())
		if err != nil {
			t.Fatal(err)
		}
		for i := range report.Findings {
			report.Findings[i].Explanation = domain.Explanation{Source: provider}
		}
		if err = s.Complete(ctx, r, report); err != nil {
			t.Fatal(err)
		}
		ids[provider] = r.ID
	}
	// Simulate reopening the app in cache-only mode, with existing database storage.
	reopened := &Store{Pool: s.Pool, Timezone: s.Timezone}
	for _, provider := range []string{"claude", "openai", "claude"} {
		r, err := reopened.SelectAnalysis(ctx, provider, "model", true)
		if err != nil || r.ID != ids[provider] || r.CacheHits < 1 {
			t.Fatal("cache missed", r.ID, err)
		}
		latest, err := s.LatestRun(ctx)
		if err != nil || latest.ID != r.ID {
			t.Fatal("wrong selected run")
		}
		fs, err := s.Findings(ctx)
		if err != nil || len(fs) != 4 {
			t.Fatal(err)
		}
		for _, f := range fs {
			detail, err := s.Finding(ctx, f.ID)
			if err != nil || f.Explanation.Source != provider || detail.RunID != r.ID {
				t.Fatal("detail kept other provider", err)
			}
		}
	}
	if _, err := reopened.EnqueueProvider(ctx, "openai", "changed-model"); !errors.Is(err, ErrAIRequestsDisabled) {
		t.Fatal("cost gate failed", err)
	}
	if _, err := reopened.SelectAnalysis(ctx, "openai", "changed-model", true); !errors.Is(err, ErrCacheMiss) {
		t.Fatal("cache-only admitted work", err)
	}
	var count int
	if err := s.Pool.QueryRow(ctx, "SELECT count(*) FROM analysis_runs").Scan(&count); err != nil || count != 2 {
		t.Fatal("new run billed", count, err)
	}
}

func TestConcurrentCompletedCacheNeverCreatesNewJobs(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	queued, err := s.Enqueue(ctx)
	if err != nil {
		t.Fatal(err)
	}
	r, err := s.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	finish(t, s, r)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cached, e := s.Enqueue(ctx)
			if e != nil || cached.ID != queued.ID || cached.Status != "COMPLETED" {
				t.Error("duplicate", e)
			}
		}()
	}
	wg.Wait()
	var count int
	if err = s.Pool.QueryRow(ctx, "SELECT count(*) FROM analysis_runs").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE analysis_cache SET prompt_version='old-contract'"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.SelectAnalysis(ctx, "rules", "", true); !errors.Is(err, ErrCacheMiss) {
		t.Fatal("old contract reused", err)
	}
	newRun, err := s.Enqueue(ctx)
	if err != nil || newRun.ID == queued.ID {
		t.Fatal("contract change did not invalidate cache", err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE analysis_cache SET prompt_version='other-contract' WHERE run_id=$1", newRun.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Enqueue(ctx); !errors.Is(err, ErrConflict) {
		t.Fatal("active job from different contract reused", err)
	}
}

func TestPaidReservationSurvivesLeaseRecovery(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	if _, err := s.EnqueueProvider(ctx, "openai", "model"); err != nil {
		t.Fatal(err)
	}
	old, err := s.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveProviderRequest(ctx, old); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Pool.Exec(ctx, "UPDATE analysis_runs SET lease_until=now()-interval '1 second' WHERE id=$1", old.ID); err != nil {
		t.Fatal(err)
	}
	recovered, err := s.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ReserveProviderRequest(ctx, recovered); !errors.Is(err, ErrConflict) {
		t.Fatal("replayed paid call", err)
	}
	if err = s.Fail(ctx, recovered, "request_outcome_unknown"); err != nil {
		t.Fatal(err)
	}
	cached, err := s.EnqueueProvider(ctx, "openai", "model")
	if err != nil || cached.ID != old.ID || cached.Status != "FAILED" {
		t.Fatal("failed attempt retried", err)
	}
}

func TestMigrationReusesLastValidPaidNarrative(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	if _, err := s.EnqueueProvider(ctx, "claude", "model"); err != nil {
		t.Fatal(err)
	}
	r, err := s.Claim(ctx)
	if err != nil {
		t.Fatal(err)
	}
	report, err := engine.Analyze(r.Input, engine.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i := range report.Findings {
		report.Findings[i].Explanation = domain.Explanation{Source: "claude", NarrativeVersion: "analyst-2"}
	}
	if err = s.Complete(ctx, r, report); err != nil {
		t.Fatal(err)
	}
	// Historical later run contains fallback output; migration must prefer the
	// older complete paid interpretation rather than spend again or hide it.
	_, err = s.Pool.Exec(ctx, `INSERT INTO analysis_runs(id,input_hash,engine_version,input,status,provider,model,summary,finished_at)
	 SELECT 'later-invalid',input_hash,engine_version,input,status,provider,model,summary,now()+interval '1 minute' FROM analysis_runs WHERE id=$1;
	`, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Pool.Exec(ctx, `INSERT INTO observations(run_id,incident_id,payload) SELECT 'later-invalid',incident_id,jsonb_set(payload,'{explanation,source}','"rules"') FROM observations WHERE run_id=$1`, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the pre-migration schema only inside this test's isolated schema.
	_, err = s.Pool.Exec(ctx, `DROP TABLE analysis_cache; ALTER TABLE analysis_runs DROP COLUMN selected_at,DROP COLUMN cache_hits,DROP COLUMN provider_request_started; DELETE FROM schema_migrations WHERE version=3;`)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	s.AllowNewAIRequests = false
	cached, err := s.SelectAnalysis(ctx, "claude", "model", true)
	if err != nil || cached.ID != r.ID {
		t.Fatal("migration lost paid response", cached.ID, err)
	}
}
