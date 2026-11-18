package orchestrator

import (
	"fmt"
	"strings"
)

// qtPkWhy words the failures that parked a Job, one "Attempt N: <words>" part
// per id, in order. The words come from the record the id names; an id that is
// not in the snapshot stays as the id, so nothing is lost.
func qtPkWhy(snap Snapshot, failures []string) string {
	parts := make([]string, 0, len(failures))
	for i, id := range failures {
		parts = append(parts, fmt.Sprintf("Attempt %d: %s", i+1, qtPkWords(snap, id)))
	}
	return strings.Join(parts, "; ")
}

// qtPkWords says what one failure id was. A worker-exit gives its reason and
// code; any other record (the assign post of a dead lease, a stale control's
// target) means the agent never reported an exit.
func qtPkWords(snap Snapshot, id string) string {
	r, ok := snap.Records[id]
	if !ok {
		return id
	}
	if r.Type != "worker-exit" {
		return "the agent stopped answering"
	}
	// A worker-exit with no reason reads as crash, as the fold reads it.
	reason := stringValue(r.Body["reason"])
	if reason == "" {
		reason = "crash"
	}
	if reason == "done" {
		// Only an empty finish is ever a failure: the agent ended and posted no result.
		return "ended without posting a result"
	}
	code := exitCodeText(r.Body["code"])
	_, textual := r.Body["code"].(string)
	if reason == "crash" {
		switch {
		case code == "":
			return "crashed, no exit code reported"
		case textual:
			return code
		default:
			return "crashed, exit " + code
		}
	}
	switch {
	case code == "":
		return reason
	case textual:
		return reason + ", " + code
	default:
		return reason + ", exit " + code
	}
}
