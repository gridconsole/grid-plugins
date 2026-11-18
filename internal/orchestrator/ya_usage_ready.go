package orchestrator

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// yaUsageReady says whether a Job parked on usage_limit is offered again. The
// holder check of UsageReady stays first. Then the reset time the worker put in
// the exit code (unix seconds) decides: at or before now, the Job is Ready.
// Otherwise an account with room makes it Ready. Otherwise it stays Parked while
// a reset time in the future or the account numbers are known; with neither, it
// is Ready as it was before the numbers were read. The reset time is Claude
// Code's; Grid sets no number of its own. Room does not offer the Job again once its last two
// attempts ended on the same reset time: the account the worker picked had no room either, so
// only the reset time lifts the hold.
func (s Store) yaUsageReady(snap Snapshot, j *contract.Job) bool {
	if s.UsageReady == nil || !s.UsageReady(j.Assign.To) {
		return false
	}
	reset, hasReset := yaResetTime(j.ExitCode)
	if hasReset && reset <= s.now().Unix() {
		return true
	}
	known := false
	if s.AccountRoom != nil && !(hasReset && usageRepeated(snap, j)) {
		var room bool
		if room, known = s.AccountRoom(); room {
			return true
		}
	}
	return !hasReset && !known
}

// usageRepeated reports whether the Job's last two worker exits, with no other exit between, were
// both usage_limit with the same code.
func usageRepeated(snap Snapshot, j *contract.Job) bool {
	var codes []string
	for _, id := range j.History {
		r := snap.Records[id]
		if r.Type != "worker-exit" {
			continue
		}
		if stringValue(r.Body["reason"]) != "usage_limit" {
			codes = nil
			continue
		}
		codes = append(codes, fmt.Sprint(r.Body["code"]))
	}
	n := len(codes)
	return n >= 2 && codes[n-1] == codes[n-2]
}

// usageWhy is the reason a Job shows while the usage limit holds it: its own reset time when it is
// parked on the limit, else the latest of the parked Jobs' (the host's hold lifts with the last).
func (s Store) usageWhy(snap Snapshot, j *contract.Job) string {
	at := int64(0)
	if j.Status == contract.Parked {
		at, _ = yaResetTime(j.ExitCode)
	} else {
		for _, p := range snap.State.Jobs {
			if t, ok := yaResetTime(p.ExitCode); ok && p.Status == contract.Parked && p.Reason == "usage_limit" && t > at {
				at = t
			}
		}
	}
	if at == 0 {
		return "Waiting for the usage limit"
	}
	layout := "15:04"
	reset, now := time.Unix(at, 0).Local(), s.now().Local()
	if reset.YearDay() != now.YearDay() || reset.Year() != now.Year() {
		layout = "Mon 15:04"
	}
	return "Waiting for the usage limit, resets " + reset.Format(layout)
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
