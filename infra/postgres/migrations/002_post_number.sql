-- Applied inside migrate.sql's transaction, using the management role.
-- Post numbers are sequential within a thread (">>1"). The thread row holds the counter,
-- so concurrent posts to the same thread serialize on that row.

-- Numbering existing posts can exceed the wrapper's 5s statement timeout on large tables.
SET LOCAL statement_timeout = 0;

ALTER TABLE board.threads ADD COLUMN post_count integer NOT NULL DEFAULT 0;
ALTER TABLE board.posts ADD COLUMN post_number integer;

-- Existing posts are numbered in ID order, and each thread continues from its count.
UPDATE board.posts AS p
SET post_number = numbered.n
FROM (
    SELECT id, row_number() OVER (PARTITION BY thread_id ORDER BY id) AS n
    FROM board.posts
) AS numbered
WHERE p.id = numbered.id;

UPDATE board.threads AS t
SET post_count = counted.n
FROM (SELECT thread_id, count(*) AS n FROM board.posts GROUP BY thread_id) AS counted
WHERE t.id = counted.thread_id;

ALTER TABLE board.posts ALTER COLUMN post_number SET NOT NULL;
ALTER TABLE board.posts
    ADD CONSTRAINT posts_thread_id_post_number_key UNIQUE (thread_id, post_number);
