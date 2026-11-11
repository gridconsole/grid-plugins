package braincmd

import (
	"encoding/json"
	"fmt"
)

// roleRefused prints the refusal of a caller role a brain verb does not allow
// and returns exit 3. Stderr gets the line "grid brain <verb>: caller role is
// not allowed to <what>" with or without --json; with --json stdout also gets
// the person_required object the contract gives person-gate refusals, with the
// reason role_not_allowed, the line after the verb prefix as its hint and the
// whole line as its message.
func roleRefused(env Env, verb, what string, asJSON bool) int {
	hint := "caller role is not allowed to " + what
	line := "grid brain " + verb + ": " + hint
	fmt.Fprintln(env.Stderr, line)
	if asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(struct {
			Error   string `json:"error"`
			Reason  string `json:"reason"`
			Hint    string `json:"hint"`
			Code    string `json:"code"`
			Message string `json:"message"`
		}{"person_required", "role_not_allowed", hint, "person_required", line})
	}
	return 3
}
