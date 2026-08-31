package app

import "testing"

type recordingRepository struct {
	writeResult WriteResult
	required    RequiredPositions
}

func (r *recordingRepository) CreateThread(Thread) WriteResult { return r.writeResult }
func (r *recordingRepository) AddPost(Post) WriteResult        { return r.writeResult }
func (r *recordingRepository) ReadThread(_ ThreadID, required RequiredPositions) ReadResult {
	r.required = required
	return ReadResult{}
}
func (r *recordingRepository) Recent(_ int, required RequiredPositions) ReadResult {
	r.required = required
	return ReadResult{}
}

func TestApplicationCarriesWritePositionIntoNextRead(t *testing.T) {
	repository := &recordingRepository{
		writeResult: WriteResult{Partition: 2, Position: 42},
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
	session.RecordWrite(WriteResult{Partition: 1, Position: 10})
	session.RecordWrite(WriteResult{Partition: 1, Position: 7})

	if got := session.RequiredPositions[1]; got != 10 {
		t.Fatalf("必要位置が後退した: got=%d want=10", got)
	}
}
