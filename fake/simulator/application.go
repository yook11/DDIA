package simulator

import (
	"fmt"

	"ddia/app/post"
	"ddia/app/thread"
)

// Application は掲示板のエンドポイントに相当する処理を持つ。
// DBへの接続方法と読み取り先の選択はRepositoryの実装へ委ねる。
type Application struct {
	repo Repository
	rec  *Recorder
	now  func() int
	ids  *IDs
}

type IDs struct{ n int }

func NewIDs() *IDs { return &IDs{} }

func (i *IDs) next(prefix string) string {
	i.n++
	return fmt.Sprintf("%s%d", prefix, i.n)
}

func New(repo Repository, rec *Recorder, now func() int, ids *IDs) *Application {
	return &Application{repo: repo, rec: rec, now: now, ids: ids}
}

func (a *Application) CreateThread(sess *Session, title string) thread.Thread {
	created := thread.Thread{ID: thread.ID(a.ids.next("t")), Title: title, AuthorID: sess.User}
	op := "POST /threads"
	a.rec.Add(Event{T: a.now(), Phase: Invoke, User: sess.User, Op: op})
	result := a.repo.CreateThread(created)
	sess.RecordWrite(result)
	a.rec.Add(Event{T: a.now(), Phase: Ok, User: sess.User, Op: op})
	return created
}

func (a *Application) Reply(sess *Session, threadID thread.ID, body string, replyTo *post.ID) post.Post {
	created := post.Post{
		ID: post.ID(a.ids.next("p")), ThreadID: threadID,
		AuthorID: sess.User, Body: body, ReplyTo: replyTo,
	}
	op := "POST /threads/{t}/posts"
	a.rec.Add(Event{T: a.now(), Phase: Invoke, User: sess.User, Op: op, Wrote: &created})
	result := a.repo.AddPost(created)
	sess.RecordWrite(result)
	a.rec.Add(Event{T: a.now(), Phase: Ok, User: sess.User, Op: op, Wrote: &created})
	return created
}

func (a *Application) ViewThread(sess *Session, id thread.ID) []post.Post {
	op := "GET /threads/{t}"
	a.rec.Add(Event{T: a.now(), Phase: Invoke, User: sess.User, Op: op, Thread: id})
	result := a.repo.ReadThread(id, sess.RequiredPositions)
	a.rec.Add(Event{
		T: a.now(), Phase: Ok, User: sess.User, Op: op, Thread: id,
		Seen: result.Posts, Nodes: result.Nodes,
	})
	return result.Posts
}

func (a *Application) ViewRecent(sess *Session, limit int) []post.Post {
	op := "GET /recent"
	a.rec.Add(Event{T: a.now(), Phase: Invoke, User: sess.User, Op: op})
	result := a.repo.Recent(limit, sess.RequiredPositions)
	a.rec.Add(Event{
		T: a.now(), Phase: Ok, User: sess.User, Op: op,
		Seen: result.Posts, Nodes: result.Nodes,
	})
	return result.Posts
}
