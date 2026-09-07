package thread

import (
	"context"
	"errors"
	"testing"

	"ddia/app/user"
)

type repositoryFunc func(context.Context, user.ID, string) (Thread, error)

func (f repositoryFunc) CreateThread(ctx context.Context, authorID user.ID, title string) (Thread, error) {
	return f(ctx, authorID, title)
}

func TestServicePassesRequestToRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	want := Thread{ID: "12", AuthorID: "7", Title: " replication "}
	called := false
	service := NewService(repositoryFunc(func(gotCtx context.Context, authorID user.ID, title string) (Thread, error) {
		called = true
		if gotCtx != ctx || authorID != want.AuthorID || title != want.Title {
			t.Fatalf("request changed: ctx=%v authorID=%q title=%q", gotCtx, authorID, title)
		}
		return want, nil
	}))

	got, err := service.CreateThread(ctx, want.AuthorID, want.Title)
	if !called || err != nil || got != want {
		t.Fatalf("CreateThread = %+v, %v; called=%v", got, err, called)
	}
}

func TestServiceReturnsRepositoryError(t *testing.T) {
	want := errors.New("repository unavailable")
	service := NewService(repositoryFunc(func(context.Context, user.ID, string) (Thread, error) {
		return Thread{}, want
	}))

	got, err := service.CreateThread(context.Background(), "7", "title")
	if err != want || got != (Thread{}) {
		t.Fatalf("CreateThread = %+v, %v", got, err)
	}
}
