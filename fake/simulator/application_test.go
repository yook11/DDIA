package simulator

import (
	"testing"

	"ddia/app/post"
	"ddia/app/thread"
	"ddia/fake/database"
)

type recordingRepository struct {
	writeResult database.WriteResult
	required    database.RequiredPositions
}

func (r *recordingRepository) CreateThread(thread.Thread) database.WriteResult { return r.writeResult }
func (r *recordingRepository) AddPost(post.Post) database.WriteResult          { return r.writeResult }
func (r *recordingRepository) ReadThread(_ thread.ID, required database.RequiredPositions) ReadResult {
	r.required = required
	return ReadResult{}
}
func (r *recordingRepository) Recent(_ int, required database.RequiredPositions) ReadResult {
	r.required = required
	return ReadResult{}
}

func TestApplicationCarriesWritePositionIntoNextRead(t *testing.T) {
	repository := &recordingRepository{
		writeResult: database.WriteResult{Partition: 2, Position: 42},
	}
	application := New(repository, &Recorder{}, func() int { return 0 }, NewIDs())
	session := NewSession("alice")

	thread := application.CreateThread(session, "スレ")
	application.ViewThread(session, thread.ID)

	if got := repository.required[2]; got != 42 {
		t.Fatalf("DBへ渡した必要位置: got=%d want=42", got)
	}
}

func TestSessionDoesNotMoveRequiredPositionBackward(t *testing.T) {
	session := NewSession("alice")
	session.RecordWrite(database.WriteResult{Partition: 1, Position: 10})
	session.RecordWrite(database.WriteResult{Partition: 1, Position: 7})

	if got := session.RequiredPositions[1]; got != 10 {
		t.Fatalf("必要位置が後退した: got=%d want=10", got)
	}
}
