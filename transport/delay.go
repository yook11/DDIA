package transport

// Delay は 1 通の配送にかかる時間。フォロワーごとに変えられる。
type Delay interface {
	Next(part, node int) int
}

// Rand は乱数の供給元。transport 自身は乱数を作らない。
type Rand interface {
	Intn(n int) int
}

// Uniform は毎回独立に引く。全フォロワーが統計的に同じになるので、
// 「1 台だけ大きく遅れている」は作れない。
type Uniform struct {
	Rng      Rand
	Min, Max int
}

func (d Uniform) Next(int, int) int { return d.Min + d.Rng.Intn(d.Max-d.Min+1) }

// Slow は 1 台だけを恒常的に遅くする。DDIA が想定しているのはこちら。
type Slow struct {
	Inner  Delay
	Node   int
	Factor int
}

func (d Slow) Next(part, node int) int {
	n := d.Inner.Next(part, node)
	if node == d.Node {
		return n * d.Factor
	}
	return n
}
