package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// wlFirstJobIDs is the Job ids of the job records on the Feed of the task at path.
func wlFirstJobIDs(t *testing.T, path string) []string {
	t.Helper()
	var ids []string
	for _, r := range gqWorkflowJobs(t, path) {
		if r.Job == nil {
			t.Fatalf("job record %s has no Job id", r.ID)
		}
		ids = append(ids, *r.Job)
	}
	return ids
}

// wlFirstJobOnlyJ1 fails unless the Feed of the task at path holds exactly one job record, J-1.
func wlFirstJobOnlyJ1(t *testing.T, path string) {
	t.Helper()
	if ids := wlFirstJobIDs(t, path); len(ids) != 1 || ids[0] != "J-1" {
		t.Fatalf("the Feed holds job records %v, want exactly one J-1", ids)
	}
}

// wlFirstJobBare makes a task the way grid task new does before it posts J-1: the task exists and its Feed has no Job.
func wlFirstJobBare(t *testing.T, h *harness) (task.Task, task.NewOptions) {
	t.Helper()
	o := task.NewOptions{Cwd: h.home, Words: "Add a search box to the site", Project: "p", Size: "auto", Getenv: h.env.Getenv}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	made, err := (task.Store{Resolver: resolver(h.env)}).New(o)
	if err != nil {
		t.Fatal(err)
	}
	if ids := wlFirstJobIDs(t, made.Path); len(ids) != 0 {
		t.Fatalf("a task before its first post holds job records %v", ids)
	}
	return made, o
}

// A second postFirstJob on a task that already has J-1 reports nothing wrong and posts nothing.
func TestWlFirstRaceJobAlreadyPosted(t *testing.T) {
	h := newTaskHarness(t)
	path := newTask(t, h)
	// The folder is T-NN-slug; Show takes the T-NN part.
	made, err := (task.Store{Resolver: resolver(h.env)}).Show(h.home, strings.Join(strings.SplitN(filepath.Base(path), "-", 3)[:2], "-"))
	if err != nil {
		t.Fatal(err)
	}
	wlFirstJobOnlyJ1(t, path)
	if err := postFirstJob(h.env, made, task.NewOptions{}); err != nil {
		t.Fatalf("a second postFirstJob on a task with J-1 returned: %v", err)
	}
	wlFirstJobOnlyJ1(t, path)
}

// A worker pass that lands between task creation and postFirstJob posts J-1 and holds the task lease;
// postFirstJob then meets the held lease, finds J-1 on the Feed and succeeds without a second J-1.
func TestWlFirstRacePassPostedFirst(t *testing.T) {
	h := newTaskHarness(t)
	made, o := wlFirstJobBare(t, h)
	var out bytes.Buffer
	s := &orchestrator.Stream{
		Member: "member", Machine: h.env.Machine, Role: "member",
		PID: os.Getpid(), PIDStart: contract.FormatTime(time.Now()),
		Out: &out, Diag: h.errb,
		Tasks:    func() ([]string, error) { return workerTaskDirs(h.env) },
		FirstJob: firstJobOf,
	}
	s.Configure = func(st *orchestrator.Store, taskID string) {
		configureTickStore(h.env, st, filepath.Base(filepath.Dir(filepath.Dir(st.Feed.TaskDir))), taskID)
		st.Diagnostics = h.errb
	}
	if err := s.Pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	wlFirstJobOnlyJ1(t, made.Path)
	if err := postFirstJob(h.env, made, o); err != nil {
		t.Fatalf("postFirstJob after a pass posted J-1 returned: %v", err)
	}
	wlFirstJobOnlyJ1(t, made.Path)
	id, err := feed.TaskID(made.Path)
	if err != nil {
		t.Fatal(err)
	}
	if !wlFirstJobPosted(feed.Store{TaskDir: made.Path, TaskID: id}) {
		t.Fatal("wlFirstJobPosted does not see J-1")
	}
}

// A lease held with no J-1 on the Feed is still an error: nothing was posted, so the task has no first Job.
func TestWlFirstHeldWithoutJobStillFails(t *testing.T) {
	h := newTaskHarness(t)
	made, o := wlFirstJobBare(t, h)
	id, err := feed.TaskID(made.Path)
	if err != nil {
		t.Fatal(err)
	}
	store := feed.Store{TaskDir: made.Path, TaskID: id}
	if wlFirstJobPosted(store) {
		t.Fatal("wlFirstJobPosted sees a J-1 on a Feed with no Job")
	}
	other := feed.Writer{Author: feed.Author{ID: "other", Role: "member"}, Worker: "other@" + h.env.Machine, Lease: "task"}
	if _, err := (lease.TaskStore{Feed: store}).TakeTask(other, os.Getpid(), contract.FormatTime(time.Now())); err != nil {
		t.Fatal(err)
	}
	err = postFirstJob(h.env, made, o)
	if !errors.Is(err, lease.ErrTaskHeld) {
		t.Fatalf("postFirstJob with the lease held and no J-1 returned %v, want the lease refusal", err)
	}
	if ids := wlFirstJobIDs(t, made.Path); len(ids) != 0 {
		t.Fatalf("the Feed holds job records %v, want none", ids)
	}
}
