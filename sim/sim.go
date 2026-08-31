package sim

import (
	"ddia/app"
	"ddia/database"
	"ddia/storage"
	"ddia/transport"
)

// 決定的シミュレーション。仮想時計なので実時間には触らない。

type Sim struct {
	now int
	Rng Streams
	c   *storage.Cluster
	t   *transport.Transport
	r   database.Replicator
	rec *app.Recorder
}

func (s *Sim) Now() int                  { return s.now }
func (s *Sim) Cluster() *storage.Cluster { return s.c }
func (s *Sim) History() []app.Event      { return s.rec.History() }

// Advance は時計を 1 進め、その時刻までに届く分を配送する。
// アプリもテストもこれ以外の方法で複製を進めてはいけない。
func (s *Sim) Advance() {
	s.now++
	for _, message := range s.t.DeliverUpTo(s.now) {
		s.c.Apply(message.Destination, message.Position, message.Record)
	}
}
