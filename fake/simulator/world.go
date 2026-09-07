package simulator

import (
	"math/rand"

	"ddia/fake/database"
	"ddia/fake/transport"
)

// 1 回の実験の組み立て。層をつなぐだけで、方針は何も持たない。

type Config struct {
	Seed       int64
	Partitions int
	Followers  int
	Replicator database.Replicator
	Delay      func(*rand.Rand) transport.Delay
}

func (c Config) withDefaults() Config {
	if c.Partitions == 0 {
		c.Partitions = 1
	}
	if c.Replicator == nil {
		c.Replicator = database.AllFollowersReplicator{}
	}
	if c.Delay == nil {
		c.Delay = func(r *rand.Rand) transport.Delay {
			return transport.Uniform{Rng: r, Min: 1, Max: 3}
		}
	}
	return c
}

type World struct {
	Sim *Sim
	ids *IDs
	rec *Recorder
}

func NewWorld(c Config) *World {
	c = c.withDefaults()
	rng := NewStreams(c.Seed)
	rec := &Recorder{}
	s := &Sim{
		Rng: rng,
		c:   database.NewCluster(c.Partitions, c.Followers),
		t:   transport.New(c.Delay(rng.Delay)),
		r:   c.Replicator,
		rec: rec,
	}
	return &World{Sim: s, ids: NewIDs(), rec: rec}
}

func (w *World) Application(router database.Router) *Application {
	repository := &Database{s: w.Sim, router: router}
	return New(repository, w.rec, w.Sim.Now, w.ids)
}

func (w *World) Advance()         { w.Sim.Advance() }
func (w *World) History() []Event { return w.Sim.History() }
