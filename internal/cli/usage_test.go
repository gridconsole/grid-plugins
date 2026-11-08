package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/session"
)

func TestUsageCommandFoldsLocalRecords(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	cwd := filepath.Join(home, "workspace")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	s := session.Store{TaskDir: home}
	if err := s.RecordUsage("session-demo", "tool-demo", session.Usage{Input: 7, Output: 3}); err != nil {
		t.Fatal(err)
	}
	var out, log bytes.Buffer
	code := Run([]string{"usage", "--json"}, Env{Stdout: &out, Stderr: &log, Home: home, GridHome: home, Cwd: cwd, Ceiling: home, Getenv: func(string) string { return "" }})
	if code != 0 || log.Len() != 0 || !strings.Contains(out.String(), `"input_tokens":7`) {
		t.Fatalf("%d %s %s", code, out.String(), log.String())
	}
}
