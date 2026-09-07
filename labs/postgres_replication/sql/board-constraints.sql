-- All fixture rows roll back. Sequences may advance, as in normal failed writes.
BEGIN;
DO $$
DECLARE
    user_id bigint;
    other_user_id bigint;
    thread_id bigint;
    post_id bigint;
    reply_id bigint;
    changed integer;
BEGIN
    IF current_user <> 'ddia_app' OR pg_is_in_recovery() THEN
        RAISE EXCEPTION 'Run constraints test as ddia_app on the primary';
    END IF;

    INSERT INTO board.users (name) VALUES ('db-board-test') RETURNING id INTO user_id;
    INSERT INTO board.users (name) VALUES ('db-board-test') RETURNING id INTO other_user_id;
    IF user_id = other_user_id THEN
        RAISE EXCEPTION 'Generated user IDs collided';
    END IF;
    INSERT INTO board.threads (title, author_id)
        VALUES ('db-board-test', user_id) RETURNING id INTO thread_id;
    INSERT INTO board.posts (thread_id, author_id, body)
        VALUES (thread_id, user_id, 'first post') RETURNING id INTO post_id;
    INSERT INTO board.posts (thread_id, author_id, body, reply_to)
        VALUES (thread_id, other_user_id, 'reply', post_id) RETURNING id INTO reply_id;
    IF post_id = reply_id OR NOT EXISTS (
        SELECT 1 FROM board.posts p
        JOIN board.threads t ON t.id = p.thread_id
        JOIN board.users u ON u.id = p.author_id
        WHERE p.id = reply_id AND p.reply_to = post_id AND u.id = other_user_id
    ) THEN
        RAISE EXCEPTION 'Post relationships are not readable';
    END IF;

    -- A deleted test user provides a guaranteed nonexistent foreign key.
    DELETE FROM board.posts WHERE id = reply_id;
    DELETE FROM board.users WHERE id = other_user_id;
    BEGIN
        INSERT INTO board.threads (title, author_id) VALUES ('orphan', other_user_id);
        RAISE EXCEPTION 'Missing thread author was accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO board.posts (thread_id, author_id, body)
            VALUES (thread_id, other_user_id, 'orphan author');
        RAISE EXCEPTION 'Missing post author was accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO board.posts (thread_id, author_id, body, reply_to)
            VALUES (thread_id, user_id, 'orphan reply', reply_id);
        RAISE EXCEPTION 'Missing reply target was accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        DELETE FROM board.threads WHERE id = thread_id;
        RAISE EXCEPTION 'Referenced thread was deleted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        DELETE FROM board.users WHERE id = user_id;
        RAISE EXCEPTION 'Referenced author was deleted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO board.posts (thread_id, author_id, body) VALUES (thread_id, user_id, NULL);
        RAISE EXCEPTION 'NULL post body was accepted';
    EXCEPTION WHEN not_null_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO board.users (id, name) VALUES (user_id, 'explicit ID');
        RAISE EXCEPTION 'Explicit identity value was accepted';
    EXCEPTION WHEN generated_always THEN NULL;
    END;
    BEGIN
        INSERT INTO board.users (id, name) OVERRIDING SYSTEM VALUE VALUES (user_id, 'duplicate ID');
        RAISE EXCEPTION 'Duplicate primary key was accepted';
    EXCEPTION WHEN unique_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO board.users (name) VALUES (NULL);
        RAISE EXCEPTION 'NULL user name was accepted';
    EXCEPTION WHEN not_null_violation THEN NULL;
    END;
    BEGIN
        INSERT INTO board.threads (title, author_id) VALUES (NULL, user_id);
        RAISE EXCEPTION 'NULL thread title was accepted';
    EXCEPTION WHEN not_null_violation THEN NULL;
    END;
    BEGIN
        CREATE TABLE board.application_must_not_create_tables (id bigint);
        RAISE EXCEPTION 'Application role was allowed to create tables';
    EXCEPTION WHEN insufficient_privilege THEN NULL;
    END;

    UPDATE board.posts SET body = 'updated' WHERE id = post_id;
    GET DIAGNOSTICS changed = ROW_COUNT;
    IF changed <> 1 THEN RAISE EXCEPTION 'Application update failed'; END IF;
    DELETE FROM board.posts WHERE id = post_id;
    DELETE FROM board.threads WHERE id = thread_id;
    BEGIN
        INSERT INTO board.posts (thread_id, author_id, body) VALUES (thread_id, user_id, 'orphan thread');
        RAISE EXCEPTION 'Missing post thread was accepted';
    EXCEPTION WHEN foreign_key_violation THEN NULL;
    END;
END
$$;
ROLLBACK;
