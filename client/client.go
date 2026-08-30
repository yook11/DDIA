package client

import (
	"fmt"

	"ddia/bbs"
)

// 掲示板とクラスタのあいだのハンドル。
// シミュレーションでも実DBでも、どの複製から読むかはここで決める。
// Backend の向こう側（遅延・時計・乱数の生成）には手が届かない。

type IDs struct{ n int }

func (i *IDs) next(prefix string) string { i.n++; return fmt.Sprintf("%s%d", prefix, i.n) }

type Client struct {
	be     Backend
	rec    *bbs.Recorder
	now    func() int
	ids    *IDs
	router Router
	sess   Session
}

func New(be Backend, rec *bbs.Recorder, now func() int, ids *IDs, user bbs.UserID, r Router) *Client {
	return &Client{be: be, rec: rec, now: now, ids: ids, router: r, sess: NewSession(user)}
}

func (c *Client) User() bbs.UserID { return c.sess.User }

// pick は Router を 1 パーティションにつき 1 回呼び、どこを読んだかを記録する。
func (c *Client) pick(seen map[int]int) Pick {
	return func(part int) int {
		node := c.router.Pick(c.be, &c.sess, part)
		seen[part] = node
		return node
	}
}

func (c *Client) CreateThread(title string) bbs.Thread {
	t := bbs.Thread{ID: bbs.ThreadID(c.ids.next("t")), Title: title, Author: c.sess.User}
	op := "POST /threads"
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Invoke, User: c.sess.User, Op: op})
	part, pos := c.be.CreateThread(t)
	c.sess.LastWritePos[part] = pos
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Ok, User: c.sess.User, Op: op})
	return t
}

func (c *Client) AddPost(thread bbs.ThreadID, body string, replyTo *bbs.PostID) bbs.Post {
	p := bbs.Post{
		ID: bbs.PostID(c.ids.next("p")), ThreadID: thread,
		Author: c.sess.User, Body: body, ReplyTo: replyTo,
	}
	op := "POST /threads/{t}/posts"
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Invoke, User: c.sess.User, Op: op, Wrote: &p})
	part, pos := c.be.AddPost(p)
	c.sess.LastWritePos[part] = pos
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Ok, User: c.sess.User, Op: op, Wrote: &p})
	return p
}

func (c *Client) ViewThread(id bbs.ThreadID) []bbs.Post {
	op := "GET /threads/{t}"
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Invoke, User: c.sess.User, Op: op, Thread: id})
	seen := map[int]int{}
	posts := c.be.ReadThread(id, c.pick(seen))
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Ok, User: c.sess.User, Op: op, Thread: id, Seen: posts, Nodes: seen})
	return posts
}

func (c *Client) ViewRecent(limit int) []bbs.Post {
	op := "GET /recent"
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Invoke, User: c.sess.User, Op: op})
	seen := map[int]int{}
	posts := c.be.Recent(limit, c.pick(seen))
	c.rec.Add(bbs.Event{T: c.now(), Phase: bbs.Ok, User: c.sess.User, Op: op, Seen: posts, Nodes: seen})
	return posts
}

var _ bbs.Handle = (*Client)(nil)
