package replicationlag

import "math/rand"

type Scheduler struct {
	rng *rand.Rand
	c   *Cluster
}

func NewScheduler(seed int64, c *Cluster) *Scheduler {
	return &Scheduler{
		rng: rand.New(rand.NewSource(seed)),
		c:   c,
	}
}

// Tick はフォロワーを 1 件進めるか、何もしない。テストからは呼ばず、ここだけが Step する。
func (s *Scheduler) Tick() {
	if s.rng.Intn(2) == 0 {
		return
	}
	s.c.Step(s.rng.Intn(len(s.c.Followers)))
}

func (s *Scheduler) PickReplica() int {
	return s.rng.Intn(len(s.c.Followers))
}
