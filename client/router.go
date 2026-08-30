package client

// Rand は乱数の供給元。client 自身は乱数を作らない。
// 生成を外に出しておくと、ルータを差し替えても遅延や負荷の乱数列がずれない。
type Rand interface {
	Intn(n int) int
}

// Router は「あるパーティションの、どの複製から読むか」だけを決める。
// 見えるのは ClusterView とセッションだけ。時計もパーティションの中身も渡さない。
type Router interface {
	Pick(v ClusterView, sess *Session, part int) int
}

// LeaderRouter は常にリーダー。対策ではなく、配管を動かすための退化ケース。
type LeaderRouter struct{}

func (LeaderRouter) Pick(ClusterView, *Session, int) int { return 0 }

// RandomRouter は毎回独立に複製を選ぶ。対策なしの読み方。
type RandomRouter struct{ Rng Rand }

func (r RandomRouter) Pick(v ClusterView, _ *Session, _ int) int {
	return r.Rng.Intn(v.Replicas())
}

// ReadYourWritesRouter は、自分が書いた位置まで追いついている複製だけを読む。
// 追いついていなければリーダー（常に自分の書き込みを持つ）。
type ReadYourWritesRouter struct{}

func (ReadYourWritesRouter) Pick(v ClusterView, sess *Session, part int) int {
	need := sess.LastWritePos[part]
	for n := v.Replicas() - 1; n >= 0; n-- {
		if v.Applied(part, n) >= need {
			return n
		}
	}
	return 0
}
