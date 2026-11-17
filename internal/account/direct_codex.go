package account

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"time"
)

// The Codex probe: `codex app-server` speaks JSON-RPC over stdio. Grid sends
// initialize, waits for its reply, sends the initialized notification and then
// account/rateLimits/read, and keeps stdin open until the reply comes or the
// deadline kills the child. Codex reads and refreshes its own sign-in.
const (
	codexInitialize = `{"id":1,"method":"initialize","params":{"clientInfo":{"name":"grid","title":"Grid","version":"1"}}}`
	codexInitialed  = `{"method":"initialized"}`
	codexReadLimits = `{"id":2,"method":"account/rateLimits/read"}`

	codexWindow5h = 300   // windowDurationMins of the 5 hour window
	codexWindow7d = 10080 // windowDurationMins of the 7 day window
)

// codexReply is one JSON-RPC response of the app-server.
type codexReply struct {
	id     int64
	result json.RawMessage
	failed bool // the reply carries an error object
}

// how one attempt ended when it gave no reading
type codexEnd int

const (
	codexRead    codexEnd = iota // a reply to account/rateLimits/read
	codexErrored                 // an error reply: worth one more try
	codexExited                  // the app-server left without answering
	codexTimeout                 // the deadline passed
	codexNoStart                 // the app-server could not be started
)

// codexUsage reads the windows of the Codex account in CODEX_HOME dir ("" for
// ~/.codex). An error reply is tried once more after a short pause, on a fresh
// app-server; everything is within ctx.
func codexUsage(ctx context.Context, env Env, dir string) (directReading, string) {
	var (
		reading directReading
		reason  string
		end     codexEnd
	)
	for try := 0; try < 2; try++ {
		if try > 0 {
			select {
			case <-time.After(env.retryWait()):
			case <-ctx.Done():
				return directReading{}, reasonTimedOut
			}
		}
		reading, reason, end = codexAttempt(ctx, env, dir)
		if end != codexErrored {
			break
		}
	}
	return reading, reason
}

func codexAttempt(ctx context.Context, env Env, dir string) (directReading, string, codexEnd) {
	cwd, err := scratchDir()
	if err != nil {
		return directReading{}, reasonCannotStart, codexNoStart
	}
	defer os.RemoveAll(cwd)

	replies := make(chan codexReply, 8)
	child, err := startProbe(ctx, env, Codex, []string{"app-server"}, dirVars[Codex], dir, cwd, nil, func(line []byte) {
		if r, ok := parseCodexLine(line); ok {
			select {
			case replies <- r:
			default: // an app-server that talks without being asked is not waited for
			}
		}
	})
	if err != nil {
		return directReading{}, reasonCannotStart, codexNoStart
	}
	defer child.stop(exitGrace)

	await := func(id int64) (codexReply, codexEnd) {
		for {
			select {
			case r := <-replies:
				if r.id != id {
					continue
				}
				if r.failed {
					return r, codexErrored
				}
				return r, codexRead
			case <-child.done:
				child.exited = true
				select {
				case r := <-replies:
					if r.id == id {
						if r.failed {
							return r, codexErrored
						}
						return r, codexRead
					}
				default:
				}
				return codexReply{}, codexExited
			case <-ctx.Done():
				return codexReply{}, codexTimeout
			}
		}
	}
	fail := func(end codexEnd) (directReading, string, codexEnd) {
		switch end {
		case codexErrored:
			return directReading{}, reasonCodexError, end
		case codexTimeout:
			return directReading{}, reasonTimedOut, end
		}
		return directReading{}, reasonCodexVersion, end
	}

	if child.send(codexInitialize) != nil {
		return fail(codexExited)
	}
	if _, end := await(1); end != codexRead {
		return fail(end)
	}
	if child.send(codexInitialed) != nil || child.send(codexReadLimits) != nil {
		return fail(codexExited)
	}
	r, end := await(2)
	if end != codexRead {
		return fail(end)
	}
	reading := parseCodexLimits(r.result)
	if !reading.ok() {
		return directReading{}, reasonCodexVersion, codexRead
	}
	return reading, "", codexRead
}

// parseCodexLine reads one line of the app-server's output. It answers ok only
// for a response (an id with a result or an error and no method); notifications
// and requests of the server are not replies.
func parseCodexLine(line []byte) (codexReply, bool) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '{' {
		return codexReply{}, false
	}
	var msg struct {
		ID     *int64          `json:"id"`
		Method string          `json:"method"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if json.Unmarshal(line, &msg) != nil || msg.ID == nil || msg.Method != "" {
		return codexReply{}, false
	}
	hasErr := len(msg.Error) > 0 && !bytes.Equal(msg.Error, []byte("null"))
	if !hasErr && len(msg.Result) == 0 {
		return codexReply{}, false
	}
	return codexReply{id: *msg.ID, result: msg.Result, failed: hasErr}, true
}

// parseCodexLimits reads the result of account/rateLimits/read. The windows are
// named by their length, whichever of primary and secondary carries them; a
// window of another length is not shown. Unknown keys are ignored.
func parseCodexLimits(raw json.RawMessage) directReading {
	var res struct {
		RateLimits *codexSnapshot           `json:"rateLimits"`
		ByLimitID  map[string]codexSnapshot `json:"rateLimitsByLimitId"`
	}
	if json.Unmarshal(raw, &res) != nil {
		return directReading{}
	}
	var out directReading
	if res.RateLimits != nil {
		out = res.RateLimits.reading()
	}
	if c, ok := res.ByLimitID["codex"]; !out.ok() && ok {
		out = c.reading()
	}
	return out
}

type codexSnapshot struct {
	Primary   json.RawMessage `json:"primary"`
	Secondary json.RawMessage `json:"secondary"`
}

func (s codexSnapshot) reading() directReading {
	var out directReading
	for _, raw := range []json.RawMessage{s.Primary, s.Secondary} {
		var w struct {
			Used     *float64 `json:"usedPercent"`
			Mins     *float64 `json:"windowDurationMins"`
			ResetsAt *float64 `json:"resetsAt"`
		}
		if len(raw) == 0 || json.Unmarshal(raw, &w) != nil || w.Used == nil || w.Mins == nil {
			continue
		}
		used := *w.Used
		if used < 0 {
			used = 0
		}
		if used > 100 {
			used = 100
		}
		r := &Reading{Used: used}
		if w.ResetsAt != nil && *w.ResetsAt > 0 {
			r.ResetsAt = int64(*w.ResetsAt)
		}
		switch int(*w.Mins) {
		case codexWindow5h:
			out.five = r
		case codexWindow7d:
			out.seven = r
		}
	}
	return out
}
