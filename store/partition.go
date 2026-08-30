package store

import "hash/fnv"

// 分割方法。1 つの掲示板をどう複数のデータベースに分けるか。
// ドメインの都合ではなくインフラの方針なので、bbs からは見えない位置に置く。
// 差し替えると、アプリも負荷もチェッカも変えずに異常が出たり消えたりする。
type Partitioner interface {
	Partition(w Write, n int) int
}

// Single は分割しない。P=1。一貫性のあるプレフィックス違反は原理的に出ない。
type Single struct{}

func (Single) Partition(Write, int) int { return 0 }

// ByThread はスレッド単位で分ける。親レスと返信が同じ保存先に落ちるので、
// スレ内の因果は保たれる。因果のあるデータを同じパーティションに置く、が対策そのもの。
type ByThread struct{}

func (ByThread) Partition(w Write, n int) int { return shard(string(w.ThreadID()), n) }

// ByPost はレス単位でばらす。同じスレの親子でも別の保存先に散るので違反が出る。
type ByPost struct{}

func (ByPost) Partition(w Write, n int) int {
	if w.Post != nil {
		return shard(string(w.Post.ID), n)
	}
	return shard(string(w.ThreadID()), n)
}

func shard(key string, n int) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32()) % n
}
