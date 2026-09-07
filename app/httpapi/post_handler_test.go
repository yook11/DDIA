package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ddia/app/post"
	"ddia/app/thread"
	"ddia/app/user"
)

type postRepositoryFunc func(context.Context, thread.ID, user.ID, string) (post.Post, error)

func (f postRepositoryFunc) CreatePost(
	ctx context.Context,
	threadID thread.ID,
	authorID user.ID,
	body string,
) (post.Post, error) {
	return f(ctx, threadID, authorID, body)
}

func TestCreatePostReturnsCreatedJSON(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/threads/12/posts",
		strings.NewReader(`{"author_id":"7","body":" post body "}`),
	)
	request.SetPathValue("thread_id", "12")
	called := false
	posts := post.NewService(postRepositoryFunc(func(
		ctx context.Context,
		threadID thread.ID,
		authorID user.ID,
		body string,
	) (post.Post, error) {
		called = true
		if ctx != request.Context() || threadID != "12" || authorID != "7" || body != " post body " {
			t.Fatalf("request changed: ctx=%v threadID=%q authorID=%q body=%q", ctx, threadID, authorID, body)
		}
		return post.Post{ID: "31", ThreadID: threadID, AuthorID: authorID, Body: body}, nil
	}))
	response := httptest.NewRecorder()

	NewPostHandler(posts).Create(response, request)

	if !called || response.Code != http.StatusCreated {
		t.Fatalf("status=%d called=%v body=%s", response.Code, called, response.Body)
	}
	var got map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got["id"] != "31" || got["thread_id"] != "12" ||
		got["author_id"] != "7" || got["body"] != " post body " || got["reply_to"] != nil {
		t.Fatalf("JSON = %v", got)
	}
}

func TestCreatePostRejectsInvalidJSONWithoutCallingService(t *testing.T) {
	tests := map[string]string{
		"empty":             "",
		"syntax":            `{"author_id":`,
		"wrong author type": `{"author_id":7,"body":"post"}`,
		"wrong body type":   `{"author_id":"7","body":1}`,
		"missing author":    `{"body":"post"}`,
		"missing body":      `{"author_id":"7"}`,
		"null author":       `{"author_id":null,"body":"post"}`,
		"null body":         `{"author_id":"7","body":null}`,
		"trailing":          `{"author_id":"7","body":"post"} {}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			posts := post.NewService(postRepositoryFunc(func(context.Context, thread.ID, user.ID, string) (post.Post, error) {
				t.Fatal("invalid JSON reached the repository")
				return post.Post{}, nil
			}))
			response := httptest.NewRecorder()

			request := httptest.NewRequest(http.MethodPost, "/threads/12/posts", strings.NewReader(body))
			request.SetPathValue("thread_id", "12")
			NewPostHandler(posts).Create(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

func TestCreatePostMapsErrorsWithoutExposingDetails(t *testing.T) {
	tests := []struct {
		name     string
		threadID string
		err      error
		status   int
	}{
		{"invalid thread ID", "invalid", post.ErrInvalidThreadID, http.StatusBadRequest},
		{"invalid author ID", "12", post.ErrInvalidAuthorID, http.StatusBadRequest},
		{"missing thread", "12", post.ErrThreadNotFound, http.StatusNotFound},
		{"missing author", "12", post.ErrAuthorNotFound, http.StatusNotFound},
		{"internal", "12", errors.New("password=private-detail"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			posts := post.NewService(postRepositoryFunc(func(context.Context, thread.ID, user.ID, string) (post.Post, error) {
				return post.Post{}, test.err
			}))
			response := httptest.NewRecorder()

			request := httptest.NewRequest(
				http.MethodPost,
				"/threads/"+test.threadID+"/posts",
				strings.NewReader(`{"author_id":"7","body":"post"}`),
			)
			request.SetPathValue("thread_id", test.threadID)
			NewPostHandler(posts).Create(response, request)

			if response.Code != test.status || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}
