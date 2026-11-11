package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	taskstore "github.com/gridconsole/grid-plugins/internal/task"
)

// The re-plan Job the loop posts after a cancel must start: it passes the lint
// options render uses and renders, although the Jobs it names (J-2, J-4, J-6)
// never had a folder under jobs/. A job: pointer to them would be a job.pointer
// error and render would refuse the Job, as it refused J-2.
func TestGkReplanBodyPassesRenderLint(t *testing.T) {
	h := newTaskHarness(t)
	h.env.Getenv = func(k string) string {
		if k == "GRID_WHO" {
			return "alex"
		}
		return ""
	}
	path := newTask(t, h)
	for _, name := range []string{"J-2", "J-4", "J-6"} {
		if _, err := os.Stat(filepath.Join(path, "jobs", name)); err == nil {
			t.Fatalf("jobs/%s exists: the test needs a Job that never rendered", name)
		}
	}
	body := orchestrator.ReplanJobBody("J-7", "J-2", "", []string{"J-4", "J-6"})
	findings, err := postFindings(h.env, path, body)
	if err != nil {
		t.Fatalf("the lint options could not be built: %v", err)
	}
	if lint.HasErrors(findings) {
		t.Fatalf("render would refuse the re-plan Job: %+v", findings)
	}
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", data); err != nil {
		t.Fatalf("the re-plan body is not a job-record: %v", err)
	}

	// Post it on the task's Feed as the task lease holder, then render it.
	id, owner, err := taskstore.FeedIdentity(path)
	if err != nil {
		t.Fatal(err)
	}
	store := feed.Store{TaskDir: path, TaskID: id}
	w := feed.Writer{Author: feed.Author{ID: owner, Role: "member"}, Worker: owner + "@" + feedMachine, Lease: "task"}
	ts := lease.TaskStore{Feed: store}
	grant, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Gen, w.Fence = grant.Gen, grant.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-7", Body: body}, w); err != nil {
		t.Fatalf("the Feed refused the re-plan Job: %v", err)
	}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
	h.out.Reset()
	h.errb.Reset()
	if code := h.run(path, "job", "render", path, "J-7", "--json"); code != 0 {
		t.Fatalf("job render exit %d: %s %s", code, h.out, h.errb)
	}
	if _, err := os.Stat(filepath.Join(path, "jobs", "J-7", "brief.md")); err != nil {
		t.Fatalf("job render saved no brief: %v (render printed %s)", err, h.out)
	}
}
