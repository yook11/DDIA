package storage

import (
	"ddia/app"
	"ddia/database"
)

// ストレージ層。乱数も時計もネットワークもimportしない。純粋な状態機械。

// replica は1パーティションの1複製。
// 順序が乱れて届いたRecordはpendingに置き、手前が埋まってから適用する。
// パーティションの中では順序が保たれる。壊れるのはまたいだときだけ。
type replica struct {
	log     []Record
	pending map[app.Position]Record
}

func newReplica() *replica { return &replica{pending: map[app.Position]Record{}} }

func (r *replica) apply(pos app.Position, record Record) {
	if pos <= app.Position(len(r.log)) {
		return // 適用済み。重複配送
	}
	r.pending[pos] = record
	for {
		next := app.Position(len(r.log) + 1)
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
func (c *Cluster) Append(location database.Location, record Record) app.Position {
	replica := c.at(location)
	replica.log = append(replica.log, record)
	return app.Position(len(replica.log))
}

func (c *Cluster) Log(location database.Location) []Record { return c.at(location).log }

func (c *Cluster) Applied(location database.Location) app.Position {
	return app.Position(len(c.at(location).log))
}

// Pending は届いたのに手前が埋まらず適用できていない件数。
// ヘッドオブラインブロッキングの観測用。
func (c *Cluster) Pending(location database.Location) int {
	return len(c.at(location).pending)
}

func (c *Cluster) Apply(location database.Location, pos app.Position, record Record) {
	c.at(location).apply(pos, record)
}

func (c *Cluster) at(location database.Location) *replica {
	return c.parts[int(location.Partition)].replicas[int(location.Node)]
}

var _ database.Topology = (*Cluster)(nil)
