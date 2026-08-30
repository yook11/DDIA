package client

import "ddia/bbs"

// Backend はクライアントから見たクラスタ。
// シミュレーションでも実DBの複数台でも、ここを満たせばクライアントは動く。

// Pick はパーティションごとにどの複製から読むかを返す。
// 複製の選択（5 章）は Router、パーティションの選択（6 章）は Backend の向こう側。
// 軸が違うので混ぜない。scatter-gather は Router を置き換えず、1 パーティションにつき 1 回呼ぶ。
type Pick func(part int) int

type ClusterView interface {
	Partitions() int
	Replicas() int
	Applied(part, node int) int
}

type Backend interface {
	ClusterView
	CreateThread(t bbs.Thread) (part, pos int)
	AddPost(p bbs.Post) (part, pos int)
	ReadThread(id bbs.ThreadID, pick Pick) []bbs.Post
	Recent(limit int, pick Pick) []bbs.Post
}
