package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

// waitingApproveHarness is the setup of TestApproveWaitsForTheAfterJob: J-1 posted,
// the approve person Job J-2 after J-1.
func waitingApproveHarness(t *testing.T) (*harness, string, feed.Store, string) {
	t.Helper()
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	text := "# Plan\n\nShip the parser.\n"
	if err := os.WriteFile(filepath.Join(path, "plan.md"), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "intent.md"), []byte("# Intent\n\nShip the parser.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	md := "---\nid: T-07\nulid: " + feedTaskID + "\ntitle: Example\nproject: example\nstate: waiting\ncreated: 2026-11-10T08:00:00Z\nupdated: 2026-11-10T08:00:00Z\n---\n"
	if err := os.WriteFile(filepath.Join(path, "task.md"), []byte(md), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := h.run(path, "job", "post", "--type", "job", "--job", "J-1", "--body", personBody(t, "J-1", nil)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-2", []any{"J-1"})), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = "approve"
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-2", "--body", string(out)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	h.out.Reset()
	h.errb.Reset()
	return h, path, store, text
}

// requireNothingAnswered checks J-1 and J-2 are still posted and no approval.json exists.
func requireNothingAnswered(t *testing.T, path string, store feed.Store) {
	t.Helper()
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"J-1", "J-2"} {
		if s := string(st.Jobs[id].Status); s != "posted" {
			t.Fatalf("%s is %s", id, s)
		}
	}
	if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("approval.json: %v", err)
	}
}

const fu2WaitingBody = `{"error":"job_waiting","code":"job_waiting","job":"J-2","waiting_for":"J-1","reason":"waiting for J-1 to be accepted"}`

func TestFu2ApproveJobWaitingJSON(t *testing.T) {
	h, path, store, text := waitingApproveHarness(t)
	code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--json")
	if code != 3 {
		t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out.String(), h.errb.String())
	}
	var got, want map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("stdout %q: %v", h.out.String(), err)
	}
	if err := json.Unmarshal([]byte(fu2WaitingBody), &want); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("stdout %s, want %s", h.out.String(), fu2WaitingBody)
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("stdout %s, want %s", h.out.String(), fu2WaitingBody)
		}
	}
	if strings.Count(strings.TrimRight(h.out.String(), "\n"), "\n") != 0 {
		t.Fatalf("stdout is not one line: %q", h.out.String())
	}
	if !strings.Contains(h.errb.String(), "J-1") || !strings.Contains(h.errb.String(), "J-2 cannot be answered yet: waiting for J-1 to be accepted") {
		t.Fatalf("stderr %q", h.errb.String())
	}
	requireNothingAnswered(t, path, store)
}

func TestFu2ApproveJobWaitingWithoutJSON(t *testing.T) {
	h, path, store, text := waitingApproveHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 3 {
		t.Fatalf("exit %d %s", code, h.errb.String())
	}
	if h.out.Len() != 0 {
		t.Fatalf("stdout %q, want empty", h.out.String())
	}
	if got := h.errb.String(); got != "grid approve: J-2 cannot be answered yet: waiting for J-1 to be accepted\n" {
		t.Fatalf("stderr %q", got)
	}
	requireNothingAnswered(t, path, store)
}

// A snapshot that cannot be loaded keeps today's output: no job_waiting object.
func TestFu2ApproveLoadFailureWithJSONHasNoJobWaiting(t *testing.T) {
	h, path, _, text := approveJobHarness(t)
	old := loadFeedSnapshot
	t.Cleanup(func() { loadFeedSnapshot = old })
	loadFeedSnapshot = func(feed.Store) (orchestrator.Snapshot, error) {
		return orchestrator.Snapshot{}, errors.New("feed unreadable")
	}
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--json"); code != 3 {
		t.Fatalf("exit %d %s", code, h.errb.String())
	}
	if h.out.Len() != 0 || strings.Contains(h.out.String(), "job_waiting") {
		t.Fatalf("stdout %q", h.out.String())
	}
	if got := h.errb.String(); got != "grid approve: feed unreadable\n" {
		t.Fatalf("stderr %q", got)
	}
}

func TestFu2ApproveJobWaitingRoles(t *testing.T) {
	for _, role := range []string{"owner", "member"} {
		t.Run(role, func(t *testing.T) {
			h, path, store, text := waitingApproveHarness(t)
			t.Setenv("GRID_ROLE", role)
			if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--json"); code != 3 {
				t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out.String(), h.errb.String())
			}
			if strings.TrimRight(h.out.String(), "\n") != fu2WaitingBody {
				t.Fatalf("stdout %q", h.out.String())
			}
			requireNothingAnswered(t, path, store)
		})
	}
	t.Run("agent", func(t *testing.T) {
		h, path, store, text := waitingApproveHarness(t)
		// runApprove has no agent refusal ahead of the Job wait (workerMarked is read
		// only by person check and agentShell; approvalWriter, approve_record.go:20-38,
		// looks at GRID_ROLE and GRID_MEMBER only). So with GRID_AGENT=1 the run is
		// refused by the wait itself, as before this change: exit 3, the stderr line,
		// nothing written. The object is the wait reason only; it opens nothing.
		t.Setenv("GRID_AGENT", "1")
		code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text), "--json")
		if code != 3 {
			t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out.String(), h.errb.String())
		}
		if got := h.errb.String(); got != "grid approve: J-2 cannot be answered yet: waiting for J-1 to be accepted\n" {
			t.Fatalf("stderr %q", got)
		}
		if strings.TrimRight(h.out.String(), "\n") != fu2WaitingBody {
			t.Fatalf("stdout %q", h.out.String())
		}
		requireNothingAnswered(t, path, store)
	})
}
