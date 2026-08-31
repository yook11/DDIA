package replicationlag

import (
	"encoding/json"
	"fmt"
	"testing"

	"ddia/app"
	"ddia/database"
	"ddia/sim"
)

type routerFactory func() database.Router

func leaderRouter() database.Router {
	return database.NewLeaderRouter(database.Single{})
}

func randomRouter(seed int64) routerFactory {
	return func() database.Router {
		return database.NewRandomRouter(database.Single{}, sim.NewStreams(seed).Routing)
	}
}

func readYourWritesRouter() database.Router {
	return database.NewReadYourWritesRouter(database.Single{})
}

func partitionedLeaderRouter(partitioner database.Partitioner) routerFactory {
	return func() database.Router {
		return database.NewLeaderRouter(partitioner)
	}
}

// writeThenView は書いてすぐスレを開き直す。Advance していないので複製はまだ届いていない。
func writeThenView(seed int64, newRouter routerFactory) []app.Event {
	w := sim.NewWorld(sim.Config{Seed: seed, Followers: 2})
	router := newRouter()
	application := w.Application(router)
	session := app.NewSession("alice")
	thread := application.CreateThread(session, "スレ")
	application.Reply(session, thread.ID, "自分のレス", nil)
	application.ViewThread(session, thread.ID)
	return w.History()
}

func TestReadAfterWrite(t *testing.T) {
	const n = 50
	violations := 0
	for seed := int64(0); seed < n; seed++ {
		naive := writeThenView(seed, randomRouter(seed))
		if err := checkReadAfterWrite(naive); err != nil {
			violations++
		}
		fixed := writeThenView(seed, readYourWritesRouter)
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
func run(seed int64, newRouter routerFactory, cfg func(*sim.Config)) *sim.World {
	c := sim.Config{Seed: seed, Followers: 3}
	if cfg != nil {
		cfg(&c)
	}
	w := sim.NewWorld(c)
	router := newRouter()
	application := w.Application(router)
	session := app.NewSession("alice")
	thread := application.CreateThread(session, "はじめてのスレ")
	for i := 0; i < 20; i++ {
		w.Advance()
		if w.Sim.Rng.Workload.Intn(2) == 0 {
			application.Reply(session, thread.ID, fmt.Sprintf("hello-%d", i), nil)
			continue
		}
		application.ViewThread(session, thread.ID)
	}
	return w
}

// 履歴にはポインタが入るので、印字ではなく中身で比べる。
func dump(t *testing.T, h []app.Event) string {
	t.Helper()
	b, err := json.Marshal(h)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSameSeedSameHistory(t *testing.T) {
	a := dump(t, run(7, leaderRouter, nil).History())
	b := dump(t, run(7, leaderRouter, nil).History())
	if a != b {
		t.Fatal("同じシードで履歴が一致しない")
	}
}

// ルータを差し替えても遅延と負荷が変わらないこと。
// 乱数を 1 本にすると壊れる。壊れると対策の効果が測れなくなる。
func TestRouterSwapDoesNotPerturbDelays(t *testing.T) {
	base := run(7, leaderRouter, nil)
	alt := run(7, randomRouter(7), nil)

	baseCluster := base.Sim.Cluster()
	altCluster := alt.Sim.Cluster()
	for part := 0; part < baseCluster.Partitions(); part++ {
		for node := 0; node < baseCluster.Replicas(); node++ {
			location := database.Location{
				Partition: app.PartitionID(part),
				Node:      database.NodeID(node),
			}
			x := baseCluster.Applied(location)
			y := altCluster.Applied(location)
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
		p    database.Partitioner
		n    int
	}{
		{"single", database.Single{}, 1},
		{"by-thread", database.ByThread{}, 4},
		{"by-post", database.ByPost{}, 4},
	} {
		w := run(7, partitionedLeaderRouter(tc.p), func(c *sim.Config) {
			c.Partitions = tc.n
		})
		used, total := 0, 0
		for part := 0; part < tc.n; part++ {
			location := database.Location{Partition: app.PartitionID(part), Node: 0}
			n := w.Sim.Cluster().Applied(location)
			total += int(n)
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
