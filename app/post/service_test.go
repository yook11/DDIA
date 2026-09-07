package post

import (
	"context"
	"errors"
	"testing"

	"ddia/app/thread"
	"ddia/app/user"
)

type repositoryFunc func(context.Context, thread.ID, user.ID, string) (Post, error)

func (f repositoryFunc) CreatePost(
	ctx context.Context,
	threadID thread.ID,
	authorID user.ID,
	body string,
) (Post, error) {
	return f(ctx, threadID, authorID, body)
}

func TestServicePassesRequestToRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	want := Post{ID: "31", ThreadID: "12", AuthorID: "7", Body: " body "}
	called := false
	service := NewService(repositoryFunc(func(
		gotCtx context.Context,
		threadID thread.ID,
		authorID user.ID,
		body string,
	) (Post, error) {
		called = true
		if gotCtx != ctx || threadID != want.ThreadID || authorID != want.AuthorID || body != want.Body {
			t.Fatalf("request changed: ctx=%v threadID=%q authorID=%q body=%q", gotCtx, threadID, authorID, body)
		}
		return want, nil
	}))

	got, err := service.CreatePost(ctx, want.ThreadID, want.AuthorID, want.Body)
	if !called || err != nil || got != want {
		t.Fatalf("CreatePost = %+v, %v; called=%v", got, err, called)
	}
}

func TestServiceReturnsRepositoryError(t *testing.T) {
	want := errors.New("repository unavailable")
	service := NewService(repositoryFunc(func(context.Context, thread.ID, user.ID, string) (Post, error) {
		return Post{}, want
	}))

	got, err := service.CreatePost(context.Background(), "12", "7", "body")
	if err != want || got != (Post{}) {
		t.Fatalf("CreatePost = %+v, %v", got, err)
	}
}
