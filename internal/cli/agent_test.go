package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/agent"
)

func TestAgentCheckJSON(t *testing.T) {
	home := t.TempDir()
	var out, errs bytes.Buffer
	env := Env{Home: home, Stdout: &out, Stderr: &errs}
	probe := agent.Env{Home: home, Lookup: func(name string) (string, error) { return filepath.Join(home, name), nil }, Runner: func(_ context.Context, argv []string) (string, int) {
		if argv[1] == "--version" {
			return "3.0.0", 0
		}
		return `{"authMethod":"oauth"}`, 0
	}}
	if code := runAgentCheck([]string{"--json"}, env, probe); code != 0 || errs.Len() != 0 {
		t.Fatalf("code=%d stderr=%s", code, &errs)
	}
	want := `{"agents":[{"agent":"claude","found":true,"path":"~/claude","version":"3.0.0","min":"2.1.236","signedIn":true},{"agent":"codex","found":true,"path":"~/codex","version":"3.0.0","min":"0.162.0","signedIn":true}]}` + "\n"
	if runtime.GOOS == "windows" {
		want = strings.ReplaceAll(want, "~/", `~\\`)
	}
	if out.String() != want {
		t.Fatalf("JSON: %s; want %s", &out, want)
	}
	out.Reset()
	if code := runAgentCheck(nil, env, probe); code != 0 {
		t.Fatalf("code=%d", code)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(lines[0], "claude found=true") || !strings.Contains(lines[1], "signedIn=true") {
		t.Fatalf("plain: %s", &out)
	}
	probe.Lookup = func(string) (string, error) { return "", os.ErrNotExist }
	out.Reset()
	if code := runAgentCheck([]string{"--json"}, env, probe); code != 0 || strings.Contains(out.String(), "signedIn") || strings.Contains(out.String(), `"path"`) || !strings.Contains(out.String(), `"found":false`) {
		t.Fatalf("missing: %d %s", code, &out)
	}
}

func TestAgentCheckDispatch(t *testing.T) {
	// Empty executables resolve but cannot start, exercising the registered
	// command without invoking any real agent on the host.
	home := t.TempDir()
	dir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"claude", "codex"} {
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE;.CMD")
	}
	var out, errs bytes.Buffer
	env := Env{Home: home, Stdout: &out, Stderr: &errs, Getenv: func(string) string { return "" }}
	if code := Run([]string{"agent", "check", "--json"}, env); code != 0 {
		t.Fatalf("code %d: %s", code, &errs)
	}
	var result struct {
		Agents []agent.Report `json:"agents"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || len(result.Agents) != 2 {
		t.Fatalf("%s: %v", &out, err)
	}
	for _, r := range result.Agents {
		if !r.Found || r.Version != "" || r.Fix == "" {
			t.Fatalf("%+v", r)
		}
	}
	for _, args := range [][]string{{"agent", "check", "--bad"}, {"agent", "check", "extra"}, {"agent", "check", "--json=true", "extra"}} {
		out.Reset()
		errs.Reset()
		if code := Run(args, env); code != 2 || out.Len() != 0 || errs.Len() == 0 {
			t.Fatalf("%v: %d out=%s err=%s", args, code, &out, &errs)
		}
	}
	out.Reset()
	errs.Reset()
	if code := Run([]string{"agent", "check", "--help"}, env); code != 0 {
		t.Fatalf("help: %d", code)
	}
	out.Reset()
	errs.Reset()
	if code := Run([]string{"agent", "--help"}, env); code != 0 || !strings.Contains(out.String(), "agent check") || !strings.Contains(out.String(), "agent set") {
		t.Fatalf("help: %d %s", code, &out)
	}
}
