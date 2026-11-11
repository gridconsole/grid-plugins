package orchestrator

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

// postRoles puts one Job per role into task i, J-1, J-2 and so on, as an owner does.
func (h *streamHome) postRoles(i int, roles ...string) {
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
	for n, role := range roles {
		id := "J-" + string(rune('1'+n))
		body := jobBody(h.t, id)
		body["role"] = role
		if _, err := st.Post(feed.Input{Type: "job", Job: id, Body: body}, w); err != nil {
			h.t.Fatal(err)
		}
	}
	if err := ts.ReleaseTask(w); err != nil {
		h.t.Fatal(err)
	}
}

func modelFrom(home string) func(string, string) (string, string, error) {
	return func(_, role string) (string, string, error) { return settings.Store{Home: home}.ModelFor(role) }
}

// Row 2: each spawn line carries the model and effort of its Job's role, from the settings the stream reads.
func TestStreamSpawnCarriesTheRoleModel(t *testing.T) {
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder", "grid-builder", "checker", "researcher", "orchestrator")
	s := h.stream(1234, nil)
	s.Model = modelFrom(t.TempDir())
	h.pass(s)
	got := map[string]string{}
	for _, sp := range kinds(h.lines(), "spawn") {
		model, _ := sp["model"].(string)
		effort, _ := sp["effort"].(string)
		got[sp["id"].(string)] = model + "/" + effort
	}
	want := map[string]string{"J-1": "sonnet/medium", "J-2": "sonnet/medium", "J-3": "sonnet/low", "J-4": "opus/medium", "J-5": "opus/medium"}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("job %s (%v): model/effort = %q, want %q", id, got, got[id], w)
		}
	}
}

func spawnKeys(sp map[string]any) string {
	var keys []string
	for k := range sp {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

// Row 4: no model to send is no key on the line, and the line has exactly the nine keys it had before.
func TestStreamSpawnWithoutAModelKeepsTheNineKeys(t *testing.T) {
	const nine = "assign,attempt,fence,gen,holder,id,task,task_dir,task_epoch"
	for name, model := range map[string]func(string, string) (string, string, error){
		"nil Model":     nil,
		"empty strings": func(string, string) (string, string, error) { return "", "", nil },
	} {
		h := newStreamHome(t, 1)
		h.postRoles(0, "builder")
		s := h.stream(1234, nil)
		s.Model = model
		h.pass(s)
		spawns := kinds(h.lines(), "spawn")
		if len(spawns) != 1 {
			t.Fatalf("%s: %d spawn lines, want 1", name, len(spawns))
		}
		if got := spawnKeys(spawns[0]); got != nine {
			t.Errorf("%s: spawn keys %s, want %s", name, got, nine)
		}
	}
}

// Row 5: a settings file that does not parse costs the spawn its two keys and nothing else.
func TestStreamSpawnSurvivesBrokenSettings(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, "settings.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newStreamHome(t, 1)
	h.postRoles(0, "builder")
	s := h.stream(1234, nil)
	s.Model = modelFrom(home)
	h.pass(s)
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 1 {
		t.Fatalf("%d spawn lines, want 1 (the Job is not skipped)", len(spawns))
	}
	if got := spawnKeys(spawns[0]); got != "assign,attempt,fence,gen,holder,id,task,task_dir,task_epoch" {
		t.Errorf("spawn keys %s, want the nine without model and effort", got)
	}
	diag := strings.TrimSpace(h.diag.String())
	if strings.Count(diag, "\n") != 0 || !strings.Contains(diag, "settings.json") || !strings.Contains(diag, h.ids[0]) || !strings.Contains(diag, "J-1") {
		t.Fatalf("want one Diag line naming the task, the Job and the settings error, got %q", diag)
	}
	h.pass(s)
	if again := kinds(h.lines(), "spawn"); len(again) != 0 || strings.TrimSpace(h.diag.String()) != diag {
		t.Errorf("second pass printed %v and diag %q; want nothing new", again, h.diag.String())
	}
}
