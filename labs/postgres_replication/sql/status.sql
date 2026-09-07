SELECT pg_is_in_recovery() AS is_replica \gset
\if :is_replica
SELECT current_database() AS database, 'replica' AS role,
       pg_last_wal_receive_lsn() AS received_lsn,
       pg_last_wal_replay_lsn() AS applied_lsn,
       pg_get_wal_replay_pause_state() AS replay_state;
\else
SELECT current_database() AS database, 'primary' AS role,
       pg_current_wal_lsn() AS written_lsn,
       pg_current_wal_flush_lsn() AS flushed_lsn;
SELECT application_name, state, sync_state, sent_lsn, flush_lsn, replay_lsn
FROM pg_stat_replication ORDER BY application_name;
SELECT slot_name, active, wal_status
FROM pg_replication_slots ORDER BY slot_name;
\endif
