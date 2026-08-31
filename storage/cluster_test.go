package storage

import (
	"testing"

	"ddia/app"
	"ddia/database"
)

// フォロワーは順序が乱れて届いても、手前が埋まるまで適用しない。
// パーティションの中で順序が保たれるのはこの性質による。
func TestReplicaAppliesInOrder(t *testing.T) {
	replica := newReplica()
	record := func(body string) Record { return Record{Post: &app.Post{Body: body}} }

	replica.apply(3, record("c"))
	replica.apply(2, record("b"))
	if len(replica.log) != 0 {
		t.Fatalf("手前が届く前に適用された: %d件", len(replica.log))
	}

	replica.apply(1, record("a"))
	if got := len(replica.log); got != 3 {
		t.Fatalf("追いついていない: %d件", got)
	}
	for i, want := range []string{"a", "b", "c"} {
		if replica.log[i].Post.Body != want {
			t.Fatalf("順序が違う: %v", replica.log)
		}
	}

	replica.apply(2, record("dup"))
	if len(replica.log) != 3 {
		t.Fatal("重複配送で増えた")
	}
}

func TestClusterAppendsOnlyAtSpecifiedLocation(t *testing.T) {
	cluster := NewCluster(2, 1)
	target := database.Location{Partition: 1, Node: 0}
	record := Record{Post: &app.Post{ID: "p1", ThreadID: "t1"}}

	position := cluster.Append(target, record)

	if position != 1 {
		t.Fatalf("書き込み位置: got=%d want=1", position)
	}
	if got := cluster.Applied(target); got != 1 {
		t.Fatalf("指定先の適用位置: got=%d want=1", got)
	}
	other := database.Location{Partition: 0, Node: 0}
	if got := cluster.Applied(other); got != 0 {
		t.Fatalf("指定していない保存先にも書かれた: got=%d want=0", got)
	}
}
