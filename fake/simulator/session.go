package simulator

import (
	"ddia/app/user"
	"ddia/fake/database"
)

// Session はユーザーと、そのユーザーが次の読み取りで最低限見る必要がある位置を持つ。
// どのレプリカへ接続するかは記憶せず、接続先の判断はRepository側のRouterへ委ねる。
type Session struct {
	User              user.ID
	RequiredPositions database.RequiredPositions
}

func NewSession(userID user.ID) *Session {
	return &Session{User: userID, RequiredPositions: database.RequiredPositions{}}
}

func (s *Session) RecordWrite(result database.WriteResult) {
	if s.RequiredPositions[result.Partition] < result.Position {
		s.RequiredPositions[result.Partition] = result.Position
	}
}
