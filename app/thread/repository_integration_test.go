package thread_test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"ddia/app/readpolicy"
	"ddia/app/thread"
	"ddia/postgres"
)

func TestRepositoryGetsThreadAndPostsFromPrimary(t *testing.T) {
	databaseURL := os.Getenv("DDIA_TEST_PRIMARY_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DDIA_TEST_PRIMARY_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	primary, err := postgres.OpenPrimary(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(primary.Close)

	var authorID int64
	err = primary.QueryRow(ctx, `
		INSERT INTO board.users (name)
		VALUES ($1)
		RETURNING id
	`, "get-thread-test-"+strconv.FormatInt(time.Now().UnixNano(), 10)).Scan(&authorID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := primary.Exec(cleanupCtx, "DELETE FROM board.users WHERE id = $1", authorID); cleanupErr != nil {
			t.Errorf("cleanup user %d: %v", authorID, cleanupErr)
		}
	})

	var threadID int64
	err = primary.QueryRow(ctx, `
		INSERT INTO board.threads (author_id, title)
		VALUES ($1, $2)
		RETURNING id
	`, authorID, "replication thread").Scan(&threadID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := primary.Exec(cleanupCtx, "DELETE FROM board.posts WHERE thread_id = $1", threadID); cleanupErr != nil {
			t.Errorf("cleanup posts for thread %d: %v", threadID, cleanupErr)
		}
		if _, cleanupErr := primary.Exec(cleanupCtx, "DELETE FROM board.threads WHERE id = $1", threadID); cleanupErr != nil {
			t.Errorf("cleanup thread %d: %v", threadID, cleanupErr)
		}
	})

	var firstPostID int64
	err = primary.QueryRow(ctx, `
		INSERT INTO board.posts (thread_id, author_id, body)
		VALUES ($1, $2, $3)
		RETURNING id
	`, threadID, authorID, "first post").Scan(&firstPostID)
	if err != nil {
		t.Fatal(err)
	}
	var replyPostID int64
	err = primary.QueryRow(ctx, `
		INSERT INTO board.posts (thread_id, author_id, body, reply_to)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, threadID, authorID, "reply post", firstPostID).Scan(&replyPostID)
	if err != nil {
		t.Fatal(err)
	}

	got, err := thread.NewRepository(postgres.NewRouter(primary, nil)).GetThread(
		ctx,
		thread.ID(strconv.FormatInt(threadID, 10)),
		readpolicy.ReadYourWrites{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got.Thread.ID != thread.ID(strconv.FormatInt(threadID, 10)) ||
		string(got.Thread.AuthorID) != strconv.FormatInt(authorID, 10) ||
		got.Thread.Title != "replication thread" || len(got.Posts) != 2 {
		t.Fatalf("unexpected thread view: %+v", got)
	}
	if got.Posts[0].ID != thread.PostID(strconv.FormatInt(firstPostID, 10)) || got.Posts[0].ReplyTo != nil {
		t.Fatalf("unexpected first post: %+v", got.Posts[0])
	}
	if got.Posts[1].ID != thread.PostID(strconv.FormatInt(replyPostID, 10)) ||
		got.Posts[1].ReplyTo == nil ||
		*got.Posts[1].ReplyTo != thread.PostID(strconv.FormatInt(firstPostID, 10)) {
		t.Fatalf("unexpected reply post: %+v", got.Posts[1])
	}
}
