package post

import (
	"ddia/app/readpolicy"
	"ddia/app/thread"
	"ddia/app/user"
)

type ID string

type WriteResult struct {
	Post     Post
	Position readpolicy.Position
}

type Post struct {
	ID       ID
	ThreadID thread.ID
	AuthorID user.ID
	Body     string

	// ReplyTo は返信先の投稿。nilならスレッドへの通常投稿。
	ReplyTo *ID
}
