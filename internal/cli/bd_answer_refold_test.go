package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// bdPublishedStatus reads a Job's status from feed/state.json as the app does:
// the file on disk, not a fold.
func bdPublishedStatus(t *testing.T, store feed.Store, job string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(store.Dir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var st struct {
		Jobs map[string]struct {
			Status string `json:"status"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatalf("state.json does not decode: %v\n%s", err, data)
	}
	return st.Jobs[job].Status
}

// bdPublish runs one `job next` so state.json is published; a refold only
// rewrites state.json when it is already there.
func bdPublish(t *testing.T, h *harness, path string, store feed.Store) {
	t.Helper()
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("next %d: %s", code, h.errb)
	}
	if got := bdPublishedStatus(t, store, "J-2"); got != "posted" {
		t.Fatalf("before the write, state.json shows J-2 as %q, want posted", got)
	}
	h.out.Reset()
	h.errb.Reset()
}

// bdPersonJob is a waiting task with a published state.json and an open person
// Job J-2 behind a leased writer.
func bdPersonJob(t *testing.T) (*harness, string, feed.Store) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("post %d: %s", code, h.errb)
	}
	bdPublish(t, h, path, store)
	return h, path, store
}

// bdPlanJob is approveJobHarness with a published state.json.
func bdPlanJob(t *testing.T) (*harness, string, feed.Store, string) {
	t.Helper()
	h, path, store, text := approveJobHarness(t)
	bdPublish(t, h, path, store)
	return h, path, store, text
}

// bdRequireRefolded says the published files are what a fold says: J-2 is
// accepted in state.json on disk, and needs-you.json equals the fold's NeedsYou.
func bdRequireRefolded(t *testing.T, h *harness, store feed.Store) {
	t.Helper()
	fold, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if want := string(fold.Jobs["J-2"].Status); want != "accepted" {
		t.Fatalf("the fold has J-2 as %q, want accepted", want)
	}
	if got, want := bdPublishedStatus(t, store, "J-2"), string(fold.Jobs["J-2"].Status); got != want {
		t.Fatalf("state.json shows J-2 as %q, the fold has %q", got, want)
	}
	if got, want := strings.Join(xhNeedsYou(t, store), ","), strings.Join(fold.NeedsYou, ","); got != want {
		t.Fatalf("needs-you.json %q, fold %q", got, want)
	}
	if h.errb.Len() != 0 {
		t.Fatalf("a good write and refold print nothing on stderr: %q", h.errb)
	}
}

// Row 1: a leased answer to a person Job shows accepted in state.json at once.
func TestBdLeasedAnswerRefolds(t *testing.T) {
	h, path, store := bdPersonJob(t)
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Yes"}`); code != 0 {
		t.Fatalf("job result %d: %s", code, h.errb)
	}
	bdRequireRefolded(t, h, store)
}

// Row 2: a leased job result --plan-hash on the approve Job refolds too.
func TestBdPlanHashResultRefolds(t *testing.T) {
	h, path, store, text := bdPlanJob(t)
	if code := h.run(path, "job", "result", "J-2", "--plan-hash", plan.Digest(text), "--body", `{"answer":"Go ahead."}`); code != 0 {
		t.Fatalf("job result %d: %s", code, h.errb)
	}
	bdRequireRefolded(t, h, store)
}

// Row 3: grid approve, the verb the app's Approve button runs, refolds too.
func TestBdApproveRefolds(t *testing.T) {
	h, path, store, text := bdPlanJob(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Digest(text)); code != 0 {
		t.Fatalf("approve %d: %s", code, h.errb)
	}
	bdRequireRefolded(t, h, store)
}

// Row 4: the answer is written when the refold fails. The exit code and stdout
// are the usual ones, and stderr says the refold failed.
func TestBdRefoldErrorKeepsTheAnswer(t *testing.T) {
	h, path, store := bdPersonJob(t)
	if err := os.RemoveAll(filepath.Join(store.Dir(), "needs-you.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(store.Dir(), "needs-you.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"Yes"}`); code != 0 {
		t.Fatalf("job result exit %d, want 0: %s", code, h.errb)
	}
	if out := strings.TrimSpace(h.out.String()); out == "" || strings.Contains(out, "\n") {
		t.Fatalf("stdout is not the one record id line: %q", h.out)
	}
	found := false
	for _, line := range strings.Split(h.errb.String(), "\n") {
		if strings.HasPrefix(line, "refold: ") {
			found = true
		}
	}
	if !found {
		t.Fatalf("stderr has no line starting %q: %q", "refold: ", h.errb)
	}
	st, err := store.State()
	if err != nil || string(st.Jobs["J-2"].Status) != "accepted" {
		t.Fatalf("the answer is not in the Feed: %v %v", st.Jobs["J-2"], err)
	}
}
