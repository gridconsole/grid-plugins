package session

import (
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// tokenTime reads a recorded start as an RFC3339 time or as the UTC lstart
// line processid.Start writes on Windows, at second precision.
func tokenTime(token string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339Nano, token); err == nil {
		return t.UTC().Truncate(time.Second), true
	}
	if t, err := processid.ParseLstart(token, time.UTC); err == nil {
		return t, true
	}
	return time.Time{}, false
}
