package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// A new task is queued and has J-1 posted. Once grid worker next has printed the spawn line of
// J-1, task.md says running and history.md holds one receipt by the worker's member and role.
// The worker runs as a child process, driven as TestWorkerNextStreamsSpawnLinesUntilSIGINT does.
func TestWorkerNextMovesQueuedTaskToRunningOnFirstAssign(t *testing.T) {
	h := newTaskHarness(t)
	h.write(filepath.Join(h.gridHome, "VERSION"), "1\n")
	h.write(filepath.Join(h.gridHome, "machine"), feedMachine+"\n")
	path := newTask(t, h)
	before, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil || !strings.Contains(string(before), "\nstate: queued\n") {
		t.Fatalf("a new task is not queued: %s %v", before, err)
	}
	pid, start := taskLeaseIdentity(t)

	cmd := exec.Command(os.Args[0], "-test.run=^TestWorkerNextProcess$")
	cmd.Env = append(os.Environ(), "GRID_TEST_WORKER_NEXT=1", "GRID_TEST_HOME="+h.gridHome,
		"GRID_TEST_PID="+strconv.Itoa(pid), "GRID_TEST_PID_START="+start)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	lines := make(chan string)
	go func() {
		sc := bufio.NewScanner(stdout)
		for sc.Scan() {
			lines <- sc.Text()
		}
		close(lines)
	}()
	select {
	case l, ok := <-lines:
		var line map[string]map[string]any
		if !ok || json.Unmarshal([]byte(l), &line) != nil || line["spawn"]["id"] != "J-1" {
			t.Fatalf("want the spawn line of J-1, got %q (open %v)\nstderr: %s", l, ok, &stderr)
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("no spawn line within 30s\nstderr: %s", &stderr)
	}

	// The move is written before the spawn line is printed, so it is on disk now.
	after, err := os.ReadFile(filepath.Join(path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	if want := strings.Replace(string(before), "\nstate: queued\n", "\nstate: running\n", 1); string(after) != want {
		t.Fatalf("task.md after the first spawn:\n%s\nwant:\n%s\nstderr: %s", after, want, &stderr)
	}
	history, err := os.ReadFile(filepath.Join(path, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	var receipt []map[string]any
	for _, l := range strings.Split(strings.TrimSpace(string(history)), "\n") {
		var r map[string]any
		if json.Unmarshal([]byte(strings.TrimPrefix(l, "- ")), &r) == nil && r["to_state"] == "running" {
			receipt = append(receipt, r)
		}
	}
	if len(receipt) != 1 {
		t.Fatalf("want one running receipt, got %d in:\n%s", len(receipt), history)
	}
	who, _ := receipt[0]["who"].(map[string]any)
	if receipt[0]["from_state"] != "queued" || receipt[0]["what"] != "status" || receipt[0]["note"] != "J-1 assigned" ||
		who["id"] != "worker" || who["role"] != "cli" {
		t.Fatalf("receipt: %v", receipt[0])
	}
}
