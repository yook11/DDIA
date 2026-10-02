package httpapi

import (
	"net/http"
	"strings"

	"ddia/app/readpolicy"
)

const (
	// writePositionHeader は書き込み応答で返す複製位置。
	writePositionHeader = "X-Write-Position"
	// requiredPositionHeader は読み取り要求で受け取る、最低限必要な複製位置。
	requiredPositionHeader = "X-Required-Position"
)

func readPolicyFromRequest(r *http.Request) readpolicy.Policy {
	position := strings.TrimSpace(r.Header.Get(requiredPositionHeader))
	if position == "" {
		return readpolicy.PrimaryOnly{}
	}
	return readpolicy.ReadYourWrites{WritePosition: readpolicy.Position(position)}
}
