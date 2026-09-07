SELECT CASE WHEN pg_is_in_recovery() THEN 'replica' ELSE 'primary' END,
       current_setting('transaction_read_only'),
       current_setting('synchronous_commit'),
       current_setting('synchronous_standby_names');
