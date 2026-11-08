package hook

import "strings"

// Event describes one implemented dispatcher and its plugin registration.
type Event struct {
	Name    string
	Matcher string
	Timeout int
}

// This table is the source for dispatch, Handles and generated plugin hooks.
var events = []struct {
	Event
	run func(Options)
}{
	{Event{"SessionStart", "startup|resume|clear|compact|fork", 10}, sessionStart},
	{Event{"PreToolUse", "", 10}, toolUse},
	{Event{"PermissionRequest", "", 10}, toolUse},
}

// Events returns registration metadata without exposing the dispatcher table.
func Events() []Event {
	out := make([]Event, 0, len(events))
	for _, e := range events {
		out = append(out, e.Event)
	}
	return out
}

func handler(name string) func(Options) {
	for _, e := range events {
		if strings.EqualFold(name, e.Name) {
			return e.run
		}
	}
	return nil
}
