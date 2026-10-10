ALTER TABLE analysis_runs ADD COLUMN selected_at timestamptz;
UPDATE analysis_runs SET selected_at=created_at;
ALTER TABLE analysis_runs ALTER COLUMN selected_at SET DEFAULT now();
ALTER TABLE analysis_runs ALTER COLUMN selected_at SET NOT NULL;
ALTER TABLE analysis_runs ADD COLUMN cache_hits integer NOT NULL DEFAULT 0;
ALTER TABLE analysis_runs ADD COLUMN provider_request_started boolean NOT NULL DEFAULT false;
CREATE TABLE analysis_cache (
  input_hash text NOT NULL,
  provider text NOT NULL,
  model text NOT NULL,
  prompt_version text NOT NULL,
  run_id text NOT NULL REFERENCES analysis_runs(id),
  PRIMARY KEY(input_hash,provider,model,prompt_version)
);
-- Reuse paid, validated narratives already generated before the cache existed.
INSERT INTO analysis_cache(input_hash,provider,model,prompt_version,run_id)
SELECT DISTINCT ON (r.input_hash,r.provider,r.model)
  r.input_hash,r.provider,r.model,'analyst-2',r.id
FROM analysis_runs r
WHERE r.status='COMPLETED' AND r.provider IN ('rules','claude','openai')
  AND (SELECT count(*) FROM observations o WHERE o.run_id=r.id) = (r.summary->>'anomalies')::int
  AND (r.provider='rules' OR NOT EXISTS (
    SELECT 1 FROM observations o WHERE o.run_id=r.id AND (
      COALESCE(o.payload#>>'{explanation,source}','')<>r.provider OR
      COALESCE(o.payload#>>'{explanation,narrative_version}','')<>'analyst-2' OR
      COALESCE(o.payload#>>'{explanation,fallback_reason}','')<>'')))
ORDER BY r.input_hash,r.provider,r.model,r.finished_at DESC;
-- A pre-upgrade running paid job may already have sent a request. Never replay it.
UPDATE analysis_runs SET provider_request_started=true WHERE provider IN ('claude','openai') AND status='RUNNING';
INSERT INTO schema_migrations(version) VALUES(3);
