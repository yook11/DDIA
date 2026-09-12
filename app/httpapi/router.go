package httpapi

import "net/http"

type Handlers struct {
	Users   *UserHandler
	Threads *ThreadHandler
	Posts   *PostHandler
}

func NewRouter(handlers Handlers) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /users", handlers.Users.Create)
	mux.HandleFunc("POST /threads", handlers.Threads.Create)
	mux.HandleFunc("POST /threads/{thread_id}/posts", handlers.Posts.Create)
	mux.HandleFunc("GET /threads/{thread_id}", handlers.Threads.Get)
	return mux
}
