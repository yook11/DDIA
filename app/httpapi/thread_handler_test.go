package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ddia/app/readpolicy"
	"ddia/app/thread"
	"ddia/app/user"
)

type threadRepositoryStub struct {
	get func(context.Context, thread.ID) (thread.View, error)
}

func (s threadRepositoryStub) CreateThread(
	context.Context,
	user.ID,
	string,
) (thread.Thread, error) {
	panic("unexpected CreateThread call")
}

func (s threadRepositoryStub) GetThread(
	ctx context.Context,
	threadID thread.ID,
	_ readpolicy.Policy,
) (thread.View, error) {
	return s.get(ctx, threadID)
}

func TestThreadHandlerGetReturnsThreadAndPosts(t *testing.T) {
	replyTo := thread.PostID("30")
	request := httptest.NewRequest(http.MethodGet, "/threads/12", nil)
	request.SetPathValue("thread_id", "12")
	handler := NewThreadHandler(thread.NewService(threadRepositoryStub{
		get: func(ctx context.Context, threadID thread.ID) (thread.View, error) {
			if ctx != request.Context() || threadID != "12" {
				t.Fatalf("request changed: ctx=%v threadID=%q", ctx, threadID)
			}
			return thread.View{
				Thread: thread.Thread{ID: "12", AuthorID: "7", Title: "replication"},
				Posts: []thread.ThreadPost{
					{ID: "31", ThreadID: "12", AuthorID: "7", Body: "first"},
					{ID: "32", ThreadID: "12", AuthorID: "8", Body: "reply", ReplyTo: &replyTo},
				},
			}, nil
		},
	}))
	response := httptest.NewRecorder()

	handler.Get(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body)
	}
	var got struct {
		ID       string `json:"id"`
		AuthorID string `json:"author_id"`
		Title    string `json:"title"`
		Posts    []struct {
			ID       string  `json:"id"`
			ThreadID string  `json:"thread_id"`
			AuthorID string  `json:"author_id"`
			Body     string  `json:"body"`
			ReplyTo  *string `json:"reply_to"`
		} `json:"posts"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != "12" || got.AuthorID != "7" || got.Title != "replication" || len(got.Posts) != 2 {
		t.Fatalf("unexpected response: %+v", got)
	}
	if got.Posts[0].ID != "31" || got.Posts[0].ReplyTo != nil ||
		got.Posts[1].ID != "32" || got.Posts[1].ReplyTo == nil || *got.Posts[1].ReplyTo != "30" {
		t.Fatalf("unexpected posts: %+v", got.Posts)
	}
}

func TestThreadHandlerGetMapsExpectedAndInternalErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "invalid ID", err: thread.ErrInvalidThreadID, status: http.StatusBadRequest},
		{name: "not found", err: thread.ErrThreadNotFound, status: http.StatusNotFound},
		{name: "internal", err: errors.New("password=private-detail"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewThreadHandler(thread.NewService(threadRepositoryStub{
				get: func(context.Context, thread.ID) (thread.View, error) {
					return thread.View{}, test.err
				},
			}))
			request := httptest.NewRequest(http.MethodGet, "/threads/12", nil)
			request.SetPathValue("thread_id", "12")
			response := httptest.NewRecorder()

			handler.Get(response, request)

			if response.Code != test.status || strings.Contains(response.Body.String(), "private") {
				t.Fatalf("status=%d body=%s", response.Code, response.Body)
			}
		})
	}
}
