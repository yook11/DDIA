package replicationlag

// PostComment はリーダーへコメントを足す。ラグもレプリカ選択も知らない。
func PostComment(c *Cluster, user, text string) Comment {
	comment := Comment{User: user, Text: text}
	c.Leader.Append(comment)
	return comment
}

// ViewThread は指定したフォロワーからスレッドを読む。追いついているかは見ない。
func ViewThread(c *Cluster, replica int) []Comment {
	return c.Followers[replica].Read()
}
