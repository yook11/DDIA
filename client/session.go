package client

import "ddia/bbs"

type Session struct {
	User bbs.UserID

	// LastWritePos はパーティションごとの、自分が最後に書いた論理位置。
	// レプリカの Applied がここに追いついていれば、自分の書き込みは見えている。
	LastWritePos map[int]int

	// Sticky はパーティションごとの固定先。
	Sticky map[int]int
}

func NewSession(u bbs.UserID) Session {
	return Session{User: u, LastWritePos: map[int]int{}, Sticky: map[int]int{}}
}
