package httpapi

import (
	"net/http"
	"strings"

	"ddia/app/readpolicy"
)

const requiredPositionHeader = "X-Required-Position"

func readPolicyFromRequest(r *http.Request) readpolicy.Policy {
	position := strings.TrimSpace(r.Header.Get(requiredPositionHeader))
	if position == "" {
		return readpolicy.PrimaryOnly{}
	}
	return readpolicy.ReadYourWrites{WritePosition: readpolicy.Position(position)}
}
