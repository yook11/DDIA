package app

// PartitionID と Position は、アプリケーションがDBから受け取る整合性トークン。
// Position の実体はシミュレーションではログ位置、実DBではWALやGTIDなどになりうる。
type PartitionID int
type Position int

type WriteResult struct {
	Partition PartitionID
	Position  Position
}

type RequiredPositions map[PartitionID]Position

// Session はユーザーと、そのユーザーが次の読み取りで最低限見る必要がある位置を持つ。
// どのレプリカへ接続するかは記憶せず、接続先の判断はRepository側のRouterへ委ねる。
type Session struct {
	User              UserID
	RequiredPositions RequiredPositions
}

func NewSession(user UserID) *Session {
	return &Session{User: user, RequiredPositions: RequiredPositions{}}
}

func (s *Session) RecordWrite(result WriteResult) {
	if s.RequiredPositions[result.Partition] < result.Position {
		s.RequiredPositions[result.Partition] = result.Position
	}
}
