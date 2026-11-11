package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/orchestrator"
	"github.com/gridconsole/grid-plugins/internal/project"
)

// newTaskHarness is a Grid home with one registered project and no environment
// that names a worker, so the caller is the person at the terminal.
func newTaskHarness(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	h.record(project.Record{Name: "p", ID: "01PROJECT"})
	h.env.Getenv = func(string) string { return "" }
	h.env.Machine = feedMachine // a real machine id is a ULID; a worker id needs one
	return h
}

// newTask runs grid task new as the website's New task route does and returns the task folder.
func newTask(t *testing.T, h *harness, args ...string) string {
	t.Helper()
	all := append([]string{"task", "new", "Add a search box to the site", "--project", "p", "--json"}, args...)
	if code := h.run(h.home, all...); code != 0 {
		t.Fatalf("grid task new exit %d: %s", code, h.errb)
	}
	var made struct{ ID, Path string }
	if err := json.Unmarshal(h.out.Bytes(), &made); err != nil {
		t.Fatal(err)
	}
	return made.Path
}

// streamPass is one pass of the stream as grid worker next builds it (worker.go).
func streamPass(t *testing.T, h *harness) string {
	t.Helper()
	pid := os.Getpid()
	var out bytes.Buffer
	s := &orchestrator.Stream{
		Member: "member", Machine: h.env.Machine, Role: "member",
		PID: pid, PIDStart: contract.FormatTime(time.Now()),
		Out: &out, Diag: h.errb,
		Tasks: func() ([]string, error) { return workerTaskDirs(h.env) },
	}
	s.Configure = func(st *orchestrator.Store, taskID string) {
		configureTickStore(h.env, st, filepath.Base(filepath.Dir(filepath.Dir(st.Feed.TaskDir))), taskID)
		st.Diagnostics = h.errb
	}
	if err := s.Pass(t.Context()); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestTaskNewFirstJobIsSpawnedByTheStream(t *testing.T) {
	for _, size := range []string{"auto", "small", "normal", "large"} {
		t.Run(size, func(t *testing.T) {
			h := newTaskHarness(t)
			path := newTask(t, h, "--size", size)
			lines := streamPass(t, h)
			var spawned bool
			for _, line := range strings.Split(strings.TrimSpace(lines), "\n") {
				var got map[string]struct {
					ID string `json:"id"`
				}
				if json.Unmarshal([]byte(line), &got) == nil && got["spawn"].ID == "J-1" {
					spawned = true
				}
			}
			if !spawned {
				t.Fatalf("one stream pass over a new %s task emitted no spawn for J-1\nstream: %q\nstderr: %s", size, lines, h.errb)
			}
			if _, err := os.Stat(filepath.Join(path, "task.md")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTaskNewFirstJobRecord(t *testing.T) {
	h := newTaskHarness(t)
	h.env.Getenv = func(k string) string {
		if k == "GRID_WHO" {
			return "alex"
		}
		return ""
	}
	path := newTask(t, h)
	id, err := feed.TaskID(path)
	if err != nil {
		t.Fatal(err)
	}
	store := feed.Store{TaskDir: path, TaskID: id}
	read, err := store.Read(feed.Filter{Type: "job"})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("want one job record, got %d: %v", len(read.Records), err)
	}
	rec := read.Records[0]
	if rec.Job == nil || *rec.Job != "J-1" || rec.Author.ID != "alex" || rec.Author.Role != "member" {
		t.Fatalf("J-1 must be authored by the task owner: job %v author %+v", rec.Job, rec.Author)
	}
	body, err := json.Marshal(rec.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := contract.Validate("job-record", body); err != nil {
		t.Fatalf("J-1 is not a valid job-record: %v\n%s", err, body)
	}
	var j struct {
		Role   string   `json:"role"`
		Skills []string `json:"skills"`
		Intent struct{ Owner, Item, Goal string }
		Verify struct{ Kind, Path string }
	}
	if err := json.Unmarshal(body, &j); err != nil {
		t.Fatal(err)
	}
	if j.Role != "orchestrator" || strings.Join(j.Skills, ",") != "plan-jobs" || j.Intent.Owner != "intent.md#request" ||
		j.Verify.Kind != "file-exists" || j.Verify.Path != "plan.md" {
		t.Fatalf("unexpected first Job: %s", body)
	}
	h.out.Reset()
	if code := h.run(path, "job", "tree", "--json"); code != 0 || !strings.Contains(h.out.String(), "J-1") || strings.Contains(h.out.String(), "accepted") {
		t.Fatalf("job tree %d: %s %s", code, h.out, h.errb)
	}
}

func TestTaskNewFirstJobRendersTheOrchestratorInstructions(t *testing.T) {
	h := newTaskHarness(t)
	path := newTask(t, h)
	if code := h.run(path, "job", "render", path, "J-1", "--json"); code != 0 {
		t.Fatalf("job render exit %d: %s %s", code, h.out, h.errb)
	}
	saved, err := os.ReadFile(filepath.Join(path, "jobs", "J-1", "brief.md"))
	if err != nil {
		t.Fatalf("job render saved no brief: %v (render printed %s)", err, h.out)
	}
	out := struct{ Brief string }{string(saved)}
	agent, err := os.ReadFile("../../agents/orchestrator.md")
	if err != nil {
		t.Fatal(err)
	}
	_, body, _ := strings.Cut(strings.TrimPrefix(string(agent), "---\n"), "\n---\n")
	for _, want := range []string{strings.TrimSpace(body), "Add a search box to the site"} {
		if !strings.Contains(out.Brief, want) {
			t.Fatalf("the rendered prompt lacks %q:\n%s", want, out.Brief)
		}
	}
	if strings.Contains(out.Brief, "model: opus") {
		t.Fatalf("the front matter of the agent file must not reach the prompt:\n%s", out.Brief)
	}
}
