package database

// Replicator は、1件の書き込みをどのDB位置へ複製するか決める。
// 実際の配送はTransportへ委ねる。
type Replicator interface {
	Targets(source Location, topology Topology) []Location
}

// AllFollowersReplicator は、同じパーティションの全フォロワーを複製先にする。
type AllFollowersReplicator struct{}

func (AllFollowersReplicator) Targets(source Location, topology Topology) []Location {
	targets := make([]Location, 0, topology.Replicas()-1)
	for node := 1; node < topology.Replicas(); node++ {
		targets = append(targets, Location{
			Partition: source.Partition,
			Node:      NodeID(node),
		})
	}
	return targets
}

var _ Replicator = AllFollowersReplicator{}
