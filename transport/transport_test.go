package transport

import (
	"testing"

	"ddia/database"
)

type fixedDelay int

func (d fixedDelay) Next(database.Location) int { return int(d) }

func TestSendDeliversOnlySpecifiedDestination(t *testing.T) {
	transport := New(fixedDelay(2))
	message := Message{
		Source:      database.Location{Partition: 1, Node: 0},
		Destination: database.Location{Partition: 1, Node: 2},
		Position:    3,
	}

	transport.Send(10, message)
	if got := transport.InFlight(); got != 1 {
		t.Fatalf("Sendが増やしたメッセージ数: got=%d want=1", got)
	}
	if got := transport.DeliverUpTo(11); len(got) != 0 {
		t.Fatalf("予定時刻より前に配送された: %v", got)
	}
	delivered := transport.DeliverUpTo(12)
	if len(delivered) != 1 {
		t.Fatalf("配送件数: got=%d want=1", len(delivered))
	}
	if got := delivered[0].Destination; got != message.Destination {
		t.Fatalf("送信先が変わった: got=%v want=%v", got, message.Destination)
	}
}
