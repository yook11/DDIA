package sim

import (
	"sort"

	"ddia/bbs"
	"ddia/client"
	"ddia/store"
)

// client.Backend の決定的シミュレーション実装。ここが sim ⇄ 実DBの差し替え点。
// Phase 2 で実物に移すとき、書き換えるのはこのファイルだけになる。

type Backend struct{ s *Sim }

func (b *Backend) Partitions() int            { return b.s.c.Partitions() }
func (b *Backend) Replicas() int              { return b.s.c.Replicas() }
func (b *Backend) Applied(part, node int) int { return b.s.c.Applied(part, node) }

func (b *Backend) CreateThread(t bbs.Thread) (int, int) {
	return b.append(store.Write{T: b.s.now, Thread: &t})
}

func (b *Backend) AddPost(p bbs.Post) (int, int) {
	return b.append(store.Write{T: b.s.now, Post: &p})
}

// append はリーダーへ書いて即返る（非同期レプリケーション）。
// ack を待つ形にすると 5.1.2 の同期／半同期がそのまま実験になる。まだ入れていない。
func (b *Backend) append(w store.Write) (int, int) {
	part, pos := b.s.c.Append(w)
	b.s.t.Broadcast(b.s.now, b.s.c, part, pos, w)
	return part, pos
}

func (b *Backend) ReadThread(id bbs.ThreadID, pick client.Pick) []bbs.Post {
	return b.scatter(pick, func(p bbs.Post) bool { return p.ThreadID == id }, 0)
}

// Recent は全パーティションに問い合わせてマージする。
// 1 回の論理的な読み取りが N 回の物理的な読み取りになり、N 個の遅れ具合はそれぞれ違う。
// 異なる論理時刻に撮られた N 枚のスナップショットを 1 枚として見せているので、
// 異常はマージのバグではなく構造的に出る。
func (b *Backend) Recent(limit int, pick client.Pick) []bbs.Post {
	return b.scatter(pick, func(bbs.Post) bool { return true }, limit)
}

type located struct {
	p    bbs.Post
	t    int
	part int
	pos  int
}

func (b *Backend) scatter(pick client.Pick, keep func(bbs.Post) bool, limit int) []bbs.Post {
	var hits []located
	for part := 0; part < b.s.c.Partitions(); part++ {
		node := pick(part)
		for i, w := range b.s.c.Log(part, node) {
			if w.Post == nil || !keep(*w.Post) {
				continue
			}
			hits = append(hits, located{p: *w.Post, t: w.T, part: part, pos: i + 1})
		}
	}
	// マージキーは書き込み時刻。パーティションをまたぐ全体順序は存在しないので、これは近似でしかない。
	// 今は完全な時計が 1 つあるので順序自体は正しく、異常は「あるべきものが無い」形でだけ出る。
	// 8 章で時計をずらすとマージ自体が壊れる。
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.t != b.t {
			return a.t < b.t
		}
		if a.part != b.part {
			return a.part < b.part
		}
		return a.pos < b.pos
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[len(hits)-limit:]
	}
	out := make([]bbs.Post, 0, len(hits))
	for _, h := range hits {
		out = append(out, h.p)
	}
	return out
}
