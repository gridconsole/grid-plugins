package task

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// flakeCRRun is the body of the two exec'd-writer tests. Run as the parent it
// starts one child per action, in the order given, waiting gap between the
// Start calls, and checks the receipts against the history. Run as a child
// (the parent re-execs this same test by name) it only drives one writer.
func flakeCRRun(t *testing.T, actions []string, gap time.Duration) {
	t.Helper()
	if os.Getenv("GRID_TASK_WRITE_CHILD") == "1" {
		flakeCRChild(t)
		return
	}
	s, task := writeFixture(t)
	type child struct {
		cmd    *exec.Cmd
		out    bytes.Buffer
		stderr bytes.Buffer
	}
	children := make([]*child, len(actions))
	for i, action := range actions {
		if i > 0 {
			time.Sleep(gap)
		}
		c := &child{}
		children[i] = c
		c.cmd = exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
		c.cmd.Env = append(os.Environ(), "GRID_TASK_WRITE_CHILD=1", "GRID_TASK_WRITE_HOME="+s.Resolver.GridHome, "GRID_TASK_WRITE_ACTION="+action)
		c.cmd.Stdout, c.cmd.Stderr = &c.out, &c.stderr
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
	}
	receipts := map[string]int{}
	for _, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("%v: %s %s", err, c.out.String(), c.stderr.String())
		}
		for _, line := range strings.Split(c.out.String(), "\n") {
			if strings.HasPrefix(line, "{") {
				receipts[line]++
			}
		}
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) < 2 {
		t.Fatalf("history %v: %v", lines, err)
	}
	state := "queued"
	for _, line := range lines {
		raw := strings.TrimPrefix(line, "- ")
		if receipts[raw] == 0 {
			t.Fatalf("missing process receipt: %s", raw)
		}
		receipts[raw]--
		var e Event
		if err := json.Unmarshal([]byte(raw), &e); err != nil {
			t.Fatal(err)
		}
		if e.FromState != state {
			t.Fatalf("broken state chain: %s", raw)
		}
		state = e.ToState
	}
	for line, count := range receipts {
		if count != 0 {
			t.Fatalf("lost history line: %s", line)
		}
	}
	got, err := Read(task.Path)
	if err != nil || got.State != state {
		t.Fatalf("torn task: %+v %v", got, err)
	}
}

// flakeCRChild is one writer: it repeats its action and prints a receipt for
// each success. A move refused for the current state counts as an attempt.
func flakeCRChild(t *testing.T) {
	t.Helper()
	s := Store{Resolver: where.Resolver{GridHome: os.Getenv("GRID_TASK_WRITE_HOME")}}
	action := os.Getenv("GRID_TASK_WRITE_ACTION")
	for range 100 {
		event, err := s.Change(WriteOptions{ID: "T-01", Action: action})
		if err == nil {
			data, _ := json.Marshal(event)
			fmt.Println(string(data))
		} else if !strings.Contains(err.Error(), "current state") {
			t.Fatal(err)
		}
	}
}

// The close child starts two seconds after the reopen child. The delay is the
// stressor: on a loaded machine one process can start that late. The test must
// still see both writers take turns.
func TestFlakeCRLateClose(t *testing.T) {
	flakeCRRun(t, []string{"reopen", "close"}, 2*time.Second)
}
