package testenv

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestChildTempCleanup(t *testing.T) {
	if os.Getenv("GRID_TESTENV_CHILD") == "1" {
		// Simulate a hook or abruptly exiting helper that leaves temp files behind.
		path := filepath.Join(os.TempDir(), "grid-cleanup-test-child")
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "leftover"), []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
		os.Exit(17)
	}
	var dir string
	t.Run("child exits without cleanup", func(t *testing.T) {
		cmd := exec.Command(os.Args[0], "-test.run=^TestChildTempCleanup$")
		cmd.Env = Temp(t, append(os.Environ(), "GRID_TESTENV_CHILD=1", "TMPDIR=unused", "TMP=unused", "TEMP=unused"))
		dir = strings.TrimPrefix(cmd.Env[len(cmd.Env)-1], "TEMP=")
		out, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 17 {
			t.Fatalf("child: %v\n%s", err, out)
		}
		if _, err := os.Stat(filepath.Join(dir, "grid-cleanup-test-child", "leftover")); err != nil {
			t.Fatalf("child did not use its test-owned temp directory: %v", err)
		}
	})
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("child temp directory survived parent cleanup: %v", err)
	}
}
