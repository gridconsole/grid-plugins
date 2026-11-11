//go:build unix

package workflow

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// A child that is stopped right after Start and continued only once the test
// is blocked in Wait stands for a busy machine: the run must still exit 0.
func TestIqWfStoppedChild(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "grid")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/grid")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	home := hardenHome(t)
	ctx := iqWfHangCtx(t)
	cmd := exec.CommandContext(ctx, binary, "workflow", "show", "--json", "--project", "id-alpha")
	cmd.Env = append(os.Environ(), "GRID_HOME="+home)
	stopped := false
	data, err := iqWfRun(t, ctx, cmd, func(p *os.Process, waiting <-chan struct{}) {
		if err := p.Signal(syscall.SIGSTOP); err != nil {
			t.Errorf("SIGSTOP: %v", err)
			return
		}
		stopped = true
		go func() {
			<-waiting
			if ctx.Err() != nil {
				return // the kill from the context ends the stopped child
			}
			if err := p.Signal(syscall.SIGCONT); err != nil {
				t.Errorf("SIGCONT: %v", err)
			}
		}()
	})
	if !stopped {
		t.Fatal("child was never stopped")
	}
	if err != nil {
		t.Fatalf("exit must be 0: %v\n%s", err, data)
	}
	var got Answer
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Errors) != 0 {
		t.Fatalf("healthy selection: %+v", got)
	}
	if len(got.Files) != 2 || len(got.Templates) != 2 || got.Files[0].Rev == nil {
		t.Fatalf("lost healthy files: %+v", got)
	}
}
