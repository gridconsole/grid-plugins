package where

import (
	"errors"
	"strings"
	"testing"
)

// A guest's answer for a task path outside the Grid home is the same sentinel as
// for an unknown task, so callers that classify it (a JSON code) treat both alike.
func TestJdHnGuestOutsideIsTaskNotFound(t *testing.T) {
	f := newFixture(t)
	f.task("alpha", "T-01-invited")
	hidden := f.task("beta", "T-02-hidden")
	guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"gus","perm":"Client"}]}`)
	outside := f.dir("elsewhere", "tasks", "T-01-x")
	q := Query{Cwd: f.tmp, Task: outside}

	f.r.Caller = Caller{Role: "guest", Member: "gus"}
	if _, unknown := f.r.Resolve(Query{Cwd: f.tmp, Task: hidden}); !errors.Is(unknown, ErrTaskNotFound) {
		t.Fatalf("unknown task answer = %v, want ErrTaskNotFound", unknown)
	}
	_, err := f.r.Resolve(q)
	if !errors.Is(err, ErrTaskNotFound) {
		t.Errorf("guest outside path: errors.Is(%v, ErrTaskNotFound) = false, want true", err)
	}
	if err == nil || err.Error() != "task not found" {
		t.Errorf("guest outside path: got %v, want task not found", err)
	}

	f.r.Caller = Caller{Role: "owner", Member: "gus"}
	if _, err := f.r.Resolve(q); err == nil || !strings.Contains(err.Error(), "outside the Grid home "+f.gridHome) {
		t.Errorf("owner outside path: got %v, want the outside-the-Grid-home message", err)
	}
}
