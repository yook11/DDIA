package replicationlag

import (
	"encoding/json"
	"fmt"
	"testing"

	"ddia/bbs"
	"ddia/client"
	"ddia/sim"
	"ddia/store"
)

// writeThenView は書いてすぐスレを開き直す。Advance していないので複製はまだ届いていない。
func writeThenView(seed int64, r client.Router) []bbs.Event {
	w := sim.NewWorld(sim.Config{Seed: seed, Followers: 2})
	cl := w.Client("alice", r)
	th := bbs.CreateThread(cl, "スレ")
	bbs.Reply(cl, th.ID, "自分のレス", nil)
	bbs.ViewThread(cl, th.ID)
	return w.History()
}

func TestReadAfterWrite(t *testing.T) {
	const n = 50
	violations := 0
	for seed := int64(0); seed < n; seed++ {
		naive := writeThenView(seed, client.RandomRouter{Rng: sim.NewStreams(seed).Routing})
		if err := checkReadAfterWrite(naive); err != nil {
			violations++
		}
		fixed := writeThenView(seed, client.ReadYourWritesRouter{})
		if err := checkReadAfterWrite(fixed); err != nil {
			t.Fatalf("ReadYourWritesRouter seed=%d: %v", seed, err)
		}
	}
	if violations == 0 {
		t.Fatal("RandomRouter でリードアフターライトが一度も壊れなかった")
	}
	t.Logf("RandomRouter: %d / %d seeds で自分のレスが見えない", violations, n)
}

// run は 1 スレッドに投稿と閲覧を混ぜるだけのシナリオ。
func run(seed int64, r client.Router, cfg func(*sim.Config)) *sim.World {
	c := sim.Config{Seed: seed, Followers: 3}
	if cfg != nil {
		cfg(&c)
	}
	w := sim.NewWorld(c)
	cl := w.Client("alice", r)
	th := bbs.CreateThread(cl, "はじめてのスレ")
	for i := 0; i < 20; i++ {
		w.Advance()
		if w.Sim.Rng.Workload.Intn(2) == 0 {
			bbs.Reply(cl, th.ID, fmt.Sprintf("hello-%d", i), nil)
			continue
		}
		bbs.ViewThread(cl, th.ID)
	}
	return w
}

// 履歴にはポインタが入るので、印字ではなく中身で比べる。
func dump(t *testing.T, h []bbs.Event) string {
	t.Helper()
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSameSeedSameHistory(t *testing.T) {
	a := dump(t, run(7, client.LeaderRouter{}, nil).History())
	b := dump(t, run(7, client.LeaderRouter{}, nil).History())
	if a != b {
		t.Fatal("同じシードで履歴が一致しない")
	}
}

// ルータを差し替えても遅延と負荷が変わらないこと。
// 乱数を 1 本にすると壊れる。壊れると対策の効果が測れなくなる。
func TestRouterSwapDoesNotPerturbDelays(t *testing.T) {
	base := run(7, client.LeaderRouter{}, nil)
	alt := run(7, client.RandomRouter{Rng: sim.NewStreams(7).Routing}, nil)

	for part := 0; part < base.Backend.Partitions(); part++ {
		for node := 0; node < base.Backend.Replicas(); node++ {
			x := base.Backend.Applied(part, node)
			y := alt.Backend.Applied(part, node)
			if x != y {
				t.Fatalf("ルータ差し替えで複製の進み方が変わった part=%d node=%d: %d != %d",
					part, node, x, y)
			}
		}
	}
}

// 分割方法を変えると、同じシナリオのまま書き込みが散る先が変わる。
// 一貫性のあるプレフィックス違反が出るかどうかはここで決まる。
func TestPartitionerSpreadsWrites(t *testing.T) {
	for _, tc := range []struct {
		name string
		p    store.Partitioner
		n    int
	}{
		{"single", store.Single{}, 1},
		{"by-thread", store.ByThread{}, 4},
		{"by-post", store.ByPost{}, 4},
	} {
		w := run(7, client.LeaderRouter{}, func(c *sim.Config) {
			c.Partitions = tc.n
			c.Partitioner = tc.p
		})
		used, total := 0, 0
		for part := 0; part < tc.n; part++ {
			n := w.Backend.Applied(part, 0)
			total += n
			if n > 0 {
				used++
			}
		}
		if total == 0 {
			t.Fatalf("%s: リーダーに何も書かれていない", tc.name)
		}
		t.Logf("%s: partitions=%d 使われた=%d entries=%d", tc.name, tc.n, used, total)
	}
}
