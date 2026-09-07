package httpapi

import (
	"net/http"

	"ddia/app/user"
)

func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name *string `json:"name"`
	}
	if !decodeJSON(w, r, &input) {
		return
	}
	if input.Name == nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	created, err := h.users.CreateUser(r.Context(), *input.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, struct {
		ID   user.ID `json:"id"`
		Name string  `json:"name"`
	}{ID: created.ID, Name: created.Name})
}
