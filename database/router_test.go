package database

import (
	"testing"

	"ddia/app"
)

type topologyStub struct {
	partitions int
	replicas   int
	applied    map[Location]app.Position
}

func (t topologyStub) Partitions() int { return t.partitions }
func (t topologyStub) Replicas() int   { return t.replicas }
func (t topologyStub) Applied(location Location) app.Position {
	return t.applied[location]
}

func TestRouteWriteUsesPartitionerAndLeader(t *testing.T) {
	topology := topologyStub{partitions: 4, replicas: 3}
	postID := app.PostID("p1")
	key := RoutingKey{ThreadID: "t1", PostID: &postID}

	for _, partitioner := range []Partitioner{Single{}, ByThread{}, ByPost{}} {
		router := NewLeaderRouter(partitioner)
		location := router.RouteWrite(key, topology)
		wantPartition := partitioner.Partition(key, topology.Partitions())

		if location.Partition != wantPartition {
			t.Fatalf("%Tの書き込み先: got=%d want=%d", partitioner, location.Partition, wantPartition)
		}
		if location.Node != 0 {
			t.Fatalf("%Tがリーダー以外へ書いた: node=%d", partitioner, location.Node)
		}
	}
}

func TestReadYourWritesRouterSelectsCaughtUpFollower(t *testing.T) {
	topology := topologyStub{
		partitions: 1,
		replicas:   3,
		applied: map[Location]app.Position{
			{Partition: 0, Node: 0}: 2,
			{Partition: 0, Node: 1}: 1,
			{Partition: 0, Node: 2}: 2,
		},
	}
	router := NewReadYourWritesRouter(Single{})

	got := router.RouteRead(0, 2, topology)

	want := (Location{Partition: 0, Node: 2})
	if got != want {
		t.Fatalf("読み取り先: got=%v want=%v", got, want)
	}
}

func TestReadYourWritesRouterFallsBackToLeader(t *testing.T) {
	topology := topologyStub{
		partitions: 1,
		replicas:   3,
		applied: map[Location]app.Position{
			{Partition: 0, Node: 0}: 2,
			{Partition: 0, Node: 1}: 1,
			{Partition: 0, Node: 2}: 1,
		},
	}
	router := NewReadYourWritesRouter(Single{})

	got := router.RouteRead(0, 2, topology)

	want := (Location{Partition: 0, Node: 0})
	if got != want {
		t.Fatalf("読み取り先: got=%v want=%v", got, want)
	}
}

func TestAllFollowersReplicatorReturnsEveryFollower(t *testing.T) {
	topology := topologyStub{partitions: 2, replicas: 4}
	source := Location{Partition: 1, Node: 0}

	got := (AllFollowersReplicator{}).Targets(source, topology)
	want := []Location{
		{Partition: 1, Node: 1},
		{Partition: 1, Node: 2},
		{Partition: 1, Node: 3},
	}

	if len(got) != len(want) {
		t.Fatalf("複製先数: got=%d want=%d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("複製先[%d]: got=%v want=%v", i, got[i], want[i])
		}
	}
}
