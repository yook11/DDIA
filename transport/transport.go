package transport

import "ddia/store"

// ノード間の配送。遅延・喪失・順序の乱れはすべてここの方針になる。
// リーダーが位置つきで押し込む（push）。喪失時に再送する責任もリーダー側にある。

type message struct {
	at   int
	part int
	node int
	pos  int
	w    store.Write
}

type Transport struct {
	q     []message
	delay Delay
}

func New(d Delay) *Transport { return &Transport{delay: d} }

// Broadcast はリーダーが受け付けた 1 件を全フォロワー宛に積む。
// 遅延はフォロワーごとに独立に引くので、届く順は乱れうる。
func (t *Transport) Broadcast(now int, c *store.Cluster, part, pos int, w store.Write) {
	for node := 1; node < c.Replicas(); node++ {
		t.q = append(t.q, message{
			at:   now + t.delay.Next(part, node),
			part: part, node: node, pos: pos, w: w,
		})
	}
}

func (t *Transport) DeliverUpTo(now int, c *store.Cluster) {
	var rest []message
	for _, m := range t.q {
		if m.at > now {
			rest = append(rest, m)
			continue
		}
		c.Deliver(m.part, m.node, m.pos, m.w)
	}
	t.q = rest
}

func (t *Transport) InFlight() int { return len(t.q) }
