package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ddia/app/post"
	"ddia/app/thread"
	"ddia/app/user"
)

func TestRouterDispatchesRequestsToFeatureHandlers(t *testing.T) {
	tests := []struct {
		name string
		path string
		body string
		want string
	}{
		{
			name: "user",
			path: "/users",
			body: `{"name":"alice"}`,
			want: "user",
		},
		{
			name: "thread",
			path: "/threads",
			body: `{"author_id":"1","title":"replication"}`,
			want: "thread",
		},
		{
			name: "post",
			path: "/threads/2/posts",
			body: `{"author_id":"1","body":"hello"}`,
			want: "post",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			called := ""
			router := newTestRouter(
				func() { called = "user" },
				func() { called = "thread" },
				func() { called = "post" },
			)
			response := httptest.NewRecorder()

			router.ServeHTTP(
				response,
				httptest.NewRequest(http.MethodPost, test.path, strings.NewReader(test.body)),
			)

			if response.Code != http.StatusCreated || called != test.want {
				t.Fatalf("status=%d called=%q body=%s", response.Code, called, response.Body)
			}
		})
	}
}

func TestRouterRejectsOtherMethods(t *testing.T) {
	for _, path := range []string{
		"/users",
		"/threads",
		"/threads/2/posts",
	} {
		t.Run(path, func(t *testing.T) {
			router := newTestRouter(
				func() { t.Fatal("user handler called") },
				func() { t.Fatal("thread handler called") },
				func() { t.Fatal("post handler called") },
			)
			response := httptest.NewRecorder()

			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))

			if response.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

func newTestRouter(userCalled, threadCalled, postCalled func()) http.Handler {
	return NewRouter(Handlers{
		Users: NewUserHandler(user.NewService(userRepositoryFunc(func(
			context.Context,
			string,
		) (user.User, error) {
			userCalled()
			return user.User{ID: "1", Name: "alice"}, nil
		}))),
		Threads: NewThreadHandler(thread.NewService(threadRepositoryFunc(func(
			context.Context,
			user.ID,
			string,
		) (thread.Thread, error) {
			threadCalled()
			return thread.Thread{ID: "2", AuthorID: "1", Title: "replication"}, nil
		}))),
		Posts: NewPostHandler(post.NewService(postRepositoryFunc(func(
			context.Context,
			thread.ID,
			user.ID,
			string,
		) (post.Post, error) {
			postCalled()
			return post.Post{ID: "3", ThreadID: "2", AuthorID: "1", Body: "hello"}, nil
		}))),
	})
}
