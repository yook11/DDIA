-- A received WAL record is not necessarily applied. NULL is not evidence of catch-up.
SELECT COALESCE(pg_last_wal_replay_lsn() >= :'required_lsn'::pg_lsn, false);
