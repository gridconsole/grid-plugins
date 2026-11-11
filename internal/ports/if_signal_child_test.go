//go:build linux || darwin

package ports

import (
	"bufio"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// ifPrDeadline only bounds a wait; every wait that hits it fails by name.
const ifPrDeadline = 10 * time.Second

// ifPrChild is a shell in its own process group that answers every signal it
// handles with one line on stdout. Its stdin stays open so that the shell
// blocks in a builtin read, where a trapped signal runs its trap at once.
type ifPrChild struct {
	cmd    *exec.Cmd
	lines  chan string // closed on EOF of the child's stdout
	exited chan struct{}
}

const ifPrScript = `trap 'echo INT' INT
trap 'echo TERM' TERM
trap 'echo WINCH' WINCH
echo ready
while :; do read -r _; done`

// ifPrStartChild starts the child and returns once it printed "ready".
func ifPrStartChild(t *testing.T) *ifPrChild {
	t.Helper()
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", "-c", ifPrScript)
	cmd.Stdout = outW
	cmd.Stdin = inR
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	err = cmd.Start()
	outW.Close()
	inR.Close()
	if err != nil {
		outR.Close()
		inW.Close()
		t.Fatal(err)
	}
	c := &ifPrChild{cmd: cmd, lines: make(chan string), exited: make(chan struct{})}
	go func() { _ = cmd.Wait(); close(c.exited) }()
	go func() {
		defer close(c.lines)
		defer outR.Close()
		sc := bufio.NewScanner(outR)
		for sc.Scan() {
			c.lines <- sc.Text()
		}
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-c.exited
		inW.Close()
		for range c.lines {
		}
	})
	if line, ok := c.next(t, "ready line"); !ok || line != "ready" {
		t.Fatalf("child did not start: %q", line)
	}
	return c
}

// next returns the child's next stdout line; ok is false on EOF.
func (c *ifPrChild) next(t *testing.T, what string) (string, bool) {
	t.Helper()
	select {
	case line, ok := <-c.lines:
		return line, ok
	case <-time.After(ifPrDeadline):
		t.Fatalf("child never answered: %s", what)
		return "", false
	}
}

// assertNoSignal sends the child SIGWINCH itself and reads its lines until
// it answers "WINCH". Pending signals are delivered lowest number first and
// the shell runs traps in that order, so an INT (2) or TERM (15) sent earlier
// is printed before WINCH (28). A SIGKILL ends the child with EOF instead.
func (c *ifPrChild) assertNoSignal(t *testing.T) {
	t.Helper()
	if err := c.cmd.Process.Signal(syscall.SIGWINCH); err != nil {
		t.Fatal(err)
	}
	for {
		line, ok := c.next(t, "child never answered the barrier signal")
		if !ok {
			t.Fatal("gc signalled reused PID: EOF (SIGKILL)")
		}
		if line == "WINCH" {
			return
		}
		t.Fatalf("gc signalled reused PID: %s", line)
	}
}
