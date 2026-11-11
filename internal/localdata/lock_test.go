package localdata

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/testenv"
)

// koLkHeld is the line the helper child writes on stdout once it holds the lock.
// The parent reads it through a pipe and goes on when it arrives.
const koLkHeld = "KO-LOCK-HELD"

// TestLockHelperProcess is the body of the child process the tests below start;
// it does nothing in a normal run.
func TestLockHelperProcess(t *testing.T) {
	if os.Getenv("LOCALDATA_LOCK_HELPER") == "" {
		t.Skip("helper process only")
	}
	if _, err := Lock(os.Getenv("LOCALDATA_HOME"), ".test.lock"); err != nil {
		t.Fatal(err)
	}
	fmt.Println(koLkHeld)
	time.Sleep(time.Hour) // hold the lock until the parent kills this process
}

// koLkPipe is the stdout and stderr of the helper child, read line by line. It
// keeps the text for failure messages and closes seen when the line the parent
// waits for arrives.
type koLkPipe struct {
	want string
	seen chan struct{} // closed at the first line equal to want
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
	p := &koLkPipe{want: want, seen: make(chan struct{})}
	go p.read(pr)
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = pr.Close()
	})
	return p
}

func (p *koLkPipe) read(r *os.File) {
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

// lockWithin fails the test when Lock does not return before the deadline.
func lockWithin(t *testing.T, home string) func() {
	t.Helper()
	type result struct {
		unlock func()
		err    error
	}
	done := make(chan result, 1)
	go func() {
		unlock, err := Lock(home, ".test.lock")
		done <- result{unlock, err}
	}()
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
		return r.unlock
	case <-time.After(10 * time.Second):
		t.Fatal("Lock is still waiting; a dead writer wedged it")
		return nil
	}
}

func TestKilledWriterDoesNotBlockTheNextWrite(t *testing.T) {
	home := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
	cmd.Env = append(testenv.Temp(t, os.Environ()), "LOCALDATA_LOCK_HELPER=1", "LOCALDATA_HOME="+home)
	holder := koLkStart(t, cmd, koLkHeld)
	koLkWait(t, "the holder to take the lock", holder)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if _, err := os.Stat(filepath.Join(home, ".test.lock")); err != nil {
		t.Fatal("the killed holder should leave its lock file behind")
	}
	lockWithin(t, home)()
}

func TestLeftoverLockFileDoesNotBlock(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".test.lock"), []byte("pid 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lockWithin(t, home)()
}

func TestLockSerializesWriters(t *testing.T) {
	home := t.TempDir()
	const writers = 8
	// fslock calls this hook when a writer finds the lock held and is about to
	// wait. The channel has a slot for every writer, so the hook never blocks.
	blocked := make(chan struct{}, writers)
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if strings.HasSuffix(path, ".test.lock") {
			select {
			case blocked <- struct{}{}:
			default:
			}
		}
	}))
	var inside atomic.Int32
	var first atomic.Bool
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Go(func() {
			unlock, err := Lock(home, ".test.lock")
			if err != nil {
				t.Error(err)
				return
			}
			defer unlock()
			if inside.Add(1) != 1 {
				t.Error("two writers hold the lock at once")
			}
			if first.CompareAndSwap(false, true) {
				// The first writer to get the lock stays inside until another one
				// is waiting for it, so the writers certainly overlap. The others
				// leave at once. The 10 s is only the failure bound.
				select {
				case <-blocked:
				case <-time.After(10 * time.Second):
					t.Error("no other writer ever waited for the lock")
				}
			}
			inside.Add(-1)
		})
	}
	wg.Wait()
}

func TestLockRefusesSymlink(t *testing.T) {
	home := t.TempDir()
	if err := os.Symlink(filepath.Join(t.TempDir(), "elsewhere"), filepath.Join(home, ".test.lock")); err != nil {
		t.Skip("symlinks not available")
	}
	if _, err := Lock(home, ".test.lock"); err == nil {
		t.Fatal("locked through a symlink")
	}
}
