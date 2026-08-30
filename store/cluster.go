package store

// 保存層。乱数も時計もネットワークも import しない。純粋な状態機械。

// replica は 1 パーティションの 1 複製。
// 順序が乱れて届いた分は pending に置き、手前が埋まってから適用する。
// パーティションの中では順序が保たれる。壊れるのはまたいだときだけ。
type replica struct {
	log     []Write
	pending map[int]Write
}

func newReplica() *replica { return &replica{pending: map[int]Write{}} }

func (r *replica) deliver(pos int, w Write) {
	if pos <= len(r.log) {
		return // 適用済み。重複配送
	}
	r.pending[pos] = w
	for {
		next := len(r.log) + 1
		w, ok := r.pending[next]
		if !ok {
			return
		}
		r.log = append(r.log, w)
		delete(r.pending, next)
	}
}

type partition struct {
	replicas []*replica // 0 がリーダー
}

type Cluster struct {
	parts []*partition
	p     Partitioner
}

func NewCluster(partitions, followers int, p Partitioner) *Cluster {
	c := &Cluster{p: p}
	for i := 0; i < partitions; i++ {
		part := &partition{}
		for n := 0; n < 1+followers; n++ {
			part.replicas = append(part.replicas, newReplica())
		}
		c.parts = append(c.parts, part)
	}
	return c
}

func (c *Cluster) Partitions() int { return len(c.parts) }

// Replicas はリーダーを含む複製数。node 0 がリーダー。
func (c *Cluster) Replicas() int { return len(c.parts[0].replicas) }

// Append はリーダーのログ末尾に足し、パーティションと論理位置（1 始まり）を返す。
// 複製はしない。誰にいつ届けるかは transport の仕事。
func (c *Cluster) Append(w Write) (part, pos int) {
	part = c.p.Partition(w, len(c.parts))
	leader := c.parts[part].replicas[0]
	leader.log = append(leader.log, w)
	return part, len(leader.log)
}

func (c *Cluster) Log(part, node int) []Write { return c.parts[part].replicas[node].log }

func (c *Cluster) Applied(part, node int) int { return len(c.parts[part].replicas[node].log) }

// Pending は届いたのに手前が埋まらず適用できていない件数。
// ヘッドオブラインブロッキングの観測用。
func (c *Cluster) Pending(part, node int) int { return len(c.parts[part].replicas[node].pending) }

func (c *Cluster) Deliver(part, node, pos int, w Write) {
	c.parts[part].replicas[node].deliver(pos, w)
}
