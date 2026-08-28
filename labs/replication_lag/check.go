package replicationlag

import "fmt"

type EventKind int

const (
	EventWrite EventKind = iota
	EventRead
)

type Event struct {
	Kind    EventKind
	User    string
	Comment Comment
	Replica int
	Visible []Comment
}

func checkReadAfterWrite(history []Event) error {
	var mine []Comment
	for _, e := range history {
		switch e.Kind {
		case EventWrite:
			mine = append(mine, e.Comment)
		case EventRead:
			for _, w := range mine {
				if w.User != e.User {
					continue
				}
				if !containsComment(e.Visible, w) {
					return fmt.Errorf("read-after-write: user %q wrote %q then read replica %d without it", e.User, w.Text, e.Replica)
				}
			}
		}
	}
	return nil
}

func containsComment(list []Comment, want Comment) bool {
	for _, c := range list {
		if c == want {
			return true
		}
	}
	return false
}
