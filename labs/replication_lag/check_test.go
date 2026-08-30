package replicationlag

import (
	"testing"

	"ddia/bbs"
)

func TestReadAfterWriteDetectsMissing(t *testing.T) {
	p := bbs.Post{ID: "p1", ThreadID: "t1", Author: "alice", Body: "hello"}
	h := []bbs.Event{
		{Phase: bbs.Ok, User: "alice", Wrote: &p},
		{Phase: bbs.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t1"},
	}
	if err := checkReadAfterWrite(h); err == nil {
		t.Fatal("自分のレスが見えない履歴を通してしまった")
	}
}

func TestReadAfterWriteAcceptsVisible(t *testing.T) {
	p := bbs.Post{ID: "p1", ThreadID: "t1", Author: "alice", Body: "hello"}
	h := []bbs.Event{
		{Phase: bbs.Ok, User: "alice", Wrote: &p},
		{Phase: bbs.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t1", Seen: []bbs.Post{p}},
	}
	if err := checkReadAfterWrite(h); err != nil {
		t.Fatal(err)
	}
}

func TestReadAfterWriteIgnoresOtherThread(t *testing.T) {
	p := bbs.Post{ID: "p1", ThreadID: "t1", Author: "alice", Body: "hello"}
	h := []bbs.Event{
		{Phase: bbs.Ok, User: "alice", Wrote: &p},
		{Phase: bbs.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t2"},
	}
	if err := checkReadAfterWrite(h); err != nil {
		t.Fatal(err)
	}
}
