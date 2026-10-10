CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS meters(id text PRIMARY KEY, created_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE IF NOT EXISTS readings(
 meter_id text NOT NULL REFERENCES meters(id), timestamp timestamptz NOT NULL,
 consumption_kwh double precision NOT NULL CHECK(consumption_kwh>=0),
 voltage_v double precision NOT NULL CHECK(voltage_v>=0), current_a double precision NOT NULL CHECK(current_a>=0),
 power_factor double precision NOT NULL CHECK(power_factor BETWEEN 0 AND 1), status text NOT NULL,
 PRIMARY KEY(meter_id,timestamp)
);
CREATE INDEX IF NOT EXISTS readings_time_idx ON readings(timestamp);
CREATE TABLE IF NOT EXISTS events(id text PRIMARY KEY, meter_id text NOT NULL REFERENCES meters(id), timestamp timestamptz NOT NULL, type text NOT NULL, description text NOT NULL, duration_hours double precision NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS users(id text PRIMARY KEY, email text UNIQUE NOT NULL, password_hash text NOT NULL);
CREATE TABLE IF NOT EXISTS sessions(token_hash text PRIMARY KEY, user_id text NOT NULL REFERENCES users(id), expires_at timestamptz NOT NULL);
CREATE INDEX IF NOT EXISTS session_expiry_idx ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS analysis_runs(
 id text PRIMARY KEY, input_hash text NOT NULL, engine_version text NOT NULL, input jsonb NOT NULL,
 status text NOT NULL CHECK(status IN ('PENDING','RUNNING','COMPLETED','FAILED')),
 stage text NOT NULL DEFAULT 'QUEUED', attempts integer NOT NULL DEFAULT 0,
 lease_token text, lease_until timestamptz, created_at timestamptz NOT NULL DEFAULT now(),
 started_at timestamptz, finished_at timestamptz, error text NOT NULL DEFAULT '', summary jsonb NOT NULL DEFAULT '{}'
);
CREATE TABLE IF NOT EXISTS incidents(
 id text PRIMARY KEY, meter_id text NOT NULL REFERENCES meters(id),
 status text NOT NULL DEFAULT 'OPEN' CHECK(status IN ('OPEN','INVESTIGATING','RESOLVED','DISMISSED')),
 version integer NOT NULL DEFAULT 1, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS observations(
 run_id text NOT NULL REFERENCES analysis_runs(id), incident_id text NOT NULL REFERENCES incidents(id),
 payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(run_id,incident_id)
);
CREATE INDEX IF NOT EXISTS observations_incident_idx ON observations(incident_id,created_at DESC);
CREATE TABLE IF NOT EXISTS actions(
 id text PRIMARY KEY, incident_id text NOT NULL REFERENCES incidents(id), action text NOT NULL,
 note text NOT NULL, actor text NOT NULL, from_status text NOT NULL, to_status text NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
INSERT INTO schema_migrations(version) VALUES(1) ON CONFLICT DO NOTHING;
