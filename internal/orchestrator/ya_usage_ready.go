package orchestrator

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// yaUsageReady says whether a Job parked on usage_limit is offered again. The
// holder check of UsageReady stays first. Then the reset time the worker put in
// the exit code (unix seconds) decides: at or before now, the Job is Ready.
// Otherwise an account with room makes it Ready. Otherwise it stays Parked while
// a reset time in the future or the account numbers are known; with neither, it
// is Ready as it was before the numbers were read. The reset time is Claude
// Code's; Grid sets no number of its own.
func (s Store) yaUsageReady(j *contract.Job) bool {
	if s.UsageReady == nil || !s.UsageReady(j.Assign.To) {
		return false
	}
	reset, hasReset := yaResetTime(j.ExitCode)
	if hasReset && reset <= s.now().Unix() {
		return true
	}
	known := false
	if s.AccountRoom != nil {
		var room bool
		if room, known = s.AccountRoom(); room {
			return true
		}
	}
	return !hasReset && !known
}

// yaResetTime reads a folded exit code as a reset time in unix seconds: a JSON
// number or a numeric string. Any other code (a reason such as prepare_failed,
// no code, zero, a negative number) is no reset time.
func yaResetTime(raw json.RawMessage) (int64, bool) {
	text := strings.TrimSpace(gqCauseText(raw))
	if text == "" {
		return 0, false
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}
