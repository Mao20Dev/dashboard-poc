ALTER TABLE actions ADD COLUMN verification jsonb;
INSERT INTO schema_migrations(version) VALUES(4);
