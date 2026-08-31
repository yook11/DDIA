package storage

import "ddia/app"

// Record はストレージのログへ格納され、レプリカへ配送される1件のデータ。
// ThreadかPostのどちらか一方を持つ。
type Record struct {
	// T はリーダーが受け付けた時刻。パーティションをまたいでマージするときの並び順に使う。
	// 単一の完全な時計を前提にしている。8章でノードごとにずらすと、この前提から壊れる。
	T      int
	Thread *app.Thread
	Post   *app.Post
}

func (r Record) ThreadID() app.ThreadID {
	if r.Thread != nil {
		return r.Thread.ID
	}
	return r.Post.ThreadID
}
