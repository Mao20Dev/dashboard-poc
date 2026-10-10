ALTER TABLE analysis_runs ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT 'legacy';
ALTER TABLE analysis_runs ADD COLUMN IF NOT EXISTS model text NOT NULL DEFAULT '';
DROP INDEX IF EXISTS one_active_input_idx;
CREATE UNIQUE INDEX IF NOT EXISTS one_active_provider_input_idx ON analysis_runs(input_hash,provider,model) WHERE status IN ('PENDING','RUNNING');
INSERT INTO schema_migrations(version) VALUES(2);
