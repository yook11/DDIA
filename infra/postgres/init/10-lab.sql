-- These passwords match compose.yaml and are intentionally local-lab-only.
CREATE ROLE ddia_app LOGIN PASSWORD 'ddia_local_app'
    NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION;
CREATE ROLE ddia_replicator LOGIN REPLICATION PASSWORD 'ddia_local_replication'
    NOSUPERUSER NOCREATEDB NOCREATEROLE;

CREATE SCHEMA lab AUTHORIZATION ddia_app;
SET ROLE ddia_app;
CREATE TABLE lab.posts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    body text NOT NULL
);
RESET ROLE;

-- Reserve WAL immediately: concurrent base backups must not lose the other's
-- starting segment before its streaming connection attaches to the slot.
SELECT pg_create_physical_replication_slot('replica1', true);
SELECT pg_create_physical_replication_slot('replica2', true);
