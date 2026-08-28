package replicationlag

import (
	"reflect"
	"testing"
)

func TestCheckReadAfterWriteDetectsMissingComment(t *testing.T) {
	hist := []Event{
		{Kind: EventWrite, User: "alice", Comment: Comment{User: "alice", Text: "hello"}},
		{Kind: EventRead, User: "alice", Replica: 2},
	}
	if err := checkReadAfterWrite(hist); err == nil {
		t.Fatal("want read-after-write violation")
	}
}

func TestCheckReadAfterWriteAcceptsVisibleComment(t *testing.T) {
	c := Comment{User: "alice", Text: "hello"}
	hist := []Event{
		{Kind: EventWrite, User: "alice", Comment: c},
		{Kind: EventRead, User: "alice", Replica: 0, Visible: []Comment{c}},
	}
	if err := checkReadAfterWrite(hist); err != nil {
		t.Fatal(err)
	}
}

func TestSimulateIsDeterministic(t *testing.T) {
	a := Simulate(1732, 20)
	b := Simulate(1732, 20)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed must replay the same history")
	}
}

func TestNaiveAppReadAfterWriteViolations(t *testing.T) {
	const n = 100
	const ops = 30
	first := int64(-1)
	var example error
	count := 0
	for seed := int64(0); seed < n; seed++ {
		hist := Simulate(seed, ops)
		if err := checkReadAfterWrite(hist); err != nil {
			count++
			if first < 0 {
				first = seed
				example = err
			}
		}
	}
	if count == 0 {
		t.Fatal("want some read-after-write violations from naive ViewThread")
	}
	t.Logf("violations: %d / %d seeds; first seed=%d: %v", count, n, first, example)
}
