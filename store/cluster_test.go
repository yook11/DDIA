package store

import (
	"testing"

	"ddia/bbs"
)

// フォロワーは順序が乱れて届いても、手前が埋まるまで適用しない。
// パーティションの中で順序が保たれるのはこの性質による。
func TestReplicaAppliesInOrder(t *testing.T) {
	r := newReplica()
	w := func(body string) Write { return Write{Post: &bbs.Post{Body: body}} }

	r.deliver(3, w("c"))
	r.deliver(2, w("b"))
	if len(r.log) != 0 {
		t.Fatalf("手前が届く前に適用された: %d 件", len(r.log))
	}

	r.deliver(1, w("a"))
	if got := len(r.log); got != 3 {
		t.Fatalf("追いついていない: %d 件", got)
	}
	for i, want := range []string{"a", "b", "c"} {
		if r.log[i].Post.Body != want {
			t.Fatalf("順序が違う: %v", r.log)
		}
	}

	r.deliver(2, w("dup"))
	if len(r.log) != 3 {
		t.Fatal("重複配送で増えた")
	}
}
