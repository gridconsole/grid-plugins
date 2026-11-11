package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// jobWithFeed makes the fixture a Job's session on a task that has a Feed and
// no result yet, and returns the path of the session's activity file.
func (f *eventFixture) jobWithFeed(t *testing.T) string {
	t.Helper()
	f.workerSession(t)
	f.env["GRID_TASK"] = f.dir
	if err := os.MkdirAll(filepath.Join(f.dir, "feed"), 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(f.dir, "sessions", ".hooks", "session-demo.json")
}

// The raw end reason must survive in the session activity, so a normal end and
// an unknown one are told apart.
func TestWorkerExitKeepsTheRawReasonInTheActivityFile(t *testing.T) {
	f := eventSetup(t)
	path := f.jobWithFeed(t)
	if _, log, code := f.run(t, "SessionEnd", hookio.Input{Reason: "prompt_input_exit"}); code != 0 || log != "" {
		t.Fatalf("end: %d %s", code, log)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "prompt_input_exit") {
		t.Fatalf("activity file has no prompt_input_exit: %s", raw)
	}
}
