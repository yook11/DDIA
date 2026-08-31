package transport

import (
	"ddia/app"
	"ddia/database"
	"ddia/storage"
)

// ノード間の配送。遅延・喪失・順序の乱れはすべてここの方針になる。
// リーダーが位置つきで押し込む（push）。喪失時に再送する責任もリーダー側にある。

// Message は1つの送信元から1つの送信先へ運ぶRecord。
// 複製先はすでに決定済みであり、Transportは増減させない。
type Message struct {
	Source      database.Location
	Destination database.Location
	Position    app.Position
	Record      storage.Record
}

type scheduledMessage struct {
	at      int
	message Message
}

type Transport struct {
	q     []scheduledMessage
	delay Delay
}

func New(d Delay) *Transport { return &Transport{delay: d} }

// Send は指定された1宛先への配送だけをキューへ積む。
func (t *Transport) Send(now int, message Message) {
	t.q = append(t.q, scheduledMessage{
		at:      now + t.delay.Next(message.Destination),
		message: message,
	})
}

// DeliverUpTo は時刻までに配送可能になったメッセージを返す。
// 宛先のStorageへ適用する責任は呼び出し側にある。
func (t *Transport) DeliverUpTo(now int) []Message {
	var delivered []Message
	var rest []scheduledMessage
	for _, scheduled := range t.q {
		if scheduled.at > now {
			rest = append(rest, scheduled)
			continue
		}
		delivered = append(delivered, scheduled.message)
	}
	t.q = rest
	return delivered
}

func (t *Transport) InFlight() int { return len(t.q) }
