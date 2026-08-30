package bbs

// エンドポイント。掲示板を叩くとこうなる、という面。
// どの章の実験でもここは変わらない。変える必要が出たら層の切り方が間違っている。
// 呼び出し相手の実装は client。どの複製から読むかはそちらが決める。

type Handle interface {
	CreateThread(title string) Thread
	AddPost(thread ThreadID, body string, replyTo *PostID) Post
	ViewThread(id ThreadID) []Post
	ViewRecent(limit int) []Post
}

func CreateThread(cl Handle, title string) Thread { return cl.CreateThread(title) }

func Reply(cl Handle, t ThreadID, body string, to *PostID) Post {
	return cl.AddPost(t, body, to)
}

func ViewThread(cl Handle, t ThreadID) []Post { return cl.ViewThread(t) }

func ViewRecent(cl Handle, limit int) []Post { return cl.ViewRecent(limit) }
