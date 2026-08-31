package transport

import "ddia/database"

// Delay は 1 通の配送にかかる時間。フォロワーごとに変えられる。
type Delay interface {
	Next(database.Location) int
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

func (d Uniform) Next(database.Location) int { return d.Min + d.Rng.Intn(d.Max-d.Min+1) }

// Slow は 1 台だけを恒常的に遅くする。DDIA が想定しているのはこちら。
type Slow struct {
	Inner  Delay
	Node   database.NodeID
	Factor int
}

func (d Slow) Next(destination database.Location) int {
	n := d.Inner.Next(destination)
	if destination.Node == d.Node {
		return n * d.Factor
	}
	return n
}
