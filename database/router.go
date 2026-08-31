package database

import "ddia/app"

// Router は、書き込みまたは読み取りを行う最終的なDB位置を決める。
// 候補だけを返したり、選んだ場所への読み書きを実行したりはしない。
type Router interface {
	RouteWrite(RoutingKey, Topology) Location
	RouteRead(app.PartitionID, app.Position, Topology) Location
}

// Rand は乱数の供給元。Router自身は乱数を作らない。
type Rand interface {
	Intn(n int) int
}

type routerBase struct {
	partitioner Partitioner
}

func newRouterBase(partitioner Partitioner) routerBase {
	if partitioner == nil {
		partitioner = Single{}
	}
	return routerBase{partitioner: partitioner}
}

func (r routerBase) routeWrite(key RoutingKey, topology Topology) Location {
	return Location{
		Partition: r.partitioner.Partition(key, topology.Partitions()),
		Node:      0,
	}
}

// LeaderRouter は書き込みも読み取りもリーダーへ送る。
type LeaderRouter struct{ routerBase }

func NewLeaderRouter(partitioner Partitioner) *LeaderRouter {
	return &LeaderRouter{routerBase: newRouterBase(partitioner)}
}

func (r *LeaderRouter) RouteWrite(key RoutingKey, topology Topology) Location {
	return r.routeWrite(key, topology)
}

func (*LeaderRouter) RouteRead(partition app.PartitionID, _ app.Position, _ Topology) Location {
	return Location{Partition: partition, Node: 0}
}

// RandomRouter は読み取り先を毎回ランダムに選ぶ。対策なしのルーティング。
type RandomRouter struct {
	routerBase
	rng Rand
}

func NewRandomRouter(partitioner Partitioner, rng Rand) *RandomRouter {
	return &RandomRouter{routerBase: newRouterBase(partitioner), rng: rng}
}

func (r *RandomRouter) RouteWrite(key RoutingKey, topology Topology) Location {
	return r.routeWrite(key, topology)
}

func (r *RandomRouter) RouteRead(
	partition app.PartitionID,
	_ app.Position,
	topology Topology,
) Location {
	return Location{Partition: partition, Node: NodeID(r.rng.Intn(topology.Replicas()))}
}

// ReadYourWritesRouter は、セッションが必要とする位置まで進んだフォロワーを選ぶ。
// 条件を満たすフォロワーがなければ、書き込みを持つリーダーへ送る。
type ReadYourWritesRouter struct{ routerBase }

func NewReadYourWritesRouter(partitioner Partitioner) *ReadYourWritesRouter {
	return &ReadYourWritesRouter{routerBase: newRouterBase(partitioner)}
}

func (r *ReadYourWritesRouter) RouteWrite(key RoutingKey, topology Topology) Location {
	return r.routeWrite(key, topology)
}

func (*ReadYourWritesRouter) RouteRead(
	partition app.PartitionID,
	required app.Position,
	topology Topology,
) Location {
	for node := topology.Replicas() - 1; node >= 1; node-- {
		location := Location{Partition: partition, Node: NodeID(node)}
		if topology.Applied(location) >= required {
			return location
		}
	}
	return Location{Partition: partition, Node: 0}
}

var (
	_ Router = (*LeaderRouter)(nil)
	_ Router = (*RandomRouter)(nil)
	_ Router = (*ReadYourWritesRouter)(nil)
)
