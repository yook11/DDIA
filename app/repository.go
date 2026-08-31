package app

// ReadResult はDB読み取りの結果と、実験でどこを読んだかを表す。
// Nodes は観測用であり、Applicationが接続先を決めるためには使わない。
type ReadResult struct {
	Posts []Post
	Nodes map[int]int
}

// Repository はアプリケーションから見たデータベース操作の境界。
// Applicationはレプリカ、遅延、配送方法を知らない。
type Repository interface {
	CreateThread(Thread) WriteResult
	AddPost(Post) WriteResult
	ReadThread(ThreadID, RequiredPositions) ReadResult
	Recent(limit int, required RequiredPositions) ReadResult
}
