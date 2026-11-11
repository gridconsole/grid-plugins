package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The app's worker starts a Job session with GRID_TASK and never sets GRID_FEED_TASK.
// These tests give the session only the app's names and run from a worktree outside the
// task folder, so the cwd names no task.

const appEnvOtherID = "01ARZ3NDEKTSV4RRFFQ69G5FB0"
const appEnvThirdID = "01ARZ3NDEKTSV4RRFFQ69G5FB1"

// appEnvTasks is the harness's task plus two others in the same project, and a
// worktree folder outside all three.
func appEnvTasks(t *testing.T) (h *harness, first, second, third, outside string) {
	t.Helper()
	h, first = feedHarness(t)
	// The app starts a Job session with GRID_ROLE=worker.
	t.Setenv("GRID_ROLE", "worker")
	second =filepath.Join(h.gridHome, "projects", "example", "tasks", "T-08-other")
	third = filepath.Join(h.gridHome, "projects", "example", "tasks", "T-09-third")
	h.write(filepath.Join(second, "task.md"), "---\nid: "+appEnvOtherID+"\n---\n")
	h.write(filepath.Join(third, "task.md"), "---\nid: "+appEnvThirdID+"\n---\n")
	outside = filepath.Join(h.gridHome, "projects", "example", "worktrees", "J-1")
	h.mkdir(outside)
	return h, first, second, third, outside
}

// appEnvPost runs grid feed post from cwd; a non-empty task is passed as --task.
func appEnvPost(t *testing.T, h *harness, cwd, text, task string) int {
	t.Helper()
	args := []string{"feed", "post", "--type", "post", "--body", `{"ref":"task","kind":"message","text":"` + text + `"}`}
	if task != "" {
		args = append(args, "--task", task)
	}
	return h.run(cwd, args...)
}

// hasFeed reports whether a task folder holds any Feed record file.
func hasFeed(dir string) bool {
	entries, err := os.ReadDir(filepath.Join(dir, "feed"))
	return err == nil && len(entries) > 0
}

// Row 4: with GRID_TASK set, no GRID_FEED_TASK and no --task, grid feed read acts on
// GRID_TASK's task from a folder that names no task.
func TestAppEnvFeedReadDefaultsToGridTask(t *testing.T) {
	h, task, _, _, outside := appEnvTasks(t)
	if code := appEnvPost(t, h, h.home, "text-of-the-session-task", task); code != 0 {
		t.Fatalf("seed post: %d %s", code, h.errb)
	}
	t.Setenv("GRID_TASK", task)
	if code := h.run(outside, "feed", "read", "--json"); code != 0 || !strings.Contains(h.out.String(), "text-of-the-session-task") {
		t.Fatalf("feed read with only GRID_TASK: exit %d out %s err %s", code, h.out, h.errb)
	}
}

// Row 4: grid job post acts on GRID_TASK's task, as it does for GRID_FEED_TASK.
func TestAppEnvJobPostDefaultsToGridTask(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	t.Setenv("GRID_TASK", a.path)
	a.postFromAgentCwd(t, filepath.Join(a.h.gridHome, "projects", "example", "worktrees", "J-1"))
}

// Row 4: the order is --task, GRID_FEED_TASK, GRID_TASK, then the cwd, in grid feed.
func TestAppEnvFeedTaskOrder(t *testing.T) {
	for _, tc := range []struct {
		name               string
		feedTask, gridTask string // which of first/second/third each variable names; "" is unset
		flag               string
		want               string
	}{
		{"GRID_TASK alone", "", "second", "", "second"},
		{"GRID_FEED_TASK beats GRID_TASK", "first", "second", "", "first"},
		{"--task beats both", "first", "second", "third", "third"},
		{"--task beats GRID_TASK", "", "second", "third", "third"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, first, second, third, outside := appEnvTasks(t)
			path := map[string]string{"first": first, "second": second, "third": third, "": ""}
			t.Setenv("GRID_FEED_TASK", path[tc.feedTask])
			t.Setenv("GRID_TASK", path[tc.gridTask])
			if code := appEnvPost(t, h, outside, "which-task", path[tc.flag]); code != 0 {
				t.Fatalf("post: %d %s", code, h.errb)
			}
			for _, name := range []string{"first", "second", "third"} {
				if got := hasFeed(path[name]); got != (name == tc.want) {
					t.Fatalf("task %s holds a feed: %v, want the post in %s only", name, got, tc.want)
				}
			}
		})
	}
}

// Row 4: the same order in grid job post: GRID_FEED_TASK beats GRID_TASK, and --task beats both.
func TestAppEnvJobPostTaskOrder(t *testing.T) {
	a := newAgentTask(t)
	a.agentEnv(t, a.epoch())
	missing := filepath.Join(a.h.gridHome, "projects", "example", "tasks", "T-99-missing")
	t.Setenv("GRID_FEED_TASK", a.path)
	t.Setenv("GRID_TASK", missing)
	a.postFromAgentCwd(t, filepath.Join(a.h.gridHome, "projects", "example", "worktrees", "J-1"))

	t.Setenv("GRID_FEED_TASK", missing)
	t.Setenv("GRID_TASK", missing)
	body, _ := json.Marshal(workerNextBody(t, "J-3"))
	if code := a.h.run(a.h.home, "job", "post", "--task", a.path, "--type", "job", "--body", string(body)); code != 0 {
		t.Fatalf("--task should win over both variables: exit %d %s", code, a.h.errb)
	}
}

// Row 5: nothing new is refused. With GRID_TASK set and GRID_FEED_TASK unset, MCP
// feed_read with no task reads GRID_TASK's task, and a task argument naming another
// task is answered, not refused, as on main.
func TestAppEnvMCPReadsGridTaskAndRefusesNothing(t *testing.T) {
	h, task, other, _, _ := appEnvTasks(t)
	if code := appEnvPost(t, h, h.home, "text-of-the-session-task", task); code != 0 {
		t.Fatalf("seed post: %d %s", code, h.errb)
	}
	if code := appEnvPost(t, h, h.home, "text-of-the-other-task", other); code != 0 {
		t.Fatalf("seed post: %d %s", code, h.errb)
	}
	t.Setenv("GRID_TASK", task)
	got := mcpCall(t, h, "feed_read", `{}`)
	if strings.Contains(got, `"isError":true`) || !strings.Contains(got, "text-of-the-session-task") || strings.Contains(got, "text-of-the-other-task") {
		t.Fatalf("feed_read with no task should read GRID_TASK's task: %s", got)
	}
	got = mcpCall(t, h, "feed_read", `{"task":"`+other+`"}`)
	if strings.Contains(got, `"isError":true`) || !strings.Contains(got, "text-of-the-other-task") || strings.Contains(got, "text-of-the-session-task") {
		t.Fatalf("feed_read naming another task should answer that task, with no isError: %s", got)
	}
	got = mcpCall(t, h, "feed_post", `{"type":"post","body":{"ref":"task","kind":"message","text":"posted-to-the-other-task"},"task":"`+other+`"}`)
	if strings.Contains(got, `"isError":true`) {
		t.Fatalf("feed_post naming another task was refused: %s", got)
	}
	if code := h.run(h.home, "feed", "read", "--task", other, "--json"); code != 0 || !strings.Contains(h.out.String(), "posted-to-the-other-task") {
		t.Fatalf("the post did not reach the other task: %d %s %s", code, h.out, h.errb)
	}
}

// Row 5: GRID_WORKER_ID is not read by the writer check. A value that is not
// <member>@<machine id>, with GRID_WORKER unset, does not refuse a post that passes
// today: a --task post, and an MCP post under GRID_FEED_TASK. This holds on main too.
func TestAppEnvWorkerIDIsNotCheckedAsTheWriter(t *testing.T) {
	for _, id := range []string{"w-1", "not-a-worker", "someone@elsewhere", "member@" + feedMachine} {
		t.Run(id, func(t *testing.T) {
			h, task, _, _, _ := appEnvTasks(t)
			t.Setenv("GRID_WORKER_ID", id)
			t.Setenv("GRID_WORKER", "")
			if code := appEnvPost(t, h, h.home, "with-a-flag", task); code != 0 {
				t.Fatalf("post with --task: %d %s", code, h.errb)
			}
			t.Setenv("GRID_FEED_TASK", task)
			got := mcpCall(t, h, "feed_post", `{"type":"post","body":{"ref":"task","kind":"message","text":"via-mcp"}}`)
			if strings.Contains(got, `"isError":true`) {
				t.Fatalf("MCP post refused: %s", got)
			}
		})
	}
}

// Row 5, with the new default: the same posts reach the GRID_TASK task whatever
// GRID_WORKER_ID holds, from a folder outside the task, by CLI and by MCP.
func TestAppEnvWorkerIDWithGridTaskIsNotRefused(t *testing.T) {
	h, task, _, _, outside := appEnvTasks(t)
	t.Setenv("GRID_WORKER_ID", "not-a-worker")
	t.Setenv("GRID_WORKER", "")
	t.Setenv("GRID_TASK", task)
	if code := appEnvPost(t, h, outside, "with-grid-task", ""); code != 0 {
		t.Fatalf("post with GRID_TASK: %d %s", code, h.errb)
	}
	if got := mcpCall(t, h, "feed_post", `{"type":"post","body":{"ref":"task","kind":"message","text":"via-mcp"}}`); strings.Contains(got, `"isError":true`) {
		t.Fatalf("MCP post refused: %s", got)
	}
	if code := h.run(outside, "feed", "read", "--json"); code != 0 || !strings.Contains(h.out.String(), "with-grid-task") || !strings.Contains(h.out.String(), "via-mcp") {
		t.Fatalf("both posts should be in GRID_TASK's task: %d %s %s", code, h.out, h.errb)
	}
}
