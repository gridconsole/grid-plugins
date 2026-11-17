package account

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"
)

// claudeUsageArgs and claudeUsageRequest are the whole of the Claude probe: print
// mode, local settings only, no session kept, one control request on stdin.
// get_usage asks Claude Code for the account's windows; it runs no turn.
var claudeUsageArgs = []string{"-p", "--verbose", "--setting-sources", "local", "--input-format", "stream-json", "--output-format", "stream-json", "--no-session-persistence"}

const (
	claudeUsageID      = "usage"
	claudeUsageRequest = `{"type":"control_request","request_id":"usage","request":{"subtype":"get_usage","skip_behaviors":true}}`
)

// claudeAnswer is what one control_response of the probe came to.
type claudeAnswer struct {
	reading directReading
	reason  string // set when there is no reading
}

// claudeUsage asks Claude Code, run for the config folder dir ("" for ~/.claude),
// for the windows of the account signed in there. The provider reads and
// refreshes its own sign-in; this process sees none of it. A reading, or a fixed
// reason line, comes back; ctx bounds the whole probe.
func claudeUsage(ctx context.Context, env Env, dir string) (directReading, string) {
	cwd, err := scratchDir()
	if err != nil {
		return directReading{}, reasonCannotStart
	}
	defer os.RemoveAll(cwd)

	got := make(chan claudeAnswer, 1)
	// An API key in the environment would make Claude Code bill the key and
	// report no subscription windows, whatever login the folder holds.
	child, err := startProbe(ctx, env, Claude, claudeUsageArgs, dirVars[Claude], dir, cwd,
		[]string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"},
		func(line []byte) {
			if a, ok := parseClaudeLine(line); ok {
				select {
				case got <- a:
				default:
				}
			}
		})
	if err != nil {
		return directReading{}, reasonCannotStart
	}
	defer child.stop(exitGrace)

	if err := child.send(claudeUsageRequest); err != nil {
		return directReading{}, reasonClaudeVersion
	}
	child.closeStdin()
	select {
	case a := <-got:
		return a.reading, a.reason
	case <-child.done:
		child.exited = true
		// The output is read to its end before the command counts as over, so an
		// answer there is in got by now.
		select {
		case a := <-got:
			return a.reading, a.reason
		default:
		}
		return directReading{}, reasonClaudeVersion
	case <-ctx.Done():
		return directReading{}, reasonTimedOut
	}
}

// parseClaudeLine reads one line of Claude Code's stream. It answers ok only for
// the control_response of the request this probe sent; every other line (init,
// other requests, noise) is not its business.
func parseClaudeLine(line []byte) (claudeAnswer, bool) {
	if !bytes.Contains(line, []byte(`"control_response"`)) {
		return claudeAnswer{}, false
	}
	var msg struct {
		Type     string `json:"type"`
		Response struct {
			Subtype   string          `json:"subtype"`
			RequestID string          `json:"request_id"`
			Response  json.RawMessage `json:"response"`
		} `json:"response"`
	}
	if json.Unmarshal(line, &msg) != nil || msg.Type != "control_response" || msg.Response.RequestID != claudeUsageID {
		return claudeAnswer{}, false
	}
	if msg.Response.Subtype != "success" {
		return claudeAnswer{reason: reasonClaudeError}, true
	}
	return parseClaudeUsage(msg.Response.Response), true
}

// parseClaudeUsage reads the get_usage reply. Unknown keys are ignored and a
// missing or oddly typed one costs only the window it belongs to.
func parseClaudeUsage(raw json.RawMessage) claudeAnswer {
	var body struct {
		Available  *bool `json:"rate_limits_available"`
		RateLimits *struct {
			FiveHour json.RawMessage `json:"five_hour"`
			SevenDay json.RawMessage `json:"seven_day"`
		} `json:"rate_limits"`
	}
	if json.Unmarshal(raw, &body) != nil {
		return claudeAnswer{reason: reasonClaudeVersion}
	}
	if body.Available != nil && !*body.Available {
		return claudeAnswer{reason: reasonSignedOut}
	}
	if body.RateLimits == nil {
		return claudeAnswer{reason: reasonClaudeVersion}
	}
	r := directReading{five: parseClaudeWindow(body.RateLimits.FiveHour), seven: parseClaudeWindow(body.RateLimits.SevenDay)}
	if !r.ok() {
		return claudeAnswer{reason: reasonClaudeVersion}
	}
	return claudeAnswer{reading: r}
}

// parseClaudeWindow reads one window: utilization is 0-100 and resets_at an ISO
// time, kept as Unix seconds (0 when absent or unreadable). A window with no
// number is not a window.
func parseClaudeWindow(raw json.RawMessage) *Reading {
	var w struct {
		Utilization *float64        `json:"utilization"`
		ResetsAt    json.RawMessage `json:"resets_at"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &w) != nil || w.Utilization == nil {
		return nil
	}
	used := *w.Utilization
	if used < 0 {
		used = 0
	}
	if used > 100 {
		used = 100
	}
	r := &Reading{Used: used}
	var at string
	if json.Unmarshal(w.ResetsAt, &at) == nil && at != "" {
		if t, err := time.Parse(time.RFC3339, at); err == nil {
			r.ResetsAt = t.Unix()
		}
	}
	return r
}
