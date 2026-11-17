package orchestrator

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// jbPostWith posts J-1 into task i as vcRepoPost does, with the given extra job-record keys.
func jbPostWith(h *streamHome, i int, extra map[string]any) {
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
	body := jobBody(h.t, "J-1")
	for k, v := range extra {
		body[k] = v
	}
	if _, err := st.Post(feed.Input{Type: "job", Job: "J-1", Body: body}, w); err != nil {
		h.t.Fatal(err)
	}
	if err := ts.ReleaseTask(w); err != nil {
		h.t.Fatal(err)
	}
}

// jbBriefHome is a stream home of three tasks: task 0's J-1 names model, effort and
// base; task 1's J-1 names only effort; task 2's J-1 names none of them.
func jbBriefHome(t *testing.T) *streamHome {
	t.Helper()
	h := newStreamHome(t, 3)
	bodies := []map[string]any{
		{"model": "opus", "effort": "medium", "base": "lane-P-run-jobs"},
		{"effort": "high"},
		{},
	}
	for i, extra := range bodies {
		jbPostWith(h, i, extra)
	}
	return h
}

// jbStream is the stream of h whose Model answers the role's values and whose Repo answers a fixed folder.
func jbStream(h *streamHome) *Stream {
	s := h.stream(4242, nil)
	s.Model = func(string, string) (string, string, error) { return "role-model", "role-effort", nil }
	s.Repo = func(string, string) (string, error) { return "/abs/repo", nil }
	return s
}

// Row 3: a Job's own model, effort and base reach its spawn line; a value it does not name keeps the role's.
func TestJbSpawnLineCarriesTheJobModelEffortAndBase(t *testing.T) {
	h := jbBriefHome(t)
	h.pass(jbStream(h))
	spawns := kinds(h.lines(), "spawn")
	if len(spawns) != 3 {
		t.Fatalf("want three spawn lines, got %v", spawns)
	}
	want := map[string][3]any{
		h.dirs[0]: {"opus", "medium", "lane-P-run-jobs"},
		h.dirs[1]: {"role-model", "high", nil},
		h.dirs[2]: {"role-model", "role-effort", nil},
	}
	for _, sp := range spawns {
		dir, _ := sp["task_dir"].(string)
		w := want[dir]
		if sp["model"] != w[0] || sp["effort"] != w[1] || sp["base"] != w[2] {
			t.Errorf("spawn of %s: model %v effort %v base %v, want %v (line %v)", dir, sp["model"], sp["effort"], sp["base"], w, sp)
		}
	}
}

// Row 3: the spawn line of a Job that names none of the three is byte for byte the line main prints.
func TestJbSpawnLineOfAPlainJobIsUnchanged(t *testing.T) {
	h := jbBriefHome(t)
	h.pass(jbStream(h))
	var raw string
	for _, l := range strings.Split(strings.TrimSpace(h.out.String()), "\n") {
		if strings.HasPrefix(l, `{"spawn":`) && strings.Contains(l, `"task_dir":`+jbQuote(h.dirs[2])) {
			raw = l
		}
	}
	if raw == "" {
		t.Fatalf("no spawn line for the plain task in %s", h.out)
	}
	var got struct{ Spawn map[string]any }
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	sp := got.Spawn
	// The spawn line as main writes it, built from the same values in main's key order.
	mainLine := struct {
		Spawn struct {
			Task      string  `json:"task"`
			TaskDir   string  `json:"task_dir"`
			ID        string  `json:"id"`
			Holder    string  `json:"holder"`
			Fence     float64 `json:"fence"`
			Gen       float64 `json:"gen"`
			Attempt   float64 `json:"attempt"`
			Assign    string  `json:"assign"`
			TaskEpoch string  `json:"task_epoch"`
			Model     string  `json:"model,omitempty"`
			Effort    string  `json:"effort,omitempty"`
			Repo      string  `json:"repo,omitempty"`
		} `json:"spawn"`
	}{}
	m := &mainLine.Spawn
	m.Task, m.TaskDir, m.ID, m.Holder = sp["task"].(string), sp["task_dir"].(string), sp["id"].(string), sp["holder"].(string)
	m.Fence, m.Gen, m.Attempt = sp["fence"].(float64), sp["gen"].(float64), sp["attempt"].(float64)
	m.Assign, m.TaskEpoch = sp["assign"].(string), sp["task_epoch"].(string)
	m.Model, m.Effort, m.Repo = "role-model", "role-effort", "/abs/repo"
	want, err := json.Marshal(mainLine)
	if err != nil {
		t.Fatal(err)
	}
	if raw != string(want) {
		t.Fatalf("plain spawn line changed:\n got %s\nwant %s", raw, want)
	}
}

func jbQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
