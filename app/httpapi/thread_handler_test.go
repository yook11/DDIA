package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ddia/app/thread"
	"ddia/app/user"
)

type threadRepositoryFunc func(context.Context, user.ID, string) (thread.Thread, error)

func (f threadRepositoryFunc) CreateThread(
	ctx context.Context,
	authorID user.ID,
	title string,
) (thread.Thread, error) {
	return f(ctx, authorID, title)
}

func TestCreateThreadReturnsCreatedJSON(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/threads",
		strings.NewReader(`{"author_id":"7","title":" レプリケーション "}`),
	)
	called := false
	threads := thread.NewService(threadRepositoryFunc(func(
		ctx context.Context,
		authorID user.ID,
		title string,
	) (thread.Thread, error) {
		called = true
		if ctx != request.Context() || authorID != "7" || title != " レプリケーション " {
			t.Fatalf("request changed: ctx=%v authorID=%q title=%q", ctx, authorID, title)
		}
		return thread.Thread{ID: "42", AuthorID: authorID, Title: title}, nil
	}))
	response := httptest.NewRecorder()

	NewThreadHandler(threads).Create(response, request)

	if !called || response.Code != http.StatusCreated {
		t.Fatalf("status=%d called=%v body=%s", response.Code, called, response.Body)
	}
	var got map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got["id"] != "42" || got["author_id"] != "7" || got["title"] != " レプリケーション " {
		t.Fatalf("JSON = %v", got)
	}
}

func TestCreateThreadRejectsInvalidJSONWithoutCallingService(t *testing.T) {
	tests := map[string]string{
		"empty":             "",
		"syntax":            `{"author_id":`,
		"wrong author type": `{"author_id":7,"title":"thread"}`,
		"wrong title type":  `{"author_id":"7","title":1}`,
		"missing author":    `{"title":"thread"}`,
		"missing title":     `{"author_id":"7"}`,
		"null author":       `{"author_id":null,"title":"thread"}`,
		"null title":        `{"author_id":"7","title":null}`,
		"trailing":          `{"author_id":"7","title":"thread"} {}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			threads := thread.NewService(threadRepositoryFunc(func(context.Context, user.ID, string) (thread.Thread, error) {
				t.Fatal("invalid JSON reached the repository")
				return thread.Thread{}, nil
			}))
			response := httptest.NewRecorder()

			NewThreadHandler(threads).Create(
				response,
				httptest.NewRequest(http.MethodPost, "/threads", strings.NewReader(body)),
			)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}

func TestCreateThreadMapsErrorsWithoutExposingDetails(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"invalid author ID", thread.ErrInvalidAuthorID, http.StatusBadRequest},
		{"missing author", thread.ErrAuthorNotFound, http.StatusNotFound},
		{"internal", errors.New("password=private-detail"), http.StatusInternalServerError},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			threads := thread.NewService(threadRepositoryFunc(func(context.Context, user.ID, string) (thread.Thread, error) {
				return thread.Thread{}, test.err
			}))
			response := httptest.NewRecorder()

			NewThreadHandler(threads).Create(
				response,
				httptest.NewRequest(http.MethodPost, "/threads", strings.NewReader(`{"author_id":"7","title":"thread"}`)),
			)

			if response.Code != test.status || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}
