package thread

import (
	"context"
	"errors"
	"testing"

	"ddia/app/readpolicy"
)

func TestGetThreadRejectsInvalidIDBeforeQuery(t *testing.T) {
	for _, threadID := range []ID{"", "abc", "0", "-1", "9223372036854775808"} {
		t.Run(string(threadID), func(t *testing.T) {
			got, err := NewRepository(nil).GetThread(context.Background(), threadID, readpolicy.ReadYourWrites{})
			if !errors.Is(err, ErrInvalidThreadID) || len(got.Posts) != 0 || got.Thread != (Thread{}) {
				t.Fatalf("GetThread(%q) = %+v, %v", threadID, got, err)
			}
		})
	}
}
