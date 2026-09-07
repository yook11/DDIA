-- Same catalog assertions on the primary and both read-only replicas.
DO $$
BEGIN
    IF (SELECT count(*) FROM information_schema.tables
        WHERE table_schema = 'board' AND table_name IN ('users', 'threads', 'posts')
          AND table_type = 'BASE TABLE') <> 3 THEN
        RAISE EXCEPTION 'Missing board tables';
    END IF;
    IF (SELECT count(*) FROM information_schema.columns
        WHERE table_schema = 'board' AND table_name IN ('users', 'threads', 'posts')
          AND column_name = 'id' AND data_type = 'bigint'
          AND is_identity = 'YES' AND identity_generation = 'ALWAYS') <> 3 THEN
        RAISE EXCEPTION 'All three IDs must be bigint GENERATED ALWAYS identities';
    END IF;
    IF (SELECT count(*) FROM pg_constraint
        WHERE contype = 'p' AND conrelid IN (
            'board.users'::regclass, 'board.threads'::regclass, 'board.posts'::regclass
        )) <> 3 THEN
        RAISE EXCEPTION 'Missing primary keys';
    END IF;
    IF (SELECT count(*) FROM information_schema.columns
        WHERE table_schema = 'board' AND table_name IN ('users', 'threads', 'posts')
          AND column_name = 'created_at' AND data_type = 'timestamp with time zone'
          AND is_nullable = 'NO' AND column_default IS NOT NULL) <> 3 THEN
        RAISE EXCEPTION 'Missing creation timestamps';
    END IF;
    IF to_regclass('board.posts_thread_id_id_idx') IS NULL THEN
        RAISE EXCEPTION 'Missing thread lookup index';
    END IF;
END
$$;
