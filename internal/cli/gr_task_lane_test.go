package cli

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// grLaneBody is the body and the plan the lane moves must leave alone.
const (
	grLaneBody = "\n## Result\nKeep this body exactly.\n"
	grLanePlan = "# Plan\n\n- [ ] step one (kept)\n"
)

// grLaneFixture is a task of the shared project "example" on a temp home, in the
// given state, with extra header keys (a comment, odd spacing, a flow mapping), a
// body, a plan and an earlier history line.
func grLaneFixture(t *testing.T, state, kind, lane, role string) (*harness, task.Task) {
	t.Helper()
	h := newHarness(t)
	for _, key := range []string{"HOME", "GRID_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		dir := h.home
		if key != "HOME" {
			dir = filepath.Join(h.root, strings.ToLower(key))
		}
		if key == "GRID_HOME" {
			dir = h.gridHome
		}
		t.Setenv(key, dir)
	}
	h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	fixture := taskFixtureProject(t, h, "example")
	h.env.Getenv = func(key string) string {
		if key == "GRID_ROLE" {
			return role
		}
		return ""
	}
	extra := "state: " + state + "\nupdated: '2026-01-01T00:00:00Z'\nsummary:  'Keep this exact spacing' # comment\nextra: { one: 1, two: 2 }"
	if kind != "" {
		extra = "kind: " + kind + "\n" + extra
	}
	if lane != "" {
		extra += "\nlane: " + lane
	}
	path := filepath.Join(fixture.Path, "task.md")
	h.write(path, strings.Replace(readTaskTestFile(t, path), "state: queued", extra, 1)+grLaneBody)
	h.write(filepath.Join(fixture.Path, "plan.md"), grLanePlan)
	h.write(filepath.Join(fixture.Path, "history.md"), "- Earlier history\n")
	return h, fixture
}

// grLaneInsert is task.md with `lane: <lane>` added as the last header key.
func grLaneInsert(original, lane string) string {
	end := strings.Index(original, "\n---\n") + 1
	return original[:end] + "lane: " + lane + "\n" + original[end:]
}

type grLaneAnswer struct {
	State string `json:"state"`
	Task  struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Lane   string `json:"lane"`
	} `json:"task"`
}

func grLaneRun(t *testing.T, h *harness, id, lane string, code int) grLaneAnswer {
	t.Helper()
	if got := h.run(h.home, "task", "lane", id, lane, "--project", "example", "--json"); got != code {
		t.Fatalf("task lane %s %s: exit %d, want %d: %s", id, lane, got, code, h.errb)
	}
	var answer grLaneAnswer
	if code == 0 {
		if err := json.Unmarshal(h.out.Bytes(), &answer); err != nil {
			t.Fatalf("answer %q: %v", h.out, err)
		}
	}
	return answer
}

// grLaneUnchanged checks that a refused move left every file of the task folder
// byte-equal; the empty .task.lock the writer takes is the only file it may add,
// as for rename and decline.
func grLaneUnchanged(t *testing.T, before map[string][]byte, dir string) {
	t.Helper()
	after := taskTestSnapshot(t, dir)
	delete(after, string(filepath.Separator)+".task.lock")
	delete(before, string(filepath.Separator)+".task.lock")
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("a refused move changed the task folder: %d files before, %d after", len(before), len(after))
	}
}

// grLaneNewHistory is the history lines a move appended.
func grLaneNewHistory(t *testing.T, dir, before string) []string {
	t.Helper()
	after := readTaskTestFile(t, filepath.Join(dir, "history.md"))
	if !strings.HasPrefix(after, before) {
		t.Fatalf("history was rewritten:\n%s", after)
	}
	return strings.Split(strings.TrimSuffix(strings.TrimPrefix(after, before), "\n"), "\n")
}

// A running task moved to Doing gets `lane: doing` and one history line; the
// header keys, the body and the plan are as they were. Moving it back to Planning
// says the plan and step were kept.
func TestGrTaskLaneMovesAndKeepsTheRest(t *testing.T) {
	h, fixture := grLaneFixture(t, "running", "", "", "")
	path := filepath.Join(fixture.Path, "task.md")
	original := readTaskTestFile(t, path)
	history := readTaskTestFile(t, filepath.Join(fixture.Path, "history.md"))

	answer := grLaneRun(t, h, fixture.ID, "doing", 0)
	if answer.State != "applied" || answer.Task.ID != fixture.ID || answer.Task.Status != "running" || answer.Task.Lane != "doing" {
		t.Fatalf("answer %+v", answer)
	}
	if !strings.Contains(h.out.String(), `"lane":"doing"`) {
		t.Fatalf("stdout lacks \"lane\":\"doing\": %s", h.out)
	}
	moved := readTaskTestFile(t, path)
	if want := grLaneInsert(original, "doing"); moved != want {
		t.Fatalf("task.md:\n%s\nwant:\n%s", moved, want)
	}
	lines := grLaneNewHistory(t, fixture.Path, history)
	if len(lines) != 1 || !strings.Contains(lines[0], "moved to Doing") {
		t.Fatalf("history gained %q", lines)
	}

	history = readTaskTestFile(t, filepath.Join(fixture.Path, "history.md"))
	grLaneRun(t, h, fixture.ID, "planning", 0)
	if want := strings.Replace(moved, "lane: doing\n", "lane: planning\n", 1); readTaskTestFile(t, path) != want {
		t.Fatalf("task.md after the move back:\n%s", readTaskTestFile(t, path))
	}
	lines = grLaneNewHistory(t, fixture.Path, history)
	if len(lines) != 1 || !strings.Contains(lines[0], "moved back to Planning") || !strings.Contains(lines[0], "plan and step kept") {
		t.Fatalf("history gained %q", lines)
	}
	if got := readTaskTestFile(t, filepath.Join(fixture.Path, "plan.md")); got != grLanePlan {
		t.Fatalf("plan.md changed: %q", got)
	}

	// A later lane again, and Waiting to start spelled out, name the lane.
	grLaneRun(t, h, fixture.ID, "delivering", 0)
	if last := grLaneNewHistory(t, fixture.Path, history); !strings.Contains(last[len(last)-1], "moved to Delivering") {
		t.Fatalf("history gained %q", last)
	}
}

// Inbox sets the state to queued where the state table allows it; a running task
// has no running -> queued move, so it is refused with nothing written.
func TestGrTaskLaneInbox(t *testing.T) {
	h, fixture := grLaneFixture(t, "paused", "", "doing", "")
	path := filepath.Join(fixture.Path, "task.md")
	history := readTaskTestFile(t, filepath.Join(fixture.Path, "history.md"))
	answer := grLaneRun(t, h, fixture.ID, "inbox", 0)
	if answer.Task.Status != "queued" || answer.Task.Lane != "inbox" {
		t.Fatalf("answer %+v", answer)
	}
	got := readTaskTestFile(t, path)
	if !strings.Contains(got, "\nstate: queued\n") || !strings.Contains(got, "\nlane: inbox\n") || strings.Contains(got, "paused") {
		t.Fatalf("task.md:\n%s", got)
	}
	// The earlier lane was later, so this is a move back.
	if lines := grLaneNewHistory(t, fixture.Path, history); len(lines) != 1 || !strings.Contains(lines[0], "moved back to Waiting to start") {
		t.Fatalf("history gained %q", lines)
	}

	h, fixture = grLaneFixture(t, "running", "", "", "")
	before := taskTestSnapshot(t, fixture.Path)
	grLaneRun(t, h, fixture.ID, "inbox", 3)
	if !strings.Contains(h.errb.String(), "running") {
		t.Fatalf("refusal does not name the status: %s", h.errb)
	}
	grLaneUnchanged(t, before, fixture.Path)
}

// Every refusal leaves the whole task folder byte-equal.
func TestGrTaskLaneRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, state, kind, lane, role, to, diagnostic string
		code                                          int
	}{
		{name: "closed", state: "closed", to: "doing", code: 3, diagnostic: "closed"},
		{name: "planner", state: "running", kind: "planner", to: "doing", code: 3, diagnostic: "Planner"},
		{name: "same_lane", state: "running", lane: "doing", to: "doing", code: 3, diagnostic: "already in the doing lane"},
		{name: "done", state: "running", to: "done", code: 2, diagnostic: "lane must be one of"},
		{name: "guest", state: "running", role: "guest", to: "doing", code: 1, diagnostic: "may not move a task to a lane"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, fixture := grLaneFixture(t, tc.state, tc.kind, tc.lane, tc.role)
			before := taskTestSnapshot(t, fixture.Path)
			grLaneRun(t, h, fixture.ID, tc.to, tc.code)
			if !strings.Contains(h.errb.String(), tc.diagnostic) {
				t.Fatalf("stderr %q lacks %q", h.errb, tc.diagnostic)
			}
			grLaneUnchanged(t, before, fixture.Path)
		})
	}
}

// An unknown task answers no_such_task, and the line on stderr is the same with
// and without --json.
func TestGrTaskLaneNoSuchTask(t *testing.T) {
	h, _ := grLaneFixture(t, "running", "", "", "")
	if code := h.run(h.home, "task", "lane", "T-99", "doing"); code != 1 {
		t.Fatalf("plain exit %d", code)
	}
	plain := h.errb.String()
	if code := h.run(h.home, "task", "lane", "T-99", "doing", "--json"); code != 1 {
		t.Fatalf("json exit %d", code)
	}
	var answer struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &answer); err != nil || answer.Error.Code != "no_such_task" {
		t.Fatalf("stdout %q: %v", h.out, err)
	}
	if h.errb.String() != plain || plain == "" {
		t.Fatalf("stderr differs: %q vs %q", h.errb.String(), plain)
	}
}

// --project names the project the task must be in.
func TestGrTaskLaneProjectMismatch(t *testing.T) {
	h, fixture := grLaneFixture(t, "running", "", "", "")
	h.record(project.Record{Name: "other", ID: "01OTHER"})
	before := taskTestSnapshot(t, fixture.Path)
	if code := h.run(h.home, "task", "lane", fixture.ID, "doing", "--project", "other"); code == 0 {
		t.Fatalf("a task of another project was moved")
	}
	grLaneUnchanged(t, before, fixture.Path)
}
