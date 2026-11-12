package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// vcRepoPost posts one Job into a task as its owner, as ownerPost does, naming repo ("" names none).
func vcRepoPost(t *testing.T, dir, taskID, id, repo string) {
	t.Helper()
	store := feed.Store{TaskDir: dir, TaskID: taskID}
	ts := lease.TaskStore{Feed: store}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + feedMachine, Lease: "task"}
	g, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		t.Fatal(err)
	}
	w.Fence = g.Fence
	if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		t.Fatal(err)
	}
	body := workerNextBody(t, id)
	if repo != "" {
		body["repo"] = repo
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: id, Body: body}, w); err != nil {
		t.Fatal(err)
	}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
}

// vcRepoSpawnRepo is the key the app worker decodes from a spawn line
// (kernel/worker/job.go: Repo, json "repo").
func vcRepoSpawnRepo(t *testing.T, raw string) string {
	t.Helper()
	var r struct {
		Spawn struct {
			Repo string `json:"repo"`
		} `json:"spawn"`
	}
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	return r.Spawn.Repo
}

// Row 3: in a project folder that holds two repos and is not one itself, the spawn
// line carries the folder of the repo the Job names, or of the first repo when it
// names none; a one-repo project's spawn carries that repo's folder.
func TestVcRepoWorkerNextSpawnCarriesTheRepoFolder(t *testing.T) {
	h, first := feedHarness(t)
	multi := t.TempDir() // holds app and plugin; not a git repo itself
	h.record(project.Record{Name: "example", Folders: []project.Folder{{Machine: feedMachine, Path: multi}},
		Repos: []project.Repo{{Path: "app"}, {Path: "plugin"}}})
	second := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-08-second")
	h.write(filepath.Join(second, "task.md"), "---\nid: "+secondTaskID+"\n---\n")
	solo := t.TempDir()
	h.record(project.Record{Name: "solo", ID: regProjectID, Folders: []project.Folder{{Machine: feedMachine, Path: solo}},
		Repos: []project.Repo{{Path: "."}}})
	third := filepath.Join(h.gridHome, "projects", "solo", "tasks", "T-09-solo")
	h.write(filepath.Join(third, "task.md"), "---\nid: "+thirdTaskID+"\n---\n")
	vcRepoPost(t, first, feedTaskID, "J-1", "plugin")
	vcRepoPost(t, second, secondTaskID, "J-1", "")
	vcRepoPost(t, third, thirdTaskID, "J-1", "")
	got, stderr := workerNextSpawnLines(t, h, 3)
	for name, c := range map[string]struct{ task, want string }{
		"a Job naming plugin":                    {feedTaskID, filepath.Join(multi, "plugin")},
		"a Job naming none takes the first repo": {secondTaskID, filepath.Join(multi, "app")},
		"the only repo of a one-repo project":    {thirdTaskID, solo},
	} {
		if r := vcRepoSpawnRepo(t, got[c.task]); r != c.want {
			t.Errorf("%s: spawn repo = %q, want %q\nline: %s\nstderr: %s", name, r, c.want, got[c.task], stderr)
		}
	}
}
