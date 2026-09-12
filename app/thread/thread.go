package thread

import "ddia/app/user"

type ID string

type Thread struct {
	ID       ID
	Title    string
	AuthorID user.ID
}

// View is the result shown when a thread and its posts are read together.
type View struct {
	Thread Thread
	Posts  []ThreadPost
}

type PostID string

type ThreadPost struct {
	ID       PostID
	ThreadID ID
	AuthorID user.ID
	Body     string
	ReplyTo  *PostID
}
