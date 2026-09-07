package database

import (
	"ddia/app/post"
	"ddia/app/thread"
)

type PartitionID int
type Position int

type WriteResult struct {
	Partition PartitionID
	Position  Position
}

type RequiredPositions map[PartitionID]Position

// NodeID は、1つのパーティションを複製しているDBノードの識別子。
// 現在のシミュレーションでは0がリーダー、1以降がフォロワー。
type NodeID int

// Location は、クラスタ上の1つの保存先を表す。
// Routerはこの値を返し、StorageとTransportは指定された値をそのまま使う。
type Location struct {
	Partition PartitionID
	Node      NodeID
}

// Topology は、Routerが接続先を決めるために参照できるDBクラスタの状態。
// RouterはStorageの具体的な実装を知らない。
type Topology interface {
	Partitions() int
	Replicas() int
	Applied(Location) Position
}

// RoutingKey は、書き込み先パーティションを決めるために必要な情報だけを持つ。
// PostIDがnilならスレッド作成、値があればレスの書き込みを表す。
type RoutingKey struct {
	ThreadID thread.ID
	PostID   *post.ID
}
