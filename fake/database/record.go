package database

import (
	"ddia/app/post"
	"ddia/app/thread"
)

// Record はストレージのログへ格納され、レプリカへ配送される1件のデータ。
// ThreadかPostのどちらか一方を持つ。
type Record struct {
	// T はリーダーが受け付けた時刻。パーティションをまたいでマージするときの並び順に使う。
	// 単一の完全な時計を前提にしている。8章でノードごとにずらすと、この前提から壊れる。
	T      int
	Thread *thread.Thread
	Post   *post.Post
}

func (r Record) ThreadID() thread.ID {
	if r.Thread != nil {
		return r.Thread.ID
	}
	return r.Post.ThreadID
}
