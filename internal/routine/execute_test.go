//go:build linux || darwin

package routine

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/proctree"
)

func TestProcessEnvironmentAndTimeout(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	t.Setenv("ROUTINE_TEST_SECRET", "not-in-child")
	program := filepath.Join(home, "grid")
	script := "#!/bin/sh\nprintf '%s|%s|%s|%s' \"${ROUTINE_TEST_SECRET-unset}\" \"$GRID_HOME\" \"$PWD\" \"$2\"\n"
	if err := os.WriteFile(program, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	p := ProcessExecutor{GridBinary: program, Home: home, Stdout: &out}
	s := Store{GridHome: home}
	if err := p.Execute(context.Background(), s, Entry{Run: `grid example "two words"`}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "unset|"+home+"|"+home+"|two words"; got != want {
		t.Fatalf("environment/argv: %q want %q", got, want)
	}
	if err := os.WriteFile(program, []byte("#!/bin/sh\nsleep 60 &\necho $! > \"$GRID_HOME/child.pid\"\nwait\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(home, "child.pid")
	// The timeout fires only once the descendant exists.
	p.withTimeout = func(ctx context.Context, _ time.Duration) (context.Context, context.CancelFunc) {
		ctx, cancel := context.WithCancel(ctx)
		go func() {
			tick := time.NewTicker(10 * time.Millisecond)
			defer tick.Stop()
			for {
				if _, ok := gtRoutineReadPid(pidFile); ok {
					cancel()
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-tick.C:
				}
			}
		}()
		return ctx, cancel
	}
	// The whole process group must be gone when the run returns, before cleanupProcess.
	p.runCommand = func(cmd *exec.Cmd) error {
		err := proctree.Run(cmd)
		pid, ok := gtRoutineReadPid(pidFile)
		if !ok {
			t.Error("descendant pid file missing when the run returned")
			return err
		}
		tick := time.NewTicker(10 * time.Millisecond)
		defer tick.Stop()
		bound := time.After(30 * time.Second)
		for gtRoutineAlive(pid) {
			select {
			case <-bound:
				t.Errorf("descendant still alive when the run returned: pid %d", pid)
				return err
			case <-tick.C:
			}
		}
		return err
	}
	if err := p.Execute(context.Background(), s, Entry{Run: "grid example"}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
}

// gtRoutineReadPid returns the pid once the file holds a whole line.
func gtRoutineReadPid(path string) (int, bool) {
	b, err := os.ReadFile(path)
	if err != nil || !bytes.HasSuffix(b, []byte("\n")) {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid, err == nil && pid > 0
}

// gtRoutineAlive reports a pid that still runs; a zombie counts as gone.
func gtRoutineAlive(pid int) bool {
	if syscall.Kill(pid, 0) != nil {
		return false
	}
	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return true
	}
	i := bytes.LastIndexByte(stat, ')')
	return !(i >= 0 && i+2 < len(stat) && stat[i+2] == 'Z')
}
