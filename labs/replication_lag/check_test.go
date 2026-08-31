package replicationlag

import (
	"testing"

	"ddia/app"
)

func TestReadAfterWriteDetectsMissing(t *testing.T) {
	p := app.Post{ID: "p1", ThreadID: "t1", Author: "alice", Body: "hello"}
	h := []app.Event{
		{Phase: app.Ok, User: "alice", Wrote: &p},
		{Phase: app.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t1"},
	}
	if err := checkReadAfterWrite(h); err == nil {
		t.Fatal("自分のレスが見えない履歴を通してしまった")
	}
}

func TestReadAfterWriteAcceptsVisible(t *testing.T) {
	p := app.Post{ID: "p1", ThreadID: "t1", Author: "alice", Body: "hello"}
	h := []app.Event{
		{Phase: app.Ok, User: "alice", Wrote: &p},
		{Phase: app.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t1", Seen: []app.Post{p}},
	}
	if err := checkReadAfterWrite(h); err != nil {
		t.Fatal(err)
	}
}

func TestReadAfterWriteIgnoresOtherThread(t *testing.T) {
	p := app.Post{ID: "p1", ThreadID: "t1", Author: "alice", Body: "hello"}
	h := []app.Event{
		{Phase: app.Ok, User: "alice", Wrote: &p},
		{Phase: app.Ok, User: "alice", Op: "GET /threads/{t}", Thread: "t2"},
	}
	if err := checkReadAfterWrite(h); err != nil {
		t.Fatal(err)
	}
}
