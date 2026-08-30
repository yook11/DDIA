package bbs

type Recorder struct{ events []Event }

func (r *Recorder) Add(e Event) { r.events = append(r.events, e) }

func (r *Recorder) History() []Event { return r.events }
