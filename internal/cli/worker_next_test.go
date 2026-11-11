package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

const secondTaskID = "01ARZ3NDEKTSV4RRFFQ69G5FAX"

// workerNextBody is the basic Job record under a new id.
func workerNextBody(t *testing.T, id string) map[string]any {
	t.Helper()
	b, err := os.ReadFile("../../contract/fixtures/job-record/valid/basic.json")
	if err != nil {
		t.Fatal(err)
	}
	var body map[string]any
	if err := json.Unmarshal(b, &body); err != nil {
		t.Fatal(err)
	}
	body["id"] = id
	return body
}

// ownerPost posts Job ids into a task as its owner, who takes the task lease and gives it back.
func ownerPost(t *testing.T, dir, taskID string, jobs ...string) {
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
	for _, id := range jobs {
		if _, err := store.Post(feed.Input{Type: "job", Job: id, Body: workerNextBody(t, id)}, w); err != nil {
			t.Fatal(err)
		}
	}
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
}

// TestWorkerNextProcess is the stream in a child process, as the worker runs it.
func TestWorkerNextProcess(t *testing.T) {
	if os.Getenv("GRID_TEST_WORKER_NEXT") != "1" {
		return
	}
	env := OSEnv()
	env.TempRoots = nil
	args := []string{"--grid-home", os.Getenv("GRID_TEST_HOME"), "--member", "worker", "--role", "worker",
		"worker", "next", "--json", "--pid", os.Getenv("GRID_TEST_PID"), "--pid-start", os.Getenv("GRID_TEST_PID_START")}
	os.Exit(Run(args, env))
}

// koWnThirdTaskID is the task a stream test adds while the stream runs.
const koWnThirdTaskID = "01ARZ3NDEKTSV4RRFFQ69G5FAY"

// koWnStart runs the real `grid worker next` in a child process (TestWorkerNextProcess) on gridHome
// under the lease identity pid and start, and returns the child with the lines of its stdout. The
// child's stdout is an OS pipe, so Wait never cuts the reader off; the channel closes when the child's
// stdout ends. The caller stops the child.
func koWnStart(t *testing.T, gridHome string, pid int, start string, stderr *bytes.Buffer) (*exec.Cmd, <-chan string) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkerNextProcess$")
	cmd.Env = append(os.Environ(), "GRID_TEST_WORKER_NEXT=1", "GRID_TEST_HOME="+gridHome,
		"GRID_TEST_PID="+strconv.Itoa(pid), "GRID_TEST_PID_START="+start)
	cmd.Stderr = stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	cmd.Stdout = w
	err = cmd.Start()
	_ = w.Close() // the child holds the write end now
	if err != nil {
		t.Fatal(err)
	}
	lines := make(chan string, 16)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	return cmd, lines
}

// koWnSpawn waits for the next stdout line of the stream and returns its spawn object. The 30 s
// bound is only the failure case of the select: the other case is the line itself.
func koWnSpawn(t *testing.T, lines <-chan string, stderr *bytes.Buffer) map[string]any {
	t.Helper()
	select {
	case l, ok := <-lines:
		if !ok {
			t.Fatalf("stream ended before a spawn line\nstderr: %s", stderr)
		}
		var line map[string]map[string]any
		if err := json.Unmarshal([]byte(l), &line); err != nil || line["spawn"] == nil {
			t.Fatalf("want one spawn object per line, got %q (%v)\nstderr: %s", l, err, stderr)
		}
		return line["spawn"]
	case <-time.After(30 * time.Second):
		t.Fatalf("no spawn line within 30s\nstderr: %s", stderr)
	}
	return nil
}

// P1 and P2: the stream runs until SIGINT and then exits 130, never 0 by itself; on a home with two
// tasks that each have a posted Job and a free lease it prints two spawn lines with every field, and
// the task lease of both tasks names worker@M with the worker's pid. A third task that appears while
// it runs gets its spawn line too, which shows the stream is still serving and did not exit.
func TestWorkerNextStreamsSpawnLinesUntilSIGINT(t *testing.T) {
	h, first := feedHarness(t)
	second := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-08-second")
	h.write(filepath.Join(second, "task.md"), "---\nid: "+secondTaskID+"\n---\n")
	ownerPost(t, first, feedTaskID, "J-1")
	ownerPost(t, second, secondTaskID, "J-1")
	pid, start := taskLeaseIdentity(t)

	var stderr bytes.Buffer
	cmd, lines := koWnStart(t, h.gridHome, pid, start, &stderr)
	defer func() { _ = cmd.Process.Kill() }()
	// The child is waited for in a goroutine, so "it is still running" is a channel that is not closed.
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	spawns := map[string]map[string]any{}
	for len(spawns) < 2 {
		sp := koWnSpawn(t, lines, &stderr)
		spawns[sp["task"].(string)] = sp
	}
	for dir, id := range map[string]string{first: feedTaskID, second: secondTaskID} {
		sp := spawns[id]
		if sp == nil {
			t.Fatalf("no spawn for task %s: %v", id, spawns)
		}
		h.run(dir, "lease", "status", "task", "--json")
		var tn struct {
			Holder string `json:"holder"`
			PID    int    `json:"pid"`
			Fence  int64  `json:"fence"`
		}
		if err := json.Unmarshal(h.out.Bytes(), &tn); err != nil || tn.Holder != "worker@"+feedMachine || tn.PID != pid {
			t.Fatalf("lease status of %s: %s (%v)", id, h.out, err)
		}
		want := map[string]any{"task": id, "task_dir": dir, "id": "J-1", "holder": "worker@" + feedMachine,
			"fence": float64(tn.Fence + 1), "gen": float64(0), "attempt": float64(1), "task_epoch": fmt.Sprintf("0.%d", tn.Fence)}
		for k, v := range want {
			if sp[k] != v {
				t.Errorf("spawn %s: %s = %v, want %v", id, k, sp[k], v)
			}
		}
		if a, _ := sp["assign"].(string); a == "" {
			t.Errorf("spawn %s has no assign id: %v", id, sp)
		}
	}
	// It is still running: a third task that appears now gets its spawn line, and the child has not
	// exited when it arrives. The folder is built under a name the stream does not list and renamed
	// into place once its Job is posted, so the stream never meets it while the owner holds the lease.
	tasks := filepath.Join(h.gridHome, "projects", "example", "tasks")
	stage := filepath.Join(tasks, "stage-T-09-third")
	third := filepath.Join(tasks, "T-09-third")
	h.write(filepath.Join(stage, "task.md"), "---\nid: "+koWnThirdTaskID+"\n---\n")
	ownerPost(t, stage, koWnThirdTaskID, "J-1")
	if err := os.Rename(stage, third); err != nil {
		t.Fatal(err)
	}
	sp := koWnSpawn(t, lines, &stderr)
	if sp["task"] != koWnThirdTaskID || sp["task_dir"] != third || sp["id"] != "J-1" {
		t.Fatalf("third spawn line: %v\nstderr: %s", sp, &stderr)
	}
	select {
	case err := <-exited:
		t.Fatalf("stream exited by itself: %v\nstderr: %s", err, &stderr)
	default:
	}
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	go func() {
		for range lines {
		}
	}()
	err := <-exited
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 130 {
		t.Fatalf("SIGINT exit: %v\nstderr: %s", err, &stderr)
	}
}

// P1: exit 2 on a bad flag, 3 on a holder from another machine.
func TestWorkerNextExitCodes(t *testing.T) {
	h, _ := feedHarness(t)
	pid, start := taskLeaseIdentity(t)
	base := func(rest ...string) []string {
		return append([]string{"--grid-home", h.gridHome, "--member", "worker", "--role", "worker", "worker", "next", "--json"}, rest...)
	}
	good := []string{"--pid", strconv.Itoa(pid), "--pid-start", start}
	for name, args := range map[string][]string{
		"unknown flag":         base(append([]string{"--bogus"}, good...)...),
		"no pid":               base("--pid-start", start),
		"no pid start":         base("--pid", strconv.Itoa(pid)),
		"pid start not time":   base("--pid", strconv.Itoa(pid), "--pid-start", "yesterday"),
		"pid not alive":        base("--pid", "2147483000", "--pid-start", start),
		"extra argument":       base(append([]string{"stray"}, good...)...),
		"global flag no value": {"--member"},
	} {
		if code := h.run(h.home, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2 (stderr %s)", name, code, h.errb)
		}
	}
	other := base(good...)
	other[3] = "worker@01ARZ3NDEKTSV4RRFFQ69G5FAX"
	if code := h.run(h.home, other...); code != 3 || !strings.Contains(h.errb.String(), "not on this machine") {
		t.Errorf("holder from another machine: exit %d, stderr %s", code, h.errb)
	}
}

// P6: an agent env with the task epoch posts a Job under the task lease, and a stale task epoch exits 3.
func TestJobPostWithTaskEpochFromAJobAgent(t *testing.T) {
	h, path := feedHarness(t)
	h.env.Machine = feedMachine
	store := feed.Store{TaskDir: path, TaskID: feedTaskID}
	ts := lease.TaskStore{Feed: store}
	pid, start := taskLeaseIdentity(t)
	w := feed.Writer{Author: feed.Author{ID: "worker", Role: "cli"}, Worker: "worker@" + feedMachine, Lease: "task"}
	hold := func() int64 {
		g, err := ts.TakeTask(w, pid, start)
		if err != nil {
			t.Fatal(err)
		}
		w.Fence = g.Fence
		if _, err := store.Handover(feed.Input{Body: map[string]any{}}, w); err != nil {
			t.Fatal(err)
		}
		return g.Fence
	}
	first := hold()
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-1", Body: workerNextBody(t, "J-1")}, w); err != nil {
		t.Fatal(err)
	}
	// The worker's pass assigns J-1 to itself.
	t.Setenv("GRID_MEMBER", "worker")
	t.Setenv("GRID_ROLE", "worker")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", first))
	if code := h.run(path, "job", "next", "--json"); code != 0 {
		t.Fatalf("job next: %d %s", code, h.errb)
	}
	state, err := store.State()
	if err != nil || state.Jobs["J-1"].Assign == nil {
		t.Fatal(state, err)
	}
	// The agent's env, as the worker builds it.
	t.Setenv("GRID_LEASE", "J-1")
	t.Setenv("GRID_LEASE_EPOCH", fmt.Sprintf("0.%d", state.Jobs["J-1"].Assign.Fence))
	t.Setenv("GRID_TASK_EPOCH", fmt.Sprintf("0.%d", first))
	body, _ := json.Marshal(workerNextBody(t, "J-2"))
	post := func() int {
		return h.run(path, "job", "post", "--task", path, "--type", "job", "--body", string(body))
	}
	if code := post(); code != 0 {
		t.Fatalf("job post with the task epoch: %d %s", code, h.errb)
	}
	if code := h.run(path, "job", "tree", "--task", path, "--json"); code != 0 || !strings.Contains(h.out.String(), `"J-2"`) {
		t.Fatalf("job tree should show J-2: %d %s %s", code, h.out, h.errb)
	}
	// The worker restarts and its stream takes the task lease again: the agent's epoch is stale.
	if err := ts.ReleaseTask(w); err != nil {
		t.Fatal(err)
	}
	second := hold()
	if second <= first {
		t.Fatalf("the new fence %d should be higher than %d", second, first)
	}
	body, _ = json.Marshal(workerNextBody(t, "J-3"))
	if code := post(); code != 3 {
		t.Fatalf("a stale task epoch should exit 3, got %d (%s)", code, h.errb)
	}
	t.Setenv("GRID_TASK_EPOCH", fmt.Sprintf("0.%d", second))
	if code := post(); code != 0 {
		t.Fatalf("job post with the new task epoch: %d %s", code, h.errb)
	}
}
