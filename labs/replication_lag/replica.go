package replicationlag

type Comment struct {
	User string
	Text string
}

type Leader struct {
	Log       []Comment
	NextIndex int
}

type Follower struct {
	Log []Comment // このノードが持っている書き込み。Step でリーダーから先頭順にコピーする
}

type Cluster struct {
	Leader    Leader
	Followers []Follower
}

func NewCluster(followers int) *Cluster {
	return &Cluster{
		Followers: make([]Follower, followers),
	}
}

func (leader *Leader) Append(c Comment) {
	leader.Log = append(leader.Log, c)
}

func (follower *Follower) Read() []Comment {
	return follower.Log
}

func (c *Cluster) Step(i int) {
	follower := &c.Followers[i]
	if len(c.Leader.Log) <= len(follower.Log) {
		return
	}
	follower.Log = append(follower.Log, c.Leader.Log[len(follower.Log)])
}
