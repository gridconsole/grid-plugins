package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/approvefix"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/hookio"
)

const peMsgMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

// peMsgPost posts a Feed record of type post as a person (or the writer given).
func peMsgPost(t *testing.T, j *jobFixture, w feed.Writer, body map[string]any) feed.Record {
	t.Helper()
	r, err := feed.Store{TaskDir: j.dir, TaskID: j.task}.Post(feed.Input{Type: "post", Body: body}, w)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func peMsgPerson(id, role string) feed.Writer {
	return feed.Writer{Author: feed.Author{ID: id, Role: role}, Worker: id + "@" + peMsgMachine}
}

func peMsgBody(text string) map[string]any {
	return map[string]any{"kind": "message", "ref": "task", "text": text}
}

// peMsgCall runs one PostToolUse of the Job's agent and returns what the hook
// wrote to stdout. It fails the test on a non-zero exit or on any log line.
func peMsgCall(t *testing.T, j *jobFixture) string {
	t.Helper()
	out, log, code := j.run(t, "PostToolUse", hookio.Input{ToolName: "Read", Cwd: j.repo})
	if code != 0 || log != "" {
		t.Fatalf("PostToolUse: exit %d, log %q", code, log)
	}
	return out
}

func peMsgContext(t *testing.T, out string) string {
	t.Helper()
	var got struct {
		Output struct {
			Event   string `json:"hookEventName"`
			Context string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("stdout is not hook output: %v: %q", err, out)
	}
	if got.Output.Event != "PostToolUse" {
		t.Fatalf("hookEventName = %q, want PostToolUse", got.Output.Event)
	}
	return got.Output.Context
}

func TestPeMsgDeliversAPersonsMessage(t *testing.T) {
	j := jobSetup(t)
	peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-1: use the staging bucket"))
	got := peMsgContext(t, peMsgCall(t, j))
	if want := "Message from owner: To J-1: use the staging bucket"; got != want {
		t.Fatalf("additionalContext = %q, want %q", got, want)
	}
}

func TestPeMsgDeliversOnce(t *testing.T) {
	j := jobSetup(t)
	peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-1: first"))
	if got := peMsgContext(t, peMsgCall(t, j)); got != "Message from owner: To J-1: first" {
		t.Fatalf("first call = %q", got)
	}
	if out := peMsgCall(t, j); out != "" {
		t.Fatalf("second call in the same session wrote %q, want nothing", out)
	}
	peMsgPost(t, j, peMsgPerson("dana", "member"), peMsgBody("To J-1: second"))
	peMsgPost(t, j, peMsgPerson("dana", "member"), peMsgBody("To J-1: third"))
	got := peMsgContext(t, peMsgCall(t, j))
	if want := "Message from dana: To J-1: second\nMessage from dana: To J-1: third"; got != want {
		t.Fatalf("new messages = %q, want %q (oldest first, first not repeated)", got, want)
	}
	if out := peMsgCall(t, j); out != "" {
		t.Fatalf("call after the new messages wrote %q, want nothing", out)
	}
}

func TestPeMsgByJobFieldAndSession(t *testing.T) {
	j := jobSetup(t)
	_, err := feed.Store{TaskDir: j.dir, TaskID: j.task}.Post(feed.Input{Type: "post", Job: "J-1", Body: peMsgBody("no prefix here")}, peMsgPerson("owner", "owner"))
	if err != nil {
		t.Fatal(err)
	}
	if got := peMsgContext(t, peMsgCall(t, j)); got != "Message from owner: no prefix here" {
		t.Fatalf("job field message = %q", got)
	}
	// A different session of the same Job is told the same record again; the
	// same session id (a resumed session) is not.
	out, log, code := j.run(t, "PostToolUse", hookio.Input{ToolName: "Read", Cwd: j.repo, SessionID: "session-other"})
	if code != 0 || log != "" || peMsgContext(t, out) != "Message from owner: no prefix here" {
		t.Fatalf("other session: exit %d, log %q, out %q", code, log, out)
	}
}

func TestPeMsgNotDelivered(t *testing.T) {
	cases := []struct {
		name string
		job  string // GRID_JOB; "-" leaves it unset
		post func(t *testing.T, j *jobFixture)
		want string // "" means empty stdout
	}{
		{"message to another Job", "", func(t *testing.T, j *jobFixture) {
			peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-2: use the staging bucket"))
		}, ""},
		{"message to J-10 is not for J-1", "", func(t *testing.T, j *jobFixture) {
			peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-10: use the staging bucket"))
		}, ""},
		{"post of kind close", "", func(t *testing.T, j *jobFixture) {
			peMsgPost(t, j, peMsgPerson("owner", "owner"), map[string]any{"kind": "close", "ref": "task", "text": "To J-1: done"})
		}, ""},
		{"note under the task lease", "", func(t *testing.T, j *jobFixture) {
			w, _ := approvefix.Lease(t, j.dir)
			peMsgPost(t, j, w, peMsgBody("To J-1: tick note"))
		}, ""},
		{"GRID_JOB unset", "-", func(t *testing.T, j *jobFixture) {
			peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-1: use the staging bucket"))
		}, ""},
		{"guest message to J-1", "", func(t *testing.T, j *jobFixture) {
			peMsgPost(t, j, peMsgPerson("gina", "guest"), peMsgBody("To J-1: please check the footer"))
		}, "Message from gina: To J-1: please check the footer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j := jobSetup(t)
			if tc.job == "-" {
				delete(j.env, "GRID_JOB")
			}
			tc.post(t, j)
			out := peMsgCall(t, j)
			if tc.want == "" {
				if out != "" {
					t.Fatalf("stdout = %q, want empty", out)
				}
				return
			}
			if got := peMsgContext(t, out); got != tc.want {
				t.Fatalf("additionalContext = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPeMsgNeverBlocksOnAnUnreadableFeed(t *testing.T) {
	j := jobSetup(t)
	peMsgPost(t, j, peMsgPerson("owner", "owner"), peMsgBody("To J-1: use the staging bucket"))
	feedDir := filepath.Join(j.dir, "feed")
	if err := os.RemoveAll(feedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(feedDir, []byte("not a folder\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, log, code := j.run(t, "PostToolUse", hookio.Input{ToolName: "Read", Cwd: j.repo})
	if code != 0 || out != "" {
		t.Fatalf("exit %d, stdout %q; want exit 0 and empty stdout", code, out)
	}
	if !strings.Contains(log, "grid hook: job message:") {
		t.Fatalf("hook log = %q, want the job message error", log)
	}
}
