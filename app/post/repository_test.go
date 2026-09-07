package post

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"ddia/app/thread"
	"ddia/app/user"
	"ddia/postgres"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryRejectsInvalidIDs(t *testing.T) {
	for _, test := range []struct {
		name     string
		threadID thread.ID
		authorID user.ID
		want     error
	}{
		{"empty thread", "", "1", ErrInvalidThreadID},
		{"nonnumeric thread", "abc", "1", ErrInvalidThreadID},
		{"zero thread", "0", "1", ErrInvalidThreadID},
		{"negative thread", "-1", "1", ErrInvalidThreadID},
		{"empty author", "1", "", ErrInvalidAuthorID},
		{"nonnumeric author", "1", "abc", ErrInvalidAuthorID},
		{"zero author", "1", "0", ErrInvalidAuthorID},
		{"negative author", "1", "-1", ErrInvalidAuthorID},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRepository(nil).CreatePost(context.Background(), test.threadID, test.authorID, "body")
			if !errors.Is(err, test.want) {
				t.Fatalf("error=%v want=%v", err, test.want)
			}
		})
	}
}

func TestRepositoryInsertsPostAndMapsMissingReferences(t *testing.T) {
	dsn := os.Getenv("DDIA_TEST_PRIMARY_DATABASE_URL")
	if dsn == "" {
		t.Skip("DDIA_TEST_PRIMARY_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	primary, err := postgres.OpenPrimary(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(primary.Close)

	userRepository := user.NewRepository(primary)
	threadRepository := thread.NewRepository(primary)
	postRepository := NewRepository(primary)
	author, err := userRepository.CreateUser(ctx, "post-author-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	authorNumber := numericID(t, string(author.ID))
	t.Cleanup(func() {
		cleanupRow(t, primary, "DELETE FROM board.users WHERE id = $1", authorNumber)
	})

	parent, err := threadRepository.CreateThread(ctx, author.ID, "post-parent")
	if err != nil {
		t.Fatal(err)
	}
	threadNumber := numericID(t, string(parent.ID))
	t.Cleanup(func() {
		cleanupRow(t, primary, "DELETE FROM board.threads WHERE id = $1", threadNumber)
	})

	body := " post integration body "
	created, err := postRepository.CreatePost(ctx, parent.ID, author.ID, body)
	if err != nil {
		t.Fatal(err)
	}
	postNumber := numericID(t, string(created.ID))
	t.Cleanup(func() {
		cleanupRow(t, primary, "DELETE FROM board.posts WHERE id = $1", postNumber)
	})
	if created.ThreadID != parent.ID || created.AuthorID != author.ID || created.Body != body || created.ReplyTo != nil {
		t.Fatalf("CreatePost = %+v", created)
	}

	var storedThreadID int64
	var storedAuthorID int64
	var storedBody string
	var replyIsNull bool
	if err := primary.QueryRow(ctx, `
		SELECT thread_id, author_id, body, reply_to IS NULL
		FROM board.posts
		WHERE id = $1
	`, postNumber).Scan(&storedThreadID, &storedAuthorID, &storedBody, &replyIsNull); err != nil {
		t.Fatal(err)
	}
	if storedThreadID != threadNumber || storedAuthorID != authorNumber || storedBody != body || !replyIsNull {
		t.Fatalf("stored post: threadID=%d authorID=%d body=%q replyIsNull=%v", storedThreadID, storedAuthorID, storedBody, replyIsNull)
	}

	missingThread, err := threadRepository.CreateThread(ctx, author.ID, "deleted thread")
	if err != nil {
		t.Fatal(err)
	}
	missingThreadNumber := numericID(t, string(missingThread.ID))
	t.Cleanup(func() {
		cleanupRow(t, primary, "DELETE FROM board.threads WHERE id = $1", missingThreadNumber)
	})
	if _, err := primary.Exec(ctx, "DELETE FROM board.threads WHERE id = $1", missingThreadNumber); err != nil {
		t.Fatal(err)
	}
	if _, err := postRepository.CreatePost(ctx, missingThread.ID, author.ID, "body"); !errors.Is(err, ErrThreadNotFound) {
		t.Fatalf("missing thread error = %v", err)
	}

	missingAuthor, err := userRepository.CreateUser(ctx, "deleted-post-author-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	missingAuthorNumber := numericID(t, string(missingAuthor.ID))
	t.Cleanup(func() {
		cleanupRow(t, primary, "DELETE FROM board.users WHERE id = $1", missingAuthorNumber)
	})
	if _, err := primary.Exec(ctx, "DELETE FROM board.users WHERE id = $1", missingAuthorNumber); err != nil {
		t.Fatal(err)
	}
	if _, err := postRepository.CreatePost(ctx, parent.ID, missingAuthor.ID, "body"); !errors.Is(err, ErrAuthorNotFound) {
		t.Fatalf("missing author error = %v", err)
	}
}

func numericID(t *testing.T, value string) int64 {
	t.Helper()
	number, err := strconv.ParseInt(value, 10, 64)
	if err != nil || number <= 0 {
		t.Fatalf("invalid generated ID %q: %v", value, err)
	}
	return number
}

func cleanupRow(t *testing.T, primary *pgxpool.Pool, query string, id int64) {
	t.Helper()
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cleanupCancel()
	if _, err := primary.Exec(cleanupCtx, query, id); err != nil {
		t.Errorf("cleanup row %d: %v", id, err)
	}
}
