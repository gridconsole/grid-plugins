package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// task new through Run with no OwnerPID, as the app's libCall builds the Call:
// the host is this long-lived test process, and after the call neither the task
// lease nor a J-1 lease names it, so a worker can take J-1.
func TestF4dTaskNewNoOwnerLeavesLeaseFree(t *testing.T) {
	h := ihClHome(t, "No owner on the Call")
	code, out, errs := ihClRun(h, h.folder, Call{Args: []string{"task", "new", "In process, no owner", "--project", "example", "--json"}})
	var made struct{ Path string }
	if code != 0 || json.Unmarshal([]byte(out), &made) != nil || made.Path == "" {
		t.Fatalf("task new: %d %s %s", code, out, errs)
	}
	code, out, errs = ihClRun(h, made.Path, Call{Args: []string{"lease", "status", "task", "--json"}})
	var st struct {
		Holder *string `json:"holder"`
		PID    *int    `json:"pid"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &st) != nil || st.Holder != nil || st.PID != nil {
		t.Fatalf("task lease after an in-process task new (host pid %d): %d %s %s", os.Getpid(), code, out, errs)
	}
	if _, err := os.Stat(filepath.Join(made.Path, "feed", "lease", "J-1.json")); !os.IsNotExist(err) {
		t.Fatalf("task new left a J-1 lease file: %v", err)
	}
}
