package replicationlag

import (
	"fmt"

	"ddia/app"
)

// checkReadAfterWrite は、同じ人が書いたレスが、その人が同じスレを開いたときに見えているか。
// 履歴だけを見る。リーダーの真値は見ない。
func checkReadAfterWrite(h []app.Event) error {
	wrote := map[app.UserID][]app.Post{}
	for _, e := range h {
		if e.Phase != app.Ok {
			continue
		}
		if e.Wrote != nil {
			wrote[e.User] = append(wrote[e.User], *e.Wrote)
			continue
		}
		if e.Op != "GET /threads/{t}" {
			continue
		}
		for _, p := range wrote[e.User] {
			if p.ThreadID != e.Thread {
				continue
			}
			if !containsPost(e.Seen, p.ID) {
				return fmt.Errorf("read-after-write: user %q が %q を書いたあと、スレ %q を開いても見えない",
					e.User, p.Body, e.Thread)
			}
		}
	}
	return nil
}

func containsPost(list []app.Post, id app.PostID) bool {
	for _, p := range list {
		if p.ID == id {
			return true
		}
	}
	return false
}
