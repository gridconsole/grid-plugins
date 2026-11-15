package hook

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/session"
)

// With the session lock held by another session, a PreToolUse does not wait for
// it: the history record is skipped at once with one log line, nothing is
// written, and the tool call's decision is the one a free lock gives.
func TestKr2PreToolUseHistoryDoesNotWaitForSessionLock(t *testing.T) {
	f := eventSetup(t)
	if err := kr2Store(f).RecordUsage(kr2Session, "kr2-usage", session.Usage{Input: 7}); err != nil {
		t.Fatal(err)
	}
	activityBefore := kr2Read(t, kr2Activity(f))
	namesBefore := kr2Names(t, filepath.Dir(kr2Activity(f)))
	historyBefore := historyOf(t, f)

	release := kr2Hold(t, f.dir)
	r := kr2Call(t, kr2Bound(t, 5*time.Second), f, release, "PreToolUse", kr2Bash(kr2Tool))
	if r.elapsed >= time.Second {
		t.Errorf("PreToolUse took %s with the lock held, want under 1 s", r.elapsed)
	}
	if n := strings.Count(r.log, "history skipped: session lock busy\n"); n != 1 {
		t.Errorf("%d skip lines in the log, want 1:\n%s", n, r.log)
	}
	if strings.Contains(r.log, "slow:") {
		t.Errorf("a slow line was logged:\n%s", r.log)
	}
	if r.code != 0 {
		t.Errorf("exit code %d, want 0", r.code)
	}
	if got := kr2Read(t, kr2Activity(f)); !bytes.Equal(got, activityBefore) {
		t.Errorf("activity file changed:\nbefore %s\nafter  %s", activityBefore, got)
	}
	if got := kr2Names(t, filepath.Dir(kr2Activity(f))); got != namesBefore {
		t.Errorf("activity folder changed: %q, was %q", got, namesBefore)
	}
	if got := historyOf(t, f); got != historyBefore {
		t.Errorf("history.md changed: %q", got)
	}

	release()
	free := kr2Call(t, kr2Bound(t, 5*time.Second), f, release, "PreToolUse", kr2Bash("kr2-tool-free"))
	if free.out != r.out {
		t.Errorf("decision differs: %q with the lock held, %q with it free", r.out, free.out)
	}
}
