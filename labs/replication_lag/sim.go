package replicationlag

import "fmt"

func cloneComments(in []Comment) []Comment {
	if len(in) == 0 {
		return nil
	}
	out := make([]Comment, len(in))
	copy(out, in)
	return out
}

// Simulate は投稿と閲覧を普通に呼び、合間にスケジューラがフォロワーを進める。
func Simulate(seed int64, ops int) []Event {
	c := NewCluster(3)
	s := NewScheduler(seed, c)
	var hist []Event
	seq := 0
	for i := 0; i < ops; i++ {
		s.Tick()
		if s.rng.Intn(2) == 0 {
			seq++
			comment := PostComment(c, "alice", fmt.Sprintf("hello-%d", seq))
			hist = append(hist, Event{Kind: EventWrite, User: "alice", Comment: comment})
			continue
		}
		replica := s.PickReplica()
		visible := cloneComments(ViewThread(c, replica))
		hist = append(hist, Event{
			Kind:    EventRead,
			User:    "alice",
			Replica: replica,
			Visible: visible,
		})
	}
	return hist
}
