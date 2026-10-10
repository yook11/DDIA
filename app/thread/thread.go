package thread

import (
	"ddia/app/readpolicy"
	"ddia/app/user"
)

type ID string

type Thread struct {
	ID       ID
	Title    string
	AuthorID user.ID
}

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

// 書き込みの結果
type WriteResult struct {
	Thread   Thread
	Position readpolicy.Position
}
