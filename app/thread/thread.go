package thread

import "ddia/app/user"

type ID string

type Thread struct {
	ID       ID
	Title    string
	AuthorID user.ID
}
