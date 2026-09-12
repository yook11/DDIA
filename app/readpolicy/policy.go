package readpolicy

type Policy interface {
	isReadPolicy()
}

type ReadYourWrites struct{}
type Eventual struct{}

func (ReadYourWrites) isReadPolicy() {}
func (Eventual) isReadPolicy()       {}
