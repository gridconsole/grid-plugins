package orchestrator

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// vcRepoPost posts one Job into task i as streamHome.post does, naming repo ("" names none).
func vcRepoPost(h *streamHome, i int, id, repo string) {
	h.t.Helper()
	st := h.store(i)
	ts := lease.TaskStore{Feed: st}
	w := feed.Writer{Author: feed.Author{ID: "owner", Role: "owner"}, Worker: "owner@" + machine, Lease: "task"}
	g, err := ts.TakeTask(w, os.Getpid(), contract.FormatTime(time.Now()))
	if err != nil {
		h.t.Fatal(err)
	}
	w.Fence = g.Fence
	if _, err := st.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
		h.t.Fatal(err)
	}
	body := jobBody(h.t, id)
	if repo != "" {
		body["repo"] = repo
	}
	if _, err := st.Post(feed.Input{Type: "job", Job: id, Body: body}, w); err != nil {
		h.t.Fatal(err)
	}
	if err := ts.ReleaseTask(w); err != nil {
		h.t.Fatal(err)
	}
}

// Row 2: the Repo func gets the task folder and the posted Job's repo, and what it answers is the spawn line's repo.
func TestVcRepoSpawnLineCarriesTheRepoFolder(t *testing.T) {
	h := newStreamHome(t, 2)
	vcRepoPost(h, 0, "J-1", "plugin")
	vcRepoPost(h, 1, "J-1", "")
	s := h.stream(4242, nil)
	asked := map[string][]string{}
	s.Repo = func(dir, repo string) (string, error) {
		asked[dir] = append(asked[dir], repo)
		if repo == "" {
			return "/abs/app", nil
		}
		return "/abs/" + repo, nil
	}
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 2 {
		t.Fatalf("want two spawn lines, got %v", spawns)
	}
	want := map[string]string{h.dirs[0]: "/abs/plugin", h.dirs[1]: "/abs/app"}
	for _, sp := range spawns {
		dir, _ := sp["task_dir"].(string)
		if sp["repo"] != want[dir] {
			t.Errorf("spawn of %s repo = %v, want %s (line %v)", dir, sp["repo"], want[dir], sp)
		}
	}
	if got := asked[h.dirs[0]]; len(got) != 1 || got[0] != "plugin" {
		t.Errorf("Repo func asked for %v in the task naming plugin, want [plugin]", got)
	}
	if got := asked[h.dirs[1]]; len(got) != 1 || got[0] != "" {
		t.Errorf("Repo func asked for %v in the task naming none, want [\"\"]", got)
	}
}

// Row 2: with no Repo func the spawn line has no repo key, as on main.
func TestVcRepoNilRepoFuncPrintsNoRepoKey(t *testing.T) {
	h := newStreamHome(t, 1)
	vcRepoPost(h, 0, "J-1", "plugin")
	s := h.stream(4242, nil)
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 {
		t.Fatalf("want one spawn line, got %v", spawns)
	}
	if v, ok := spawns[0]["repo"]; ok {
		t.Fatalf("a nil Repo func printed repo %v: %v", v, spawns[0])
	}
}

// A Repo func error is one diag line; the spawn line is still printed, without a repo key.
func TestVcRepoFuncErrorIsOneDiagLine(t *testing.T) {
	h := newStreamHome(t, 1)
	vcRepoPost(h, 0, "J-1", "nope")
	s := h.stream(4242, nil)
	s.Repo = func(string, string) (string, error) { return "/half", errors.New("no repo nope in project p") }
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 {
		t.Fatalf("want one spawn line, got %v", spawns)
	}
	if v, ok := spawns[0]["repo"]; ok {
		t.Fatalf("a failed Repo func printed repo %v: %v", v, spawns[0])
	}
	diag := strings.TrimSpace(h.diag.String())
	if strings.Count(diag, "\n") != 0 || !strings.Contains(diag, "J-1") || !strings.Contains(diag, "no repo nope") {
		t.Fatalf("want one diag line naming the Job and the error, got %q", diag)
	}
}
