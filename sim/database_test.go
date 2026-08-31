package sim

import (
	"testing"

	"ddia/app"
	"ddia/database"
	"ddia/storage"
)

func databaseWithFollowerPositions(positions ...int) *Database {
	cluster := storage.NewCluster(1, len(positions))
	records := []storage.Record{
		{Post: &app.Post{ID: "p1", ThreadID: "t1", Body: "one"}},
		{Post: &app.Post{ID: "p2", ThreadID: "t1", Body: "two"}},
	}
	leader := database.Location{Partition: 0, Node: 0}
	for _, record := range records {
		cluster.Append(leader, record)
	}
	for follower, applied := range positions {
		location := database.Location{Partition: 0, Node: database.NodeID(follower + 1)}
		for pos := 1; pos <= applied; pos++ {
			cluster.Apply(location, app.Position(pos), records[pos-1])
		}
	}
	return &Database{
		s:      &Sim{c: cluster},
		router: database.NewReadYourWritesRouter(database.Single{}),
	}
}

func TestDatabaseReadsFromCaughtUpFollowerSelectedByRouter(t *testing.T) {
	database := databaseWithFollowerPositions(1, 2)

	result := database.ReadThread("t1", app.RequiredPositions{0: 2})

	if got := result.Nodes[0]; got != 2 {
		t.Fatalf("追いついたフォロワーへルーティングしなかった: got=%d want=2", got)
	}
	if got := len(result.Posts); got != 2 {
		t.Fatalf("選択先から読み取った投稿数: got=%d want=2", got)
	}
}

func TestDatabaseReadsFromLeaderWhenFollowersLag(t *testing.T) {
	database := databaseWithFollowerPositions(1, 1)

	result := database.ReadThread("t1", app.RequiredPositions{0: 2})

	if got := result.Nodes[0]; got != 0 {
		t.Fatalf("リーダーへルーティングしなかった: got=%d want=0", got)
	}
	if got := len(result.Posts); got != 2 {
		t.Fatalf("リーダーから読み取った投稿数: got=%d want=2", got)
	}
}
