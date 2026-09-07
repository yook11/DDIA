-- Applied inside migrate.sql's transaction, using the management role.
CREATE SCHEMA board;

CREATE TABLE board.users (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE board.threads (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    title text NOT NULL,
    author_id bigint NOT NULL REFERENCES board.users (id),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE board.posts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    thread_id bigint NOT NULL REFERENCES board.threads (id),
    author_id bigint NOT NULL REFERENCES board.users (id),
    body text NOT NULL,
    reply_to bigint REFERENCES board.posts (id),
    created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX posts_thread_id_id_idx ON board.posts (thread_id, id);

-- Application connections can manipulate data, but do not own the schema or DDL.
GRANT USAGE ON SCHEMA board TO ddia_app;
GRANT SELECT, INSERT, UPDATE, DELETE
    ON board.users, board.threads, board.posts TO ddia_app;
GRANT USAGE, SELECT
    ON SEQUENCE board.users_id_seq, board.threads_id_seq, board.posts_id_seq TO ddia_app;
