-- Must fail with SQLSTATE 25006 on a replica. If accidentally run on a primary,
-- ROLLBACK prevents this probe from leaving a row behind.
BEGIN;
INSERT INTO lab.posts (body) VALUES ('read-only probe');
ROLLBACK;
