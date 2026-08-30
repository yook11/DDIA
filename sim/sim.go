package sim

import (
	"ddia/bbs"
	"ddia/store"
	"ddia/transport"
)

// 決定的シミュレーション。仮想時計なので実時間には触らない。

type Sim struct {
	now int
	Rng Streams
	c   *store.Cluster
	t   *transport.Transport
	rec *bbs.Recorder
}

func (s *Sim) Now() int                { return s.now }
func (s *Sim) Cluster() *store.Cluster { return s.c }
func (s *Sim) History() []bbs.Event    { return s.rec.History() }

// Advance は時計を 1 進め、その時刻までに届く分を配送する。
// アプリもテストもこれ以外の方法で複製を進めてはいけない。
func (s *Sim) Advance() {
	s.now++
	s.t.DeliverUpTo(s.now, s.c)
}
