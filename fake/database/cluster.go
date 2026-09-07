package database

// ストレージ層。乱数も時計もネットワークもimportしない。純粋な状態機械。

// replica は1パーティションの1複製。
// 順序が乱れて届いたRecordはpendingに置き、手前が埋まってから適用する。
// パーティションの中では順序が保たれる。壊れるのはまたいだときだけ。
type replica struct {
	log     []Record
	pending map[Position]Record
}

func newReplica() *replica { return &replica{pending: map[Position]Record{}} }

func (r *replica) apply(pos Position, record Record) {
	if pos <= Position(len(r.log)) {
		return // 適用済み。重複配送
	}
	r.pending[pos] = record
	for {
		next := Position(len(r.log) + 1)
		record, ok := r.pending[next]
		if !ok {
			return
		}
		r.log = append(r.log, record)
		delete(r.pending, next)
	}
}

type partition struct {
	replicas []*replica // 0がリーダー
}

type Cluster struct {
	parts []*partition
}

func NewCluster(partitions, followers int) *Cluster {
	cluster := &Cluster{}
	for i := 0; i < partitions; i++ {
		part := &partition{}
		for node := 0; node < 1+followers; node++ {
			part.replicas = append(part.replicas, newReplica())
		}
		cluster.parts = append(cluster.parts, part)
	}
	return cluster
}

func (c *Cluster) Partitions() int { return len(c.parts) }

// Replicas はリーダーを含む複製数。node 0がリーダー。
func (c *Cluster) Replicas() int { return len(c.parts[0].replicas) }

// Append は指定された保存先のログ末尾へRecordを足し、論理位置（1始まり）を返す。
// 保存先の選択と複製は行わない。
func (c *Cluster) Append(location Location, record Record) Position {
	replica := c.at(location)
	replica.log = append(replica.log, record)
	return Position(len(replica.log))
}

func (c *Cluster) Log(location Location) []Record { return c.at(location).log }

func (c *Cluster) Applied(location Location) Position {
	return Position(len(c.at(location).log))
}

// Pending は届いたのに手前が埋まらず適用できていない件数。
// ヘッドオブラインブロッキングの観測用。
func (c *Cluster) Pending(location Location) int {
	return len(c.at(location).pending)
}

func (c *Cluster) Apply(location Location, pos Position, record Record) {
	c.at(location).apply(pos, record)
}

func (c *Cluster) at(location Location) *replica {
	return c.parts[int(location.Partition)].replicas[int(location.Node)]
}

var _ Topology = (*Cluster)(nil)
