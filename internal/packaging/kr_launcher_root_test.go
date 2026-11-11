package packaging

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// krLauncherRun runs the embedded launcher template with $0 inside a plugin
// folder that does not exist, as plugin-dist is between two renames.
func krLauncherRun(t *testing.T, args ...string) (stdout, stderr string, exit int) {
	t.Helper()
	gone := filepath.Join(t.TempDir(), "gone", "bin", "grid")
	cmd := exec.Command("sh", append([]string{"-c", string(launcher), gone}, args...)...)
	cmd.Dir = t.TempDir()
	cmd.Stdin = strings.NewReader(`{"hook_event_name":"PostToolUse"}` + "\n")
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		exit = ee.ExitCode()
	default:
		t.Fatal(err)
	}
	return out.String(), errOut.String(), exit
}

func TestKrLauncherMissingRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release launcher supports macOS and Linux")
	}
	const note = "grid: plugin root not found; hook skipped\n"
	for _, args := range [][]string{{"hook", "posttooluse"}, {"hook"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, exit := krLauncherRun(t, args...)
			if exit != 0 || stdout != "" || stderr != note {
				t.Fatalf("exit=%d stdout=%q stderr=%q; want exit 0, empty stdout, stderr %q", exit, stdout, stderr, note)
			}
		})
	}
	t.Run("version", func(t *testing.T) {
		stdout, stderr, exit := krLauncherRun(t, "version")
		if exit == 0 || stderr == "" || stdout != "" {
			t.Fatalf("exit=%d stdout=%q stderr=%q; want non-zero exit and a stderr line", exit, stdout, stderr)
		}
	})
}
