package store

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"energy/internal/domain"
	"energy/internal/engine"
	"energy/internal/explain"
	"energy/internal/ingest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration tests")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	name := "test_energy_" + ID()
	quoted := pgx.Identifier{name}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		admin.Close()
	})
	s := &Store{Pool: pool, Timezone: "America/Bogota", AllowNewAIRequests: true}
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	return s
}
func seed(t *testing.T, s *Store) domain.Input {
	t.Helper()
	in, e := ingest.Load("../../../data", s.Timezone)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.Import(context.Background(), in); e != nil {
		t.Fatal(e)
	}
	return in
}
func finish(t *testing.T, s *Store, r domain.Run) domain.Report {
	t.Helper()
	report, e := engine.Analyze(r.Input, engine.DefaultConfig())
	if e != nil {
		t.Fatal(e)
	}
	var c *explain.Client
	for i := range report.Findings {
		report.Findings[i].Explanation = c.Explain(context.Background(), report.Findings[i])
	}
	if e = s.Complete(context.Background(), r, report); e != nil {
		t.Fatal(e)
	}
	return report
}
func TestImportIdempotentAndConflictsAtomic(t *testing.T) {
	s := testStore(t)
	in := seed(t, s)
	ctx := context.Background()
	if e := s.Import(ctx, in); e != nil {
		t.Fatal(e)
	}
	got, e := s.Input(ctx)
	if e != nil || len(got.Readings) != 4032 {
		t.Fatal("duplicate import", e, len(got.Readings))
	}
	in.Readings[0].Consumption++
	if e = s.Import(ctx, in); e == nil {
		t.Fatal("conflicting import accepted")
	}
	got, _ = s.Input(ctx)
	if got.Readings[0].Consumption == in.Readings[0].Consumption {
		t.Fatal("failed import changed persisted data")
	}
}
func TestConcurrentEnqueueAndClaim(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	var wg sync.WaitGroup
	ids := make(chan string, 4)
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := s.Enqueue(ctx)
			if e != nil {
				errs <- e
			} else {
				ids <- r.ID
			}
		}()
	}
	wg.Wait()
	close(ids)
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
	unique := map[string]bool{}
	for id := range ids {
		unique[id] = true
	}
	if len(unique) != 1 {
		t.Fatal("duplicate active jobs", unique)
	}
	r, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Claim(ctx); !errors.Is(e, pgx.ErrNoRows) {
		t.Fatal("double claim", e)
	}
	if r.Attempts != 1 {
		t.Fatal(r)
	}
}
func TestLeaseRecoveryFencesStaleWorker(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	_, _ = s.Enqueue(ctx)
	old, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.Pool.Exec(ctx, "UPDATE analysis_runs SET lease_until=now()-interval '1 second' WHERE id=$1", old.ID)
	if e != nil {
		t.Fatal(e)
	}
	recovered, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if recovered.Lease == old.Lease || recovered.Attempts != 2 {
		t.Fatal("lease not renewed")
	}
	if e = s.Complete(ctx, old, domain.Report{}); !errors.Is(e, ErrConflict) {
		t.Fatal("stale worker could commit", e)
	}
	finish(t, s, recovered)
}
func TestRerunPreservesActionsAndHistoricalEvidence(t *testing.T) {
	s := testStore(t)
	seed(t, s)
	ctx := context.Background()
	_, _ = s.Enqueue(ctx)
	r, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	finish(t, s, r)
	fs, e := s.Findings(ctx)
	if e != nil || len(fs) != 4 {
		t.Fatal(e, len(fs))
	}
	f := fs[0]
	if e = s.Act(ctx, f.ID, "INVESTIGATE", "Operación consultada", "tester", 1); e != nil {
		t.Fatal(e)
	}
	if e = s.Act(ctx, f.ID, "DISMISS", "Otra acción concurrente", "other", 1); !errors.Is(e, ErrConflict) {
		t.Fatal("stale action accepted", e)
	}
	cached, e := s.Enqueue(ctx)
	if e != nil || cached.ID != r.ID || cached.CacheHits != 1 {
		t.Fatal("did not reuse completed run", cached.ID, e)
	}
	// A new event changes the evidence, legitimately admitting a new snapshot.
	if _, e = s.Pool.Exec(ctx, "INSERT INTO events VALUES('new-context','M-101',now(),'MAINTENANCE','Nuevo contexto',1)"); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Enqueue(ctx); e != nil {
		t.Fatal(e)
	}
	next, e := s.Claim(ctx)
	if e != nil {
		t.Fatal(e)
	}
	finish(t, s, next)
	got, e := s.Finding(ctx, f.ID)
	if e != nil || got.Status != "INVESTIGATING" || got.Version != 2 {
		t.Fatal("rerun erased state", got, e)
	}
	history, e := s.History(ctx, f.ID)
	if e != nil || len(history) != 2 || history[1].Status != "OPEN" {
		t.Fatal("historical snapshot was changed", e)
	}
	acts, e := s.Actions(ctx, f.ID)
	if e != nil || len(acts) != 1 {
		t.Fatal("audit trail lost", e)
	}
}
func TestExpiredSessionAndLogout(t *testing.T) {
	s := testStore(t)
	ctx := context.Background()
	if e := s.EnsureUser(ctx, "test@local", "test-password-123"); e != nil {
		t.Fatal(e)
	}
	token, e := s.Login(ctx, "test@local", "test-password-123")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.Session(ctx, token); e != nil {
		t.Fatal(e)
	}
	_, _ = s.Pool.Exec(ctx, "UPDATE sessions SET expires_at=$1", time.Now().Add(-time.Minute))
	if _, e = s.Session(ctx, token); e == nil {
		t.Fatal("expired session accepted")
	}
	token, _ = s.Login(ctx, "test@local", "test-password-123")
	_ = s.Logout(ctx, token)
	if _, e = s.Session(ctx, token); e == nil {
		t.Fatal("revoked session accepted")
	}
}
func TestTransitions(t *testing.T) {
	if _, e := NextStatus("OPEN", "RESOLVE"); e == nil {
		t.Fatal("resolution without investigation")
	}
	if next, e := NextStatus("INVESTIGATING", "RESOLVE"); e != nil || next != "RESOLVED" {
		t.Fatal(next, e)
	}
}
