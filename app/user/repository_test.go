package user

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"ddia/postgres"
)

func TestUserRepositoryInsertsUserWithGeneratedID(t *testing.T) {
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

	var before, after int64
	if err := primary.QueryRow(ctx, "SELECT count(*) FROM board.users").Scan(&before); err != nil {
		t.Fatal(err)
	}
	name := "repository-integration-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	user, err := NewRepository(primary).CreateUser(ctx, name)
	if err != nil {
		t.Fatal(err)
	}
	id, err := strconv.ParseInt(string(user.ID), 10, 64)
	if err != nil || id <= 0 || user.Name != name {
		t.Fatalf("CreateUser = %+v, id error=%v", user, err)
	}
	if err := primary.QueryRow(ctx, "SELECT count(*) FROM board.users").Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("users count = %d, want %d", after, before+1)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		if _, cleanupErr := primary.Exec(cleanupCtx, "DELETE FROM board.users WHERE id = $1", id); cleanupErr != nil {
			t.Errorf("cleanup user %d: %v", id, cleanupErr)
		}
	})
}
