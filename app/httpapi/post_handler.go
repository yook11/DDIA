package httpapi

import (
	"errors"
	"net/http"

	"ddia/app/post"
	"ddia/app/thread"
	"ddia/app/user"
)

type PostHandler struct {
	service *post.Service
}

func NewPostHandler(service *post.Service) *PostHandler {
	return &PostHandler{service: service}
}

func (h *PostHandler) Create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		AuthorID *string `json:"author_id"`
		Body     *string `json:"body"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.AuthorID == nil || input.Body == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.service.CreatePost(
		r.Context(),
		thread.ID(r.PathValue("thread_id")),
		user.ID(*input.AuthorID),
		*input.Body,
	)
	switch {
	case errors.Is(err, post.ErrInvalidThreadID):
		writeError(w, http.StatusBadRequest, "invalid thread_id")
		return
	case errors.Is(err, post.ErrInvalidAuthorID):
		writeError(w, http.StatusBadRequest, "invalid author_id")
		return
	case errors.Is(err, post.ErrThreadNotFound):
		writeError(w, http.StatusNotFound, "thread not found")
		return
	case errors.Is(err, post.ErrAuthorNotFound):
		writeError(w, http.StatusNotFound, "user not found")
		return
	case err != nil:
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, struct {
		ID       post.ID   `json:"id"`
		ThreadID thread.ID `json:"thread_id"`
		AuthorID user.ID   `json:"author_id"`
		Body     string    `json:"body"`
		ReplyTo  *post.ID  `json:"reply_to"`
	}{
		ID:       created.ID,
		ThreadID: created.ThreadID,
		AuthorID: created.AuthorID,
		Body:     created.Body,
		ReplyTo:  created.ReplyTo,
	})
}
