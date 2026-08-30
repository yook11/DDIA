package sim

import (
	"hash/fnv"
	"math/rand"
)

// 乱数は層ごとに 1 本ずつ。マスタ seed から独立に導出する。
// 1 本を共有すると、ルータを差し替えただけで遅延も負荷も変わってしまい、
// 「対策が効いたのか、たまたま楽な実行だったのか」が区別できなくなる。
type Streams struct {
	Workload *rand.Rand
	Delay    *rand.Rand
	Routing  *rand.Rand
	Fault    *rand.Rand
}

func NewStreams(seed int64) Streams {
	return Streams{
		Workload: derive(seed, "workload"),
		Delay:    derive(seed, "delay"),
		Routing:  derive(seed, "routing"),
		Fault:    derive(seed, "fault"),
	}
}

func derive(seed int64, name string) *rand.Rand {
	h := fnv.New64a()
	h.Write([]byte(name))
	return rand.New(rand.NewSource(seed ^ int64(h.Sum64())))
}
