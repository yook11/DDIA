package sim

import (
	"math/rand"

	"ddia/bbs"
	"ddia/client"
	"ddia/store"
	"ddia/transport"
)

// 1 回の実験の組み立て。層をつなぐだけで、方針は何も持たない。

type Config struct {
	Seed        int64
	Partitions  int
	Followers   int
	Partitioner store.Partitioner
	Delay       func(*rand.Rand) transport.Delay
}

func (c Config) withDefaults() Config {
	if c.Partitions == 0 {
		c.Partitions = 1
	}
	if c.Partitioner == nil {
		c.Partitioner = store.Single{}
	}
	if c.Delay == nil {
		c.Delay = func(r *rand.Rand) transport.Delay {
			return transport.Uniform{Rng: r, Min: 1, Max: 3}
		}
	}
	return c
}

type World struct {
	Sim     *Sim
	Backend client.Backend
	ids     *client.IDs
	rec     *bbs.Recorder
}

func NewWorld(c Config) *World {
	c = c.withDefaults()
	rng := NewStreams(c.Seed)
	rec := &bbs.Recorder{}
	s := &Sim{
		Rng: rng,
		c:   store.NewCluster(c.Partitions, c.Followers, c.Partitioner),
		t:   transport.New(c.Delay(rng.Delay)),
		rec: rec,
	}
	return &World{Sim: s, Backend: &Backend{s: s}, ids: &client.IDs{}, rec: rec}
}

func (w *World) Client(user bbs.UserID, r client.Router) *client.Client {
	return client.New(w.Backend, w.rec, w.Sim.Now, w.ids, user, r)
}

func (w *World) Advance()             { w.Sim.Advance() }
func (w *World) History() []bbs.Event { return w.Sim.History() }
