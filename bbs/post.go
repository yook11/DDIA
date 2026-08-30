package bbs

type PostID string

type Post struct {
	ID       PostID
	ThreadID ThreadID
	Author   UserID
	Body     string

	// ReplyTo は「>>123」。データの中に先後関係を作る唯一の要素で、
	// 「親は子より先に見えなければならない」という検査可能な規則を生む。
	// nil はスレッドに普通に書き込んだだけのレス。
	ReplyTo *PostID
}
