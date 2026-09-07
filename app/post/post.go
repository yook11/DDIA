package post

import (
	"ddia/app/thread"
	"ddia/app/user"
)

type ID string

type Post struct {
	ID       ID
	ThreadID thread.ID
	AuthorID user.ID
	Body     string

	// ReplyTo は返信先の投稿。nilならスレッドへの通常投稿。
	ReplyTo *ID
}
