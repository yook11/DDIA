package httpapi

import (
	"errors"
	"net/http"

	"ddia/app/thread"
	"ddia/app/user"
)

type ThreadHandler struct {
	service *thread.Service
}

type getThreadResponse struct {
	ID       thread.ID               `json:"id"`
	AuthorID user.ID                 `json:"author_id"`
	Title    string                  `json:"title"`
	Posts    []getThreadPostResponse `json:"posts"`
}

type getThreadPostResponse struct {
	ID       thread.PostID  `json:"id"`
	ThreadID thread.ID      `json:"thread_id"`
	AuthorID user.ID        `json:"author_id"`
	Body     string         `json:"body"`
	ReplyTo  *thread.PostID `json:"reply_to"`
}

func NewThreadHandler(service *thread.Service) *ThreadHandler {
	return &ThreadHandler{service: service}
}

func (h *ThreadHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AuthorID *string `json:"author_id"`
		Title    *string `json:"title"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.AuthorID == nil || input.Title == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.service.CreateThread(r.Context(), user.ID(*input.AuthorID), *input.Title)
	switch {
	case errors.Is(err, thread.ErrInvalidAuthorID):
		writeError(w, http.StatusBadRequest, "invalid author_id")
		return
	case errors.Is(err, thread.ErrAuthorNotFound):
		writeError(w, http.StatusNotFound, "user not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if created.Position != "" {
		w.Header().Set(writePositionHeader, string(created.Position))
	}

	writeJSON(w, http.StatusCreated, struct {
		ID       thread.ID `json:"id"`
		AuthorID user.ID   `json:"author_id"`
		Title    string    `json:"title"`
	}{ID: created.Thread.ID, AuthorID: created.Thread.AuthorID, Title: created.Thread.Title})
}

func (h *ThreadHandler) Get(w http.ResponseWriter, r *http.Request) {
	threadID := thread.ID(r.PathValue("thread_id"))

	policy := readPolicyFromRequest(r)

	result, err := h.service.GetThread(r.Context(), threadID, policy)

	switch {
	case errors.Is(err, thread.ErrInvalidThreadID):
		writeError(w, http.StatusBadRequest, "invalid thread_id")
		return
	case errors.Is(err, thread.ErrThreadNotFound):
		writeError(w, http.StatusNotFound, "thread not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	posts := make([]getThreadPostResponse, 0, len(result.Posts))
	for _, post := range result.Posts {
		posts = append(posts, getThreadPostResponse{
			ID:       post.ID,
			ThreadID: post.ThreadID,
			AuthorID: post.AuthorID,
			Body:     post.Body,
			ReplyTo:  post.ReplyTo,
		})
	}
	writeJSON(w, http.StatusOK, getThreadResponse{
		ID:       result.Thread.ID,
		AuthorID: result.Thread.AuthorID,
		Title:    result.Thread.Title,
		Posts:    posts,
	})
}
