package brain

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/testenv"
)

const helperEnv = "BRAIN_LOCK_HELPER"

// The lines a helper child writes on stdout at its event. The parent reads them
// through a pipe and goes on when the line it waits for arrives.
const (
	koLkBlocked = "KO-LOCK-BLOCKED" // the child is about to wait for a lock held elsewhere
	koLkHeld    = "KO-LOCK-HELD"    // the child holds the lock
)

// koLkSay is the child's side of the event: one fixed line on stdout.
func koLkSay(line string) { fmt.Println(line) }

// TestLockHelperProcess is the body of the child processes the tests below
// start; it does nothing in a normal run.
func TestLockHelperProcess(t *testing.T) {
	mode := os.Getenv(helperEnv)
	if mode == "" {
		t.Skip("helper process only")
	}
	s := Store{GridHome: os.Getenv("BRAIN_HOME"), Project: "example"}
	switch mode {
	case "propose":
		n := sample()
		n.Name = os.Getenv("BRAIN_NAME")
		// The parent holds the lock, so Propose has to wait for it. fslock calls
		// the hook when it is about to wait, and the line tells the parent so.
		fslock.OnBlockForTest(func(path string) {
			if strings.HasSuffix(path, lockName) {
				koLkSay(koLkBlocked)
			}
		})
		if _, err := s.Propose(n, n.Origin); err != nil {
			t.Fatal(err)
		}
	case "hold":
		// Hold the lock until the parent kills this process.
		root, err := s.scope("project", true)
		if err != nil {
			t.Fatal(err)
		}
		_ = locked(root, func() error {
			koLkSay(koLkHeld)
			time.Sleep(time.Hour)
			return nil
		})
	}
}

// koLkPipe is the stdout and stderr of a helper child, read line by line. It
// keeps the text for failure messages and closes seen when the line the parent
// waits for arrives.
type koLkPipe struct {
	want string
	seen chan struct{} // closed at the first line equal to want
	done chan struct{} // closed when the child's output has ended
	once sync.Once
	mu   sync.Mutex
	buf  strings.Builder
}

// koLkStart starts cmd with its output on a pipe that a goroutine reads, and
// kills the child when the test ends.
func koLkStart(t *testing.T, cmd *exec.Cmd, want string) *koLkPipe {
	t.Helper()
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = pw, pw
	err = cmd.Start()
	_ = pw.Close() // the child has its own copy; this one would keep the pipe open
	if err != nil {
		_ = pr.Close()
		t.Fatal(err)
	}
	p := &koLkPipe{want: want, seen: make(chan struct{}), done: make(chan struct{})}
	go p.read(pr)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = pr.Close()
	})
	return p
}

func (p *koLkPipe) read(r *os.File) {
	defer close(p.done)
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		p.mu.Lock()
		p.buf.WriteString(line)
		p.mu.Unlock()
		if strings.TrimRight(line, "\r\n") == p.want {
			p.once.Do(func() { close(p.seen) })
		}
		if err != nil {
			return
		}
	}
}

func (p *koLkPipe) text() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.String()
}

// output is everything the child wrote, for a failure message after the child
// has exited; the deadline only bounds that read of the pipe.
func (p *koLkPipe) output() string {
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
	}
	return p.text()
}

// koLkWait goes on when the child has written its line. The 10 s is only the
// failure bound; the message carries what the child wrote so far.
func koLkWait(t *testing.T, what string, p *koLkPipe) {
	t.Helper()
	select {
	case <-p.seen:
	case <-time.After(10 * time.Second):
		t.Fatalf("timed out waiting for %s; the child wrote:\n%s", what, p.text())
	}
}

type child struct {
	cmd  *exec.Cmd
	pipe *koLkPipe
}

// start runs the helper in mode; want is the line it writes at its event.
func start(t *testing.T, s Store, mode, name, want string) child {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(testenv.Temp(t, os.Environ()), helperEnv+"="+mode, "BRAIN_HOME="+s.GridHome, "BRAIN_NAME="+name)
	return child{cmd, koLkStart(t, cmd, want)}
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestTwoProcessesProposeAtOnce(t *testing.T) {
	s := testStore(t)
	root, err := s.scope("project", true)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	names := []string{"first", "second"}
	var children []child
	// Both writers start while this process holds the lock, so each has to wait
	// for it and then for the other. Each says so when the lock is about to make
	// it wait, and this process lets go only after both have.
	err = locked(root, func() error {
		for _, name := range names {
			children = append(children, start(t, s, "propose", name, koLkBlocked))
		}
		for i, c := range children {
			koLkWait(t, "writer "+names[i]+" to wait for the lock", c.pipe)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range children {
		if err := c.cmd.Wait(); err != nil {
			t.Fatalf("writer %s failed: %v\n%s", names[i], err, c.pipe.output())
		}
		path := filepath.Join(s.GridHome, "projects", "example", "brain-proposals", names[i]+".md")
		if !exists(path) {
			t.Fatalf("proposal %s was lost", names[i])
		}
	}
}

func TestKilledWriterLeavesNoStaleLock(t *testing.T) {
	s := testStore(t)
	holder := start(t, s, "hold", "", koLkHeld)
	koLkWait(t, "the holder to take the lock", holder.pipe)
	if err := holder.cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = holder.cmd.Wait()
	if !exists(filepath.Join(s.GridHome, "projects", "example", lockName)) {
		t.Fatal("the killed holder should leave its lock file behind")
	}
	n := sample()
	n.Name = "after-kill"
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatalf("lock left by a killed writer wedged the brain: %v", err)
	}
}

func TestLeftoverLockFileDoesNotBlock(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/"+lockName, "pid 1\n")
	n := sample()
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatalf("leftover lock file blocked a proposal: %v", err)
	}
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatal(err)
	}
}
