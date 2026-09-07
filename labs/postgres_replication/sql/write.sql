-- psql: set body first. Capture a conservative WAL token AFTER COMMIT, not inside INSERT.
BEGIN;
SET LOCAL synchronous_commit = on;
INSERT INTO lab.posts (body) VALUES (:'body') RETURNING id \gset
COMMIT;
SELECT :id AS id, pg_current_wal_lsn() AS required_lsn;
