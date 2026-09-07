\set ON_ERROR_STOP on
BEGIN;
SET LOCAL lock_timeout = '5s';

-- Serialize this lab's migration runners, including first-time initialization.
SELECT pg_advisory_xact_lock(20260903, 1);
CREATE SCHEMA IF NOT EXISTS ddia_migrations;
CREATE TABLE IF NOT EXISTS ddia_migrations.applied (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

SELECT EXISTS (
    SELECT 1 FROM ddia_migrations.applied WHERE version = '001_board'
) AS board_applied \gset
\if :board_applied
    \echo '001_board already applied; existing tables and data are unchanged.'
\else
    \ir migrations/001_board.sql
    INSERT INTO ddia_migrations.applied (version) VALUES ('001_board');
\endif
COMMIT;
