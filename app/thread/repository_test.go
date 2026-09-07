package thread

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"ddia/app/user"
	"ddia/postgres"
)

func TestRepositoryRejectsInvalidAuthorID(t *testing.T) {
	for _, authorID := range []user.ID{"", "abc", "0", "-1"} {
		_, err := NewRepository(nil).CreateThread(context.Background(), authorID, "title")
		if !errors.Is(err, ErrInvalidAuthorID) {
			t.Fatalf("authorID=%q error=%v", authorID, err)
		}
	}
}

func TestRepositoryInsertsThreadAndMapsMissingAuthor(t *testing.T) {
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
	author, err := userRepository.CreateUser(ctx, "thread-author-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	authorNumber, err := strconv.ParseInt(string(author.ID), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := primary.Exec(cleanupCtx, "DELETE FROM board.users WHERE id = $1", authorNumber); cleanupErr != nil {
			t.Errorf("cleanup user %d: %v", authorNumber, cleanupErr)
		}
	})

	title := "thread-integration-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	created, err := NewRepository(primary).CreateThread(ctx, author.ID, title)
	if err != nil {
		t.Fatal(err)
	}
	threadNumber, err := strconv.ParseInt(string(created.ID), 10, 64)
	if err != nil || threadNumber <= 0 || created.AuthorID != author.ID || created.Title != title {
		t.Fatalf("CreateThread = %+v, id error=%v", created, err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := primary.Exec(cleanupCtx, "DELETE FROM board.threads WHERE id = $1", threadNumber); cleanupErr != nil {
			t.Errorf("cleanup thread %d: %v", threadNumber, cleanupErr)
		}
	})

	var storedTitle string
	var storedAuthorID int64
	if err := primary.QueryRow(ctx, `
		SELECT title, author_id FROM board.threads WHERE id = $1
	`, threadNumber).Scan(&storedTitle, &storedAuthorID); err != nil {
		t.Fatal(err)
	}
	if storedTitle != title || storedAuthorID != authorNumber {
		t.Fatalf("stored thread: title=%q authorID=%d", storedTitle, storedAuthorID)
	}

	missingAuthor, err := userRepository.CreateUser(ctx, "missing-author-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	if err != nil {
		t.Fatal(err)
	}
	missingAuthorNumber, err := strconv.ParseInt(string(missingAuthor.ID), 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := primary.Exec(ctx, "DELETE FROM board.users WHERE id = $1", missingAuthorNumber); err != nil {
		t.Fatal(err)
	}
	_, err = NewRepository(primary).CreateThread(ctx, missingAuthor.ID, "missing author")
	if !errors.Is(err, ErrAuthorNotFound) {
		t.Fatalf("missing author error = %v", err)
	}
}
