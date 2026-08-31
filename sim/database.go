package sim

import (
	"sort"

	"ddia/app"
	"ddia/database"
	"ddia/storage"
	"ddia/transport"
)

// Database はシミュレーション上のDBクラスタへのアクセス口。
// Routerが決めたLocationに対する読み書きと、決定済みの複製先への送信を実行する。
type Database struct {
	s      *Sim
	router database.Router
}

func (d *Database) CreateThread(thread app.Thread) app.WriteResult {
	record := storage.Record{T: d.s.now, Thread: &thread}
	return d.append(record, database.RoutingKey{ThreadID: thread.ID})
}

func (d *Database) AddPost(post app.Post) app.WriteResult {
	record := storage.Record{T: d.s.now, Post: &post}
	return d.append(record, database.RoutingKey{ThreadID: post.ThreadID, PostID: &post.ID})
}

// append はリーダーへ書いて即返る（非同期レプリケーション）。
// ack を待つ形にすると5.1.2の同期／半同期がそのまま実験になる。まだ入れていない。
func (d *Database) append(record storage.Record, key database.RoutingKey) app.WriteResult {
	source := d.router.RouteWrite(key, d.s.c)
	position := d.s.c.Append(source, record)
	for _, destination := range d.s.r.Targets(source, d.s.c) {
		d.s.t.Send(d.s.now, transport.Message{
			Source:      source,
			Destination: destination,
			Position:    position,
			Record:      record,
		})
	}
	return app.WriteResult{Partition: source.Partition, Position: position}
}

func (d *Database) ReadThread(id app.ThreadID, required app.RequiredPositions) app.ReadResult {
	return d.read(required, func(post app.Post) bool { return post.ThreadID == id }, 0)
}

// recent は全パーティションに問い合わせてマージする。
// 1回の論理的な読み取りがN回の物理的な読み取りになり、N個の遅れ具合はそれぞれ違う。
func (d *Database) Recent(limit int, required app.RequiredPositions) app.ReadResult {
	return d.read(required, func(app.Post) bool { return true }, limit)
}

type located struct {
	post app.Post
	t    int
	part app.PartitionID
	pos  app.Position
}

func (d *Database) read(
	required app.RequiredPositions,
	keep func(app.Post) bool,
	limit int,
) app.ReadResult {
	var hits []located
	nodes := map[int]int{}
	for part := 0; part < d.s.c.Partitions(); part++ {
		partition := app.PartitionID(part)
		location := d.router.RouteRead(partition, required[partition], d.s.c)
		nodes[part] = int(location.Node)
		for i, record := range d.s.c.Log(location) {
			if record.Post == nil || !keep(*record.Post) {
				continue
			}
			hits = append(hits, located{
				post: *record.Post,
				t:    record.T,
				part: partition,
				pos:  app.Position(i + 1),
			})
		}
	}

	// マージキーは書き込み時刻。パーティションをまたぐ全体順序は存在しないので、これは近似でしかない。
	// 今は完全な時計が1つあるので順序自体は正しく、異常は「あるべきものが無い」形でだけ出る。
	sort.Slice(hits, func(i, j int) bool {
		a, b := hits[i], hits[j]
		if a.t != b.t {
			return a.t < b.t
		}
		if a.part != b.part {
			return a.part < b.part
		}
		return a.pos < b.pos
	})
	if limit > 0 && len(hits) > limit {
		hits = hits[len(hits)-limit:]
	}
	posts := make([]app.Post, 0, len(hits))
	for _, hit := range hits {
		posts = append(posts, hit.post)
	}
	return app.ReadResult{Posts: posts, Nodes: nodes}
}

var _ app.Repository = (*Database)(nil)
