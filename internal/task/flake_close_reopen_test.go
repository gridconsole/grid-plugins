package task

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/where"
)

const (
	// flakeCRMoves is how many successful moves each child must make.
	flakeCRMoves = 25
	// flakeCRDeadline bounds a child that cannot finish; it is not what makes
	// the two children overlap.
	flakeCRDeadline = 60 * time.Second
)

// koTfRefused is the one line, and not a receipt, that a child prints on stdout
// the first time a move is refused for the current state. The parent that
// waits for it knows the child is running and has met the other writer's turn.
const koTfRefused = "koTf refused"

// koTfRead reads a child's stdout to its end and keeps all of it in out. It
// closes refused at the first line that is koTfRefused, and done at the end of
// the stream. Read out only after done is closed.
func koTfRead(r io.Reader, out *bytes.Buffer, refused, done chan struct{}) {
	defer close(done)
	br := bufio.NewReader(r)
	seen := false
	for {
		line, err := br.ReadString('\n')
		out.WriteString(line)
		if !seen && strings.TrimSpace(line) == koTfRefused {
			seen = true
			close(refused)
		}
		if err != nil {
			return
		}
	}
}

// flakeCRRun is the body of the two exec'd-writer tests. Run as the parent it
// starts one child per action, in the order given, and checks the receipts
// against the history. With waitRefused each child starts only after the child
// before it has printed koTfRefused, its first refused move; without it all
// children start at once. Run as a child (the parent re-execs this same test by
// name) it only drives one writer.
func flakeCRRun(t *testing.T, actions []string, waitRefused bool) {
	t.Helper()
	if os.Getenv("GRID_TASK_WRITE_CHILD") == "1" {
		flakeCRChild(t)
		return
	}
	s, task := writeFixture(t)
	type child struct {
		cmd     *exec.Cmd
		out     bytes.Buffer
		stderr  bytes.Buffer
		refused chan struct{}
		done    chan struct{}
	}
	children := make([]*child, len(actions))
	for i, action := range actions {
		if i > 0 && waitRefused {
			// The event, not a delay: the child before is up and has been
			// refused. It cannot end first, since its moves need this child.
			prev := children[i-1]
			select {
			case <-prev.refused:
			case <-prev.done:
				t.Fatalf("%s child ended before it was refused a move: %s %s", actions[i-1], prev.out.String(), prev.stderr.String())
			}
		}
		c := &child{refused: make(chan struct{}), done: make(chan struct{})}
		children[i] = c
		c.cmd = exec.Command(os.Args[0], "-test.run=^"+t.Name()+"$")
		c.cmd.Env = append(os.Environ(), "GRID_TASK_WRITE_CHILD=1", "GRID_TASK_WRITE_HOME="+s.Resolver.GridHome, "GRID_TASK_WRITE_ACTION="+action)
		c.cmd.Stderr = &c.stderr
		stdout, err := c.cmd.StdoutPipe()
		if err != nil {
			t.Fatal(err)
		}
		if err := c.cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = c.cmd.Process.Kill() })
		go koTfRead(stdout, &c.out, c.refused, c.done)
	}
	receipts := map[string]int{}
	for i, c := range children {
		// Wait must not run before the reads of the pipe are over.
		<-c.done
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("%v: %s %s", err, c.out.String(), c.stderr.String())
		}
		printed := 0
		for _, line := range strings.Split(c.out.String(), "\n") {
			if strings.HasPrefix(line, "{") {
				receipts[line]++
				printed++
			}
		}
		if printed != flakeCRMoves {
			t.Fatalf("%s child printed %d receipts, want %d: %s", actions[i], printed, flakeCRMoves, c.out.String())
		}
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != len(actions)*flakeCRMoves {
		t.Fatalf("history has %d lines, want %d: %v: %v", len(lines), len(actions)*flakeCRMoves, lines, err)
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
	// Close and reopen alternate from queued and the same number of each is
	// made, so the reopen child makes the last move.
	if state != "queued" {
		t.Fatalf("final state %q, want queued", state)
	}
}

// flakeCRChild is one writer: it repeats its action until it has made
// flakeCRMoves successful moves and prints a receipt for each. A move refused
// for the current state is retried at once and is not counted. Close is only
// allowed from queued and reopen only from closed, so the two moves strictly
// alternate: a writer cannot finish before the other has made its moves too,
// whatever the start order or delay. The deadline only bounds a failure.
func flakeCRChild(t *testing.T) {
	t.Helper()
	s := Store{Resolver: where.Resolver{GridHome: os.Getenv("GRID_TASK_WRITE_HOME")}}
	action := os.Getenv("GRID_TASK_WRITE_ACTION")
	deadline := time.Now().Add(flakeCRDeadline)
	moves, refused := 0, 0
	for moves < flakeCRMoves {
		if time.Now().After(deadline) {
			t.Fatalf("%s: deadline %v: %d of %d moves made, %d refused", action, flakeCRDeadline, moves, flakeCRMoves, refused)
		}
		event, err := s.Change(WriteOptions{ID: "T-01", Action: action})
		switch {
		case err == nil:
			data, _ := json.Marshal(event)
			fmt.Println(string(data))
			moves++
		case strings.Contains(err.Error(), "current state"):
			if refused == 0 {
				fmt.Println(koTfRefused)
			}
			refused++
		default:
			t.Fatal(err)
		}
	}
}

// The close child starts only after the reopen child has been refused its first
// move: the state starts queued, so reopen is refused at once, and it keeps
// retrying while close is not yet running. The late start is the stressor: on a
// loaded machine one process can start that late. The test must still see both
// writers take turns.
func TestFlakeCRLateClose(t *testing.T) {
	flakeCRRun(t, []string{"reopen", "close"}, true)
}
