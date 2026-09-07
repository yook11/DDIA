package user

import (
	"context"
	"errors"
	"testing"
)

type userRepositoryFunc func(context.Context, string) (User, error)

func (f userRepositoryFunc) CreateUser(ctx context.Context, name string) (User, error) {
	return f(ctx, name)
}

func TestUserServicePassesContextAndNameToRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	want := User{ID: "42", Name: " Alice "}
	called := false
	service := NewService(userRepositoryFunc(func(gotCtx context.Context, name string) (User, error) {
		called = true
		if gotCtx != ctx || name != want.Name {
			t.Fatalf("request changed: ctx=%v name=%q", gotCtx, name)
		}
		return want, nil
	}))
	got, err := service.CreateUser(ctx, want.Name)
	if !called || err != nil || got != want {
		t.Fatalf("CreateUser = %+v, %v; called=%v", got, err, called)
	}
}

func TestUserServiceReturnsRepositoryError(t *testing.T) {
	want := errors.New("repository unavailable")
	service := NewService(userRepositoryFunc(func(context.Context, string) (User, error) {
		return User{}, want
	}))
	got, err := service.CreateUser(context.Background(), "alice")
	if err != want || got != (User{}) {
		t.Fatalf("CreateUser = %+v, %v", got, err)
	}
}
