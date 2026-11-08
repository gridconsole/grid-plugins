//go:build linux || darwin

package routine

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
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
	if err := os.WriteFile(program, []byte("#!/bin/sh\nexec sleep 5\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	p.Timeout = 20 * time.Millisecond
	if err := p.Execute(context.Background(), s, Entry{Run: "grid example"}); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout: %v", err)
	}
}
