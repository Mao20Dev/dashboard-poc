package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"energy/internal/domain"
	"energy/internal/engine"
	"energy/internal/explain"
)

//go:embed schema.sql
var schema string

//go:embed migration2.sql
var migration2 string

//go:embed migration3.sql
var migration3 string

//go:embed migration4.sql
var migration4 string

//go:embed migration5.sql
var migration5 string
var ErrAIRequestsDisabled = errors.New("ai_requests_disabled")
var ErrCacheMiss = errors.New("cache_miss")
var ErrConflict = errors.New("conflict")
var ErrNotFound = errors.New("not found")

type Store struct {
	AllowNewAIRequests bool
	ChatEnabled        bool
	ChatDailyLimit     int
	Pool               *pgxpool.Pool
	Timezone           string
}

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func Open(ctx context.Context, url, tz string) (*Store, error) {
	p, e := pgxpool.New(ctx, url)
	if e != nil {
		return nil, e
	}
	if e = p.Ping(ctx); e != nil {
		p.Close()
		return nil, e
	}
	return &Store{Pool: p, Timezone: tz}, nil
}
func (s *Store) Migrate(ctx context.Context) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71932001)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, schema); err != nil {
		return err
	}
	var migrated bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=2)").Scan(&migrated); err != nil {
		return err
	}
	if !migrated {
		if _, err = tx.Exec(ctx, migration2); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=3)").Scan(&migrated); err != nil {
		return err
	}
	if !migrated {
		if _, err = tx.Exec(ctx, migration3); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=4)").Scan(&migrated); err != nil {
		return err
	}
	if !migrated {
		if _, err = tx.Exec(ctx, migration4); err != nil {
			return err
		}
	}
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=5)").Scan(&migrated); err != nil {
		return err
	}
	if !migrated {
		if _, err = tx.Exec(ctx, migration5); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Import(ctx context.Context, in domain.Input) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	for _, r := range in.Readings {
		if _, err = tx.Exec(ctx, "INSERT INTO meters(id) VALUES($1) ON CONFLICT DO NOTHING", r.MeterID); err != nil {
			return err
		}
		// Identical retries are accepted; a conflicting measurement aborts the entire import.
		tag, e := tx.Exec(ctx, `INSERT INTO readings VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(meter_id,timestamp) DO UPDATE SET status=readings.status WHERE readings.consumption_kwh=EXCLUDED.consumption_kwh AND readings.voltage_v=EXCLUDED.voltage_v AND readings.current_a=EXCLUDED.current_a AND readings.power_factor=EXCLUDED.power_factor AND readings.status=EXCLUDED.status`, r.MeterID, r.Timestamp, r.Consumption, r.Voltage, r.Current, r.PowerFactor, r.Status)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("conflicting reading %s %s", r.MeterID, r.Timestamp)
		}
	}
	for _, e := range in.Events {
		if _, err = tx.Exec(ctx, "INSERT INTO events VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING", e.ID, e.MeterID, e.Timestamp, e.Type, e.Description, e.DurationHours); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (s *Store) Input(ctx context.Context) (domain.Input, error) {
	in := domain.Input{Timezone: s.Timezone, Readings: []domain.Reading{}, Events: []domain.Event{}}
	tx, err := s.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return in, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, "SELECT meter_id,timestamp,consumption_kwh,voltage_v,current_a,power_factor,status FROM readings ORDER BY meter_id,timestamp")
	if err != nil {
		return in, err
	}
	for rows.Next() {
		var r domain.Reading
		if err = rows.Scan(&r.MeterID, &r.Timestamp, &r.Consumption, &r.Voltage, &r.Current, &r.PowerFactor, &r.Status); err != nil {
			rows.Close()
			return in, err
		}
		in.Readings = append(in.Readings, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return in, err
	}
	rows, err = tx.Query(ctx, "SELECT id,meter_id,timestamp,type,description,duration_hours FROM events ORDER BY id")
	if err != nil {
		return in, err
	}
	for rows.Next() {
		var e domain.Event
		if err = rows.Scan(&e.ID, &e.MeterID, &e.Timestamp, &e.Type, &e.Description, &e.DurationHours); err != nil {
			rows.Close()
			return in, err
		}
		in.Events = append(in.Events, e)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return in, err
	}
	return in, tx.Commit(ctx)
}
func (s *Store) EnsureUser(ctx context.Context, email, password string) error {
	if len(password) < 12 {
		return fmt.Errorf("DEMO_PASSWORD must have at least 12 characters")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.Pool.Exec(ctx, "INSERT INTO users(id,email,password_hash) VALUES($1,$2,$3) ON CONFLICT(email) DO NOTHING", ID(), email, string(hash))
	return err
}
func tokenHash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func (s *Store) Login(ctx context.Context, email, password string) (string, error) {
	var uid, hash string
	err := s.Pool.QueryRow(ctx, "SELECT id,password_hash FROM users WHERE email=$1", email).Scan(&uid, &hash)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return "", ErrNotFound
	}
	token := ID() + ID()
	_, err = s.Pool.Exec(ctx, "INSERT INTO sessions VALUES($1,$2,$3)", tokenHash(token), uid, time.Now().Add(12*time.Hour))
	return token, err
}
func (s *Store) Session(ctx context.Context, token string) (string, error) {
	var email string
	err := s.Pool.QueryRow(ctx, "SELECT u.email FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=$1 AND s.expires_at>now()", tokenHash(token)).Scan(&email)
	return email, err
}
func (s *Store) Logout(ctx context.Context, token string) error {
	_, err := s.Pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash=$1", tokenHash(token))
	return err
}

const runColumns = `id,status,stage,input_hash,engine_version,created_at,started_at,finished_at,attempts,error,summary,input,COALESCE(lease_token,''),provider,model,selected_at,cache_hits`

func scanRun(row pgx.Row) (domain.Run, error) {
	var r domain.Run
	var input, summary []byte
	err := row.Scan(&r.ID, &r.Status, &r.Stage, &r.InputHash, &r.EngineVersion, &r.CreatedAt, &r.StartedAt, &r.FinishedAt, &r.Attempts, &r.Error, &summary, &input, &r.Lease, &r.Provider, &r.Model, &r.SelectedAt, &r.CacheHits)
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(input, &r.Input); err != nil {
		return r, err
	}
	err = json.Unmarshal(summary, &r.Summary)
	return r, err
}
func (s *Store) Enqueue(ctx context.Context) (domain.Run, error) {
	return s.EnqueueProvider(ctx, "rules", "")
}
func (s *Store) EnqueueProvider(ctx context.Context, provider, model string) (domain.Run, error) {
	return s.SelectAnalysis(ctx, provider, model, false)
}

// SelectAnalysis never expires a result: identical evidence, model and narrative contract reuse it.
func (s *Store) SelectAnalysis(ctx context.Context, provider, model string, cachedOnly bool) (domain.Run, error) {
	in, err := s.Input(ctx)
	if err != nil {
		return domain.Run{}, err
	}
	if len(in.Readings) == 0 {
		return domain.Run{}, fmt.Errorf("no readings")
	}
	hash := engine.Hash(in, engine.DefaultConfig())
	raw, err := json.Marshal(in)
	if err != nil {
		return domain.Run{}, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return domain.Run{}, err
	}
	defer tx.Rollback(ctx)
	// Serialize cache lookup + admission, including jobs that finish concurrently.
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(71932002)"); err != nil {
		return domain.Run{}, err
	}
	var id string
	err = tx.QueryRow(ctx, "SELECT run_id FROM analysis_cache WHERE input_hash=$1 AND provider=$2 AND model=$3 AND prompt_version=$4", hash, provider, model, explain.PromptVersion).Scan(&id)
	if err == nil {
		run, e := scanRun(tx.QueryRow(ctx, "UPDATE analysis_runs SET selected_at=clock_timestamp(),cache_hits=cache_hits+1 WHERE id=$1 RETURNING "+runColumns, id))
		if e != nil {
			return run, e
		}
		return run, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Run{}, err
	}
	if cachedOnly {
		return domain.Run{}, ErrCacheMiss
	}
	if provider != "rules" && !s.AllowNewAIRequests {
		return domain.Run{}, ErrAIRequestsDisabled
	}
	run, err := scanRun(tx.QueryRow(ctx, "INSERT INTO analysis_runs(id,input_hash,engine_version,input,status,provider,model) VALUES($1,$2,$3,$4,'PENDING',$5,$6) ON CONFLICT(input_hash,provider,model) WHERE status IN ('PENDING','RUNNING') DO NOTHING RETURNING "+runColumns, ID(), hash, engine.Version, raw, provider, model))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Run{}, ErrConflict
	}
	if err != nil {
		return run, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO analysis_cache VALUES($1,$2,$3,$4,$5)", hash, provider, model, explain.PromptVersion, run.ID); err != nil {
		return run, err
	}
	return run, tx.Commit(ctx)
}
func (s *Store) Run(ctx context.Context, id string) (domain.Run, error) {
	return scanRun(s.Pool.QueryRow(ctx, "SELECT "+runColumns+" FROM analysis_runs WHERE id=$1", id))
}
func (s *Store) LatestRun(ctx context.Context) (domain.Run, error) {
	return scanRun(s.Pool.QueryRow(ctx, "SELECT "+runColumns+" FROM analysis_runs ORDER BY selected_at DESC,id DESC LIMIT 1"))
}
func (s *Store) Claim(ctx context.Context) (domain.Run, error) {
	if _, err := s.Pool.Exec(ctx, "UPDATE analysis_runs SET status='FAILED',error='retry_limit',finished_at=now() WHERE status='RUNNING' AND lease_until<now() AND attempts>=3"); err != nil {
		return domain.Run{}, err
	}
	return scanRun(s.Pool.QueryRow(ctx, `UPDATE analysis_runs SET status='RUNNING',stage='READINGS',attempts=attempts+1,started_at=now(),lease_token=$1,lease_until=now()+interval '120 seconds' WHERE id=(SELECT id FROM analysis_runs WHERE status='PENDING' OR (status='RUNNING' AND lease_until<now() AND attempts<3) ORDER BY created_at FOR UPDATE SKIP LOCKED LIMIT 1) RETURNING `+runColumns, ID()))
}
func (s *Store) Stage(ctx context.Context, r domain.Run, stage string) error {
	tag, err := s.Pool.Exec(ctx, "UPDATE analysis_runs SET stage=$3 WHERE id=$1 AND lease_token=$2 AND status='RUNNING' AND lease_until>now()", r.ID, r.Lease, stage)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}
func (s *Store) Fail(ctx context.Context, r domain.Run, code string) error {
	_, err := s.Pool.Exec(ctx, "UPDATE analysis_runs SET status='FAILED',error=$3,finished_at=now() WHERE id=$1 AND lease_token=$2 AND status='RUNNING'", r.ID, r.Lease, code)
	return err
}
func (s *Store) Complete(ctx context.Context, r domain.Run, report domain.Report) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id string
	if err = tx.QueryRow(ctx, "SELECT id FROM analysis_runs WHERE id=$1 AND lease_token=$2 AND status='RUNNING' AND lease_until>now() FOR UPDATE", r.ID, r.Lease).Scan(&id); err != nil {
		return ErrConflict
	}
	high := 0
	for i := range report.Findings {
		f := &report.Findings[i]
		f.RunID = r.ID
		if _, err = tx.Exec(ctx, "INSERT INTO incidents(id,meter_id) VALUES($1,$2) ON CONFLICT DO NOTHING", f.ID, f.MeterID); err != nil {
			return err
		}
		if err = tx.QueryRow(ctx, "SELECT status,version FROM incidents WHERE id=$1", f.ID).Scan(&f.Status, &f.Version); err != nil {
			return err
		}
		raw, e := json.Marshal(f)
		if e != nil {
			return e
		}
		if _, err = tx.Exec(ctx, "INSERT INTO observations(run_id,incident_id,payload) VALUES($1,$2,$3)", r.ID, f.ID, raw); err != nil {
			return err
		}
		if f.Severity == "HIGH" {
			high++
		}
	}
	summary, _ := json.Marshal(map[string]any{"anomalies": len(report.Findings), "high_priority": high, "issues": report.Issues, "readings": len(r.Input.Readings), "timezone": r.Input.Timezone, "config": engine.DefaultConfig()})
	if _, err = tx.Exec(ctx, "UPDATE analysis_runs SET status='COMPLETED',stage='COMPLETED',finished_at=now(),summary=$2 WHERE id=$1", r.ID, summary); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// ReserveProviderRequest is durable and fenced by the worker lease. A crash after
// reservation sacrifices a response rather than risk a second bill on recovery.
func (s *Store) ReserveProviderRequest(ctx context.Context, r domain.Run) error {
	if !s.AllowNewAIRequests {
		return ErrAIRequestsDisabled
	}
	tag, err := s.Pool.Exec(ctx, "UPDATE analysis_runs SET provider_request_started=true WHERE id=$1 AND lease_token=$2 AND status='RUNNING' AND lease_until>now() AND NOT provider_request_started", r.ID, r.Lease)
	if err == nil && tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return err
}
