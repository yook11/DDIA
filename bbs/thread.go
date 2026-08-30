package bbs

type ThreadID string

type Thread struct {
	ID     ThreadID
	Title  string
	Author UserID
}
