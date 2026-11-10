package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/plan"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

func approveWriter(t *testing.T, h *harness) feed.Writer {
	t.Helper()
	w, err := orchestratorWriter(h.env, "task", os.Getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

// acceptedControls counts the controls that accepted job with a plan digest.
func acceptedControls(t *testing.T, store feed.Store, job string) int {
	t.Helper()
	r, err := store.Read(feed.Filter{Type: "control"})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range r.Records {
		if e.Body["target"] == job && e.Body["status"] == "accepted" && e.Body["plan_digest"] != nil {
			n++
		}
	}
	return n
}

// Two presses of the same digest at once post one control: the "already
// accepted with this digest" check runs under .orchestrator.lock, in the same
// pass as the write. Run it with -count=20.
func TestPersonApprovalSimultaneousPressesPostOneControl(t *testing.T) {
	h, _, store, text := approveJobHarness(t)
	w := approveWriter(t, h)
	e := taskstore.Event{PlanHash: plan.Hash(text), PlanDigest: plan.Digest(text)}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			errs <- personApproval(store, w, "J-2", "approved", e)
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if n := acceptedControls(t, store, "J-2"); n != 1 {
		t.Fatalf("two presses posted %d controls, want 1", n)
	}
}

// An accepted person Job of another kind that carries a plan_digest is not the
// approval of the plan: only the person Job of the approve step is.
func TestApprovalFromJobIgnoresAcceptedPersonJobOfAnotherKind(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, "J-3", nil)), &body); err != nil {
		t.Fatal(err)
	}
	body["kind"] = "review"
	out, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--person", "dana", "--job", "J-3", "--body", string(out)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	w := approveWriter(t, h)
	accept := map[string]any{"target": "J-3", "status": "accepted", "reason": "answered by member", "answer": "approved", "plan_hash": plan.Hash(text), "plan_digest": plan.Digest(text)}
	if _, err := orchestrator.Write(store, w, feed.Input{Type: "control", Body: accept}); err != nil {
		t.Fatal(err)
	}
	if st, _ := store.State(); string(st.Jobs["J-3"].Status) != "accepted" {
		t.Fatalf("J-3 is %s", st.Jobs["J-3"].Status)
	}
	if a, err := taskstore.ApprovalFromJob(path); err == nil {
		t.Fatalf("ApprovalFromJob counted the accepted review Job: %+v", a)
	}
	if taskstore.Approved(path) {
		t.Fatal("Approved counted the accepted review Job")
	}
}

// ApprovalFromJob reads the Feed and plan.md and writes nothing: not state.json
// (a fold that saves state would replace the sentinel or create the file) and not
// the reader cache.
func TestApprovalFromJobWritesNoState(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	if code := h.run(path, "approve", path, "--plan-hash", plan.Hash(text)); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
	statePath := filepath.Join(store.Dir(), "state.json")
	cachePath := filepath.Join(store.Dir(), ".cache", "reader.json")
	sentinel := []byte("{\"sentinel\":true}\n")
	if err := os.WriteFile(statePath, sentinel, 0o600); err != nil {
		t.Fatal(err)
	}
	cacheBefore, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := taskstore.ApprovalFromJob(path); err != nil {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(statePath); err != nil || !bytes.Equal(got, sentinel) {
		t.Fatalf("state.json after ApprovalFromJob: %q %v", got, err)
	}
	if got, _ := os.ReadFile(cachePath); !bytes.Equal(got, cacheBefore) {
		t.Fatal("ApprovalFromJob rewrote the reader cache")
	}
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	if _, err := taskstore.ApprovalFromJob(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(statePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("ApprovalFromJob wrote state.json: %v", err)
	}
}

// runPersonApproval gives the --plan-hash path the checks AnswerPerson makes: an
// answer over the cap, and a Job that is not a Job for a person, are refused
// before approval.json or any control is written.
func TestRunPersonApprovalAnswerLengthAndPersonJob(t *testing.T) {
	h, path, store, text := approveJobHarness(t)
	w := approveWriter(t, h)
	h.env.Cwd = path
	long := strings.Repeat("x", 3501)
	for _, c := range []struct{ name, job, answer, want string }{
		{"answer over the cap", "J-2", long, "longer than 3500 characters"},
		{"worker Job", "J-1", "yes", "not a Job for a person"},
		{"unknown Job", "J-9", "yes", "not a Job for a person"},
	} {
		t.Run(c.name, func(t *testing.T) {
			h.errb.Reset()
			code := runPersonApproval(h.env, store, w, path, c.job, map[string]any{"answer": c.answer}, plan.Hash(text), false)
			if code == 0 || !strings.Contains(h.errb.String(), c.want) {
				t.Fatalf("%d %s", code, h.errb.String())
			}
			if _, err := os.Stat(filepath.Join(path, "approval.json")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("wrote approval.json: %v", err)
			}
			if n := acceptedControls(t, store, c.job); n != 0 {
				t.Fatalf("posted %d accepting controls for %s", n, c.job)
			}
		})
	}
	// An answer at the cap is still taken.
	if code := runPersonApproval(h.env, store, w, path, "J-2", map[string]any{"answer": strings.Repeat("x", 3500)}, plan.Hash(text), false); code != 0 {
		t.Fatalf("%d %s", code, h.errb.String())
	}
}
