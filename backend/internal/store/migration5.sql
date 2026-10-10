CREATE TABLE chat_requests (
    cache_key text PRIMARY KEY,
    actor text NOT NULL,
    status text NOT NULL CHECK(status IN ('PENDING','COMPLETED','FAILED')),
    created_at timestamptz NOT NULL DEFAULT now(),
    response jsonb
);
CREATE INDEX chat_requests_actor_time ON chat_requests(actor,created_at);
INSERT INTO schema_migrations(version) VALUES(5);
