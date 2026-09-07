package database

import (
	"hash/fnv"
)

// Partitioner は、RoutingKeyをどのパーティションへ送るか決めるRouterの部品。
type Partitioner interface {
	Partition(RoutingKey, int) PartitionID
}

// Single はすべての書き込みをパーティション0へ送る。
type Single struct{}

func (Single) Partition(RoutingKey, int) PartitionID { return 0 }

// ByThread はスレッド単位で分ける。親レスと返信は同じ保存先になる。
type ByThread struct{}

func (ByThread) Partition(key RoutingKey, partitions int) PartitionID {
	return shard(string(key.ThreadID), partitions)
}

// ByPost はレス単位で分ける。スレッド作成だけはThreadIDで分ける。
type ByPost struct{}

func (ByPost) Partition(key RoutingKey, partitions int) PartitionID {
	if key.PostID != nil {
		return shard(string(*key.PostID), partitions)
	}
	return shard(string(key.ThreadID), partitions)
}

func shard(key string, partitions int) PartitionID {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return PartitionID(int(hash.Sum32()) % partitions)
}
