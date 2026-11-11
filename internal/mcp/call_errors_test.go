package mcp

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

type codedError struct{ code int }

func (e codedError) Error() string { return "coded failure" }
func (e codedError) ExitCode() int { return e.code }

// errorPayload decodes the JSON text of an isError result.
func errorPayload(t *testing.T, r Result) map[string]any {
	t.Helper()
	if !r.IsError {
		t.Fatalf("result is not an error: %+v", r)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(r.Content[0].Text), &payload); err != nil {
		t.Fatalf("error text is not JSON: %q: %v", r.Content[0].Text, err)
	}
	return payload
}

func TestToolErrorCode(t *testing.T) {
	coded := errorPayload(t, toolError(codedError{3}))
	if coded["code"] != float64(3) || coded["error"] != "coded failure" {
		t.Fatalf("coded error payload = %v, want code 3 and the error text", coded)
	}
	plain := errorPayload(t, toolError(errors.New("x")))
	if _, has := plain["code"]; has || plain["error"] != "x" {
		t.Fatalf("plain error payload = %v, want error x and no code", plain)
	}
}

func TestRenderRefusesUnmarshalableValue(t *testing.T) {
	if _, err := render(map[string]any{"c": make(chan int)}, nil); err == nil {
		t.Fatal("render of a chan value = nil error, want an error")
	}
}

func TestCallErrors(t *testing.T) {
	const postArgs = `{"type":"post","body":{"ref":"task","kind":"message","text":"must not post"}}`
	noPost := func(t *testing.T, store feed.Store) {
		t.Helper()
		read, err := store.Read(feed.Filter{})
		if err != nil || len(read.Records) != 0 {
			t.Fatalf("a refused call wrote to the Feed: %+v %v", read, err)
		}
	}
	emptyTask := func(s Server) Server {
		s.Resolve = func(task, project string) (feed.Store, error) { return feed.Store{}, nil }
		return s
	}
	tests := []struct {
		name  string
		setup func(Server) Server
		tool  string
		args  string
		want  string
	}{
		{"no resolver", func(Server) Server { return Server{Version: "test"} }, "feed_read", `{}`, "Feed resolver is unavailable"},
		{"feed_read without task id", emptyTask, "feed_read", `{}`, feed.ErrNoTaskID.Error()},
		{"feed_state without task id", emptyTask, "feed_state", `{}`, feed.ErrNoTaskID.Error()},
		{"no session", func(s Server) Server { s.Session = nil; return s }, "feed_post", postArgs, "Feed session is unavailable"},
		// Contrast: already covered, kept to show the two session refusals differ.
		{"session fails", func(s Server) Server {
			s.Session = func(in feed.Input) (feed.Input, feed.Writer, error) {
				return in, feed.Writer{}, errors.New("no session")
			}
			return s
		}, "feed_post", postArgs, "no session"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, store, _ := fixture(t)
			s = tt.setup(s)
			out := result(t, last(t, exchange(t, s, prelude+request(tt.tool, tt.args))))
			payload := errorPayload(t, out)
			if got, _ := payload["error"].(string); !strings.Contains(got, tt.want) {
				t.Fatalf("%s error = %q, want it to hold %q", tt.tool, got, tt.want)
			}
			noPost(t, store)
		})
	}
}

func TestToolsCallWithoutArguments(t *testing.T) {
	s, _, _ := fixture(t)
	call := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"feed_types"}}` + "\n"
	out := result(t, last(t, exchange(t, s, prelude+call)))
	if out.IsError {
		t.Fatalf("feed_types without arguments is an error: %s", out.Content[0].Text)
	}
	if !strings.Contains(out.Content[0].Text, "types") {
		t.Fatalf("feed_types text = %.200s, want it to hold %q", out.Content[0].Text, "types")
	}
}
