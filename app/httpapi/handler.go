package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"ddia/app/post"
	"ddia/app/thread"
	"ddia/app/user"
)

type Services struct {
	Users   *user.Service
	Threads *thread.Service
	Posts   *post.Service
}

type Handler struct {
	users   *user.Service
	threads *thread.Service
	posts   *post.Service
}

func NewHandler(services Services) http.Handler {
	h := &Handler{users: services.Users, threads: services.Threads, posts: services.Posts}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", h.createUser)
	mux.HandleFunc("POST /threads", h.createThread)
	mux.HandleFunc("POST /threads/{thread_id}/posts", h.createPost)
	return mux
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, struct {
		Error string `json:"error"`
	}{Error: message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
