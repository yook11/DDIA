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

	writeJSON(w, http.StatusCreated, struct {
		ID       thread.ID `json:"id"`
		AuthorID user.ID   `json:"author_id"`
		Title    string    `json:"title"`
	}{ID: created.ID, AuthorID: created.AuthorID, Title: created.Title})
}
