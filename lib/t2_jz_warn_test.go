package lib

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// jzQwLine is the warning a fold prints for a task whose workflow.md cannot be read.
const jzQwLine = "warning: cannot read task withdrawal policy (workflow.md); using withdraw: []"

// jzQwUnreadable makes the task's workflow.md a folder. Reading it is a read
// error, which no file mode would give while the tests run as root.
func jzQwUnreadable(t *testing.T, h home) {
	t.Helper()
	path := filepath.Join(h.task, "workflow.md")
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

// jzQwPipeStderr points os.Stderr at a pipe for the rest of the test and
// returns what the process stderr received once the test calls the returned
// func (it closes the write end and reads the pipe dry; the lines are short).
func jzQwPipeStderr(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = w
	var once sync.Once
	var got string
	finish := func() string {
		once.Do(func() {
			os.Stderr = old
			w.Close()
			b, _ := io.ReadAll(r)
			r.Close()
			got = string(b)
		})
		return got
	}
	t.Cleanup(func() { finish() })
	return finish
}

// jzQwState is the call `feed state --json` in the task's folder with its own Stderr.
func jzQwState(h home, stderr *bytes.Buffer) (int, string) {
	var out bytes.Buffer
	code := Run(Call{Args: []string{"feed", "state", "--json"}, GridHome: h.grid, Dir: h.task, Environ: h.environ(), Stdout: &out, Stderr: stderr})
	return code, out.String()
}

// TestJzQwPolicyWarningGoesToCallStderr: the warning of a fold that cannot read
// workflow.md is the call's own Stderr, not the host process's.
func TestJzQwPolicyWarningGoesToCallStderr(t *testing.T) {
	h := newHome(t, "Unreadable policy")
	jzQwUnreadable(t, h)
	process := jzQwPipeStderr(t)
	var errb bytes.Buffer
	code, out := jzQwState(h, &errb)
	host := process()
	if code != 0 {
		t.Fatalf("feed state --json exit %d, want 0: stdout %q stderr %q", code, out, errb.String())
	}
	if got := strings.Count(errb.String(), jzQwLine); got != 1 {
		t.Errorf("call Stderr holds the warning %d times, want once: %q", got, errb.String())
	}
	if strings.Contains(host, "cannot read task withdrawal policy") {
		t.Errorf("the host's os.Stderr got the warning: %q", host)
	}
}

// TestJzQwTwoCallsEachKeepTheirWarning: two calls started together, each with
// its own Grid home and Stderr, each get the warning once and the host none.
func TestJzQwTwoCallsEachKeepTheirWarning(t *testing.T) {
	homes := []home{newHome(t, "Unreadable policy one"), newHome(t, "Unreadable policy two")}
	for _, h := range homes {
		jzQwUnreadable(t, h)
	}
	process := jzQwPipeStderr(t)
	errs := make([]bytes.Buffer, len(homes))
	codes := make([]int, len(homes))
	start := make(chan struct{})
	var ready, done sync.WaitGroup
	for i := range homes {
		ready.Add(1)
		done.Add(1)
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			codes[i], _ = jzQwState(homes[i], &errs[i])
		}()
	}
	ready.Wait()
	close(start)
	done.Wait()
	host := process()
	for i := range homes {
		if codes[i] != 0 {
			t.Errorf("call %d: exit %d, want 0: stderr %q", i, codes[i], errs[i].String())
		}
		if got := strings.Count(errs[i].String(), jzQwLine); got != 1 {
			t.Errorf("call %d: Stderr holds the warning %d times, want once: %q", i, got, errs[i].String())
		}
	}
	if strings.Contains(host, "cannot read task withdrawal policy") {
		t.Errorf("the host's os.Stderr got the warning: %q", host)
	}
}
