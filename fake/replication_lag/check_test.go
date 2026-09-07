package replicationlag

import (
	"testing"

	"ddia/app/post"
	"ddia/fake/simulator"
)

func TestReadAfterWriteDetectsMissing(t *testing.T) {
	p := post.Post{ID: "p1", ThreadID: "t1", AuthorID: "alice", Body: "hello"}
	h := []simulator.Event{
		{Phase: simulator.Ok, User: "alice", Wrote: &p},
		{Phase: simulator.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t1"},
	}
	if err := checkReadAfterWrite(h); err == nil {
		t.Fatal("自分のレスが見えない履歴を通してしまった")
	}
}

func TestReadAfterWriteAcceptsVisible(t *testing.T) {
	p := post.Post{ID: "p1", ThreadID: "t1", AuthorID: "alice", Body: "hello"}
	h := []simulator.Event{
		{Phase: simulator.Ok, User: "alice", Wrote: &p},
		{Phase: simulator.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t1", Seen: []post.Post{p}},
	}
	if err := checkReadAfterWrite(h); err != nil {
		t.Fatal(err)
	}
}

func TestReadAfterWriteIgnoresOtherThread(t *testing.T) {
	p := post.Post{ID: "p1", ThreadID: "t1", AuthorID: "alice", Body: "hello"}
	h := []simulator.Event{
		{Phase: simulator.Ok, User: "alice", Wrote: &p},
		{Phase: simulator.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t2"},
	}
	if err := checkReadAfterWrite(h); err != nil {
		t.Fatal(err)
	}
}
