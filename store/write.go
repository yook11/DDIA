package store

import "ddia/bbs"

// Write はレプリケーションされる 1 レコード。Thread か Post のどちらか一方を持つ。
type Write struct {
	// T はリーダーが受け付けた時刻。パーティションをまたいでマージするときの並び順に使う。
	// 単一の完全な時計を前提にしている。8 章でノードごとにずらすと、この前提から壊れる。
	T      int
	Thread *bbs.Thread
	Post   *bbs.Post
}

func (w Write) ThreadID() bbs.ThreadID {
	if w.Thread != nil {
		return w.Thread.ID
	}
	return w.Post.ThreadID
}
