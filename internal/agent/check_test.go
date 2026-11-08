package agent

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func fakeEnv(t *testing.T, runner Runner) Env {
	t.Helper()
	home := t.TempDir()
	return Env{Home: home, Runner: runner, Lookup: func(name string) (string, error) { return filepath.Join(home, name), nil }}
}

func TestAgentCheckBothFound(t *testing.T) {
	var mu sync.Mutex
	calls := map[string][]string{}
	env := fakeEnv(t, func(ctx context.Context, argv []string) (string, int) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 4*time.Second || time.Until(deadline) > Wait {
			t.Errorf("deadline %v, %v", deadline, ok)
		}
		name := filepath.Base(argv[0])
		mu.Lock()
		calls[name] = append(calls[name], strings.Join(argv[1:], " "))
		mu.Unlock()
		if argv[1] == "--version" {
			if name == "claude" {
				return "2.1.236 (Claude Code)\n", 0
			}
			return "codex-cli v0.162.0+build\n", 0
		}
		return `{"authMethod":"oauth"}`, 0
	})
	got := Check(context.Background(), env)
	data, err := json.Marshal(got)
	want := `[{"agent":"claude","found":true,"path":"~/claude","version":"2.1.236","min":"2.1.236","signedIn":true},{"agent":"codex","found":true,"path":"~/codex","version":"0.162.0","min":"0.162.0","signedIn":true}]`
	if runtime.GOOS == "windows" {
		want = strings.ReplaceAll(want, "~/", `~\\`)
	}
	if err != nil || string(data) != want {
		t.Fatalf("%s, %v; want %s", data, err, want)
	}
	if !reflect.DeepEqual(calls, map[string][]string{"claude": {"--version", "auth status"}, "codex": {"--version", "login status"}}) {
		t.Fatalf("calls: %v", calls)
	}
}

func TestAgentCheckMissing(t *testing.T) {
	env := fakeEnv(t, func(context.Context, []string) (string, int) { t.Error("missing agent was run"); return "", -1 })
	env.Lookup = func(name string) (string, error) { return resolveAgent("", name) }
	for _, r := range Check(context.Background(), env) {
		if r.Found || r.Path != "" || r.Version != "" || r.SignedIn != nil || r.Fix == "" {
			t.Fatalf("%+v", r)
		}
	}
}

func TestAgentCheckOldVersion(t *testing.T) {
	env := fakeEnv(t, func(_ context.Context, argv []string) (string, int) {
		if !reflect.DeepEqual(argv[1:], []string{"--version"}) {
			t.Error("asked status for old agent")
		}
		return "0.1.0", 0
	})
	for _, r := range Check(context.Background(), env) {
		if !r.Found || r.Version != "0.1.0" || r.Min == "" || r.SignedIn != nil || r.Fix != fixLine(r.Agent, "update", false, runtime.GOOS) {
			t.Fatalf("%+v", r)
		}
	}
}

func TestAgentCheckNotSignedIn(t *testing.T) {
	for _, tc := range []struct {
		name, out string
		code      int
		want      *bool
	}{
		{"claude none", `{"authMethod":"none"}`, 0, boolPtr(false)},
		{"nonzero", `{"authMethod":"oauth"}`, 1, boolPtr(false)},
		{"unknown", `{}`, 0, nil},
		{"invalid json", `oops`, 0, nil},
		{"wrong type", `{"authMethod":true}`, 0, nil},
		{"case sensitive", `{"AuthMethod":"oauth"}`, 0, nil},
		{"null method", `{"authMethod":null}`, 0, nil},
		{"failed", "", -1, nil},
		{"empty method", `{"authMethod":""}`, 0, boolPtr(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := fakeEnv(t, func(_ context.Context, argv []string) (string, int) {
				if argv[1] == "--version" {
					return "3.0.0", 0
				}
				return tc.out, tc.code
			})
			reports := Check(context.Background(), env)
			for _, r := range reports {
				want := tc.want
				if r.Agent == "codex" && tc.code >= 0 {
					want = boolPtr(tc.code == 0)
				}
				if !reflect.DeepEqual(r.SignedIn, want) {
					t.Fatalf("%+v; want %v", r, want)
				}
				if (want == nil || !*want) != (r.Fix != "") {
					t.Fatalf("fix: %+v", r)
				}
			}
		})
	}
}
func boolPtr(b bool) *bool { return &b }

func TestAgentCheckOddVersionOutput(t *testing.T) {
	for _, tc := range []struct{ out, want string }{
		{"", ""}, {"warning\nClaude Code v2.1.236-beta+1\n", "2.1.236"},
		{"codex-cli 0.162.0", "0.162.0"}, {"2.bad 3.0.0", ""},
		{"v2", "2"}, {"2.1.", ""}, {"18446744073709551616.0.0", ""},
	} {
		if got := parseVersion(tc.out); got != tc.want {
			t.Errorf("parse %q = %q, want %q", tc.out, got, tc.want)
		}
	}
	for _, tc := range []struct {
		version, min string
		want         bool
	}{
		{"2.1.236", "2.1.236", true}, {"2.1.235", "2.1.236", false}, {"2.10", "2.9.99", true},
		{"2.1", "2.1.0", true}, {"2.1.0.1", "2.1", true}, {"bad", "2", false},
	} {
		if got := atLeast(tc.version, tc.min); got != tc.want {
			t.Errorf("atLeast(%q,%q) = %v", tc.version, tc.min, got)
		}
	}
	for _, tc := range []struct {
		out  string
		code int
	}{{"", 0}, {"bad", 0}, {"3.0.0", 1}, {"3.0.0", -1}} {
		env := fakeEnv(t, func(_ context.Context, argv []string) (string, int) {
			if argv[1] != "--version" {
				t.Error("status called with unknown version")
			}
			return tc.out, tc.code
		})
		for _, r := range Check(context.Background(), env) {
			if !r.Found || r.Version != "" || r.SignedIn != nil || r.Fix == "" {
				t.Fatalf("%+v", r)
			}
		}
	}
}

func TestAgentCheckTimeout(t *testing.T) {
	// Both agents must reach their blocking probe concurrently. Claude blocks on
	// version; Codex blocks on status after receiving a fresh five-second budget.
	entered := make(chan struct{}, 2)
	env := fakeEnv(t, func(ctx context.Context, argv []string) (string, int) {
		if filepath.Base(argv[0]) == "codex" && argv[1] == "--version" {
			return "0.162.0", 0
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) < 4*time.Second || time.Until(deadline) > Wait {
			t.Errorf("wrong probe deadline: %v", deadline)
		}
		entered <- struct{}{}
		<-ctx.Done()
		return "3.0.0", 0 // A late answer must not be accepted.
	})
	done := make(chan []Report, 1)
	go func() { done <- Check(context.Background(), env) }()
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(2 * time.Second):
			t.Fatal("probes did not run concurrently")
		}
	}
	select {
	case reports := <-done:
		if reports[0].Version != "" || reports[1].Version != "0.162.0" {
			t.Fatalf("versions: %+v", reports)
		}
		for _, r := range reports {
			if !r.Found || r.SignedIn != nil || r.Fix == "" {
				t.Fatalf("%+v", r)
			}
		}
	case <-time.After(Wait + time.Second):
		t.Fatal("probe timeout did not stop check")
	}
}

func TestAgentCheckCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	env := fakeEnv(t, func(context.Context, []string) (string, int) {
		t.Error("cancelled check started process")
		return "", 0
	})
	for _, r := range Check(ctx, env) {
		if r.Fix == "" || r.Version != "" {
			t.Fatalf("%+v", r)
		}
	}
}

func TestFixLines(t *testing.T) {
	for _, tc := range []struct {
		name, fix, platform string
		brew                bool
		want                string
	}{
		{"claude", "install", "darwin", false, "brew install --cask claude-code"},
		{"codex", "install", "darwin", false, "brew install --cask codex"},
		{"claude", "install", "windows", false, "irm https://claude.ai/install.ps1 | iex"},
		{"codex", "update", "windows", false, `powershell -ExecutionPolicy ByPass -c "irm https://chatgpt.com/codex/install.ps1 | iex"`},
		{"claude", "update", "darwin", true, "brew upgrade --cask claude-code"},
		{"codex", "update", "darwin", true, "brew upgrade --cask codex"},
		{"claude", "update", "linux", false, "claude update"},
		{"codex", "update", "darwin", false, "curl -fsSL https://chatgpt.com/codex/install.sh | sh"},
		{"claude", "signin", "linux", false, "claude, then /login"},
		{"codex", "signin", "linux", false, "codex login"},
	} {
		if got := fixLine(tc.name, tc.fix, tc.brew, tc.platform); got != tc.want {
			t.Errorf("%+v: %q", tc, got)
		}
	}
}

func TestBrewSymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "Caskroom", "claude-code", "claude")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "claude")
	if err := os.Symlink(target, link); err != nil {
		if runtime.GOOS == "windows" {
			t.Skip(err)
		}
		t.Fatal(err)
	}
	if !fromBrew(link) || fromBrew(filepath.Join(root, "absent")) {
		t.Fatal("cask detection")
	}
	env := fakeEnv(t, func(context.Context, []string) (string, int) { return "0.1.0", 0 })
	env.Lookup = func(string) (string, error) { return link, nil }
	for _, r := range Check(context.Background(), env) {
		if !strings.HasPrefix(r.Fix, "brew upgrade --cask ") {
			t.Fatalf("%+v", r)
		}
	}
}

func TestResolveStandardOrder(t *testing.T) {
	home := t.TempDir()
	dirs := []string{filepath.Join(home, ".local", "bin"), t.TempDir(), t.TempDir()}
	launch := t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		t.Setenv("PATHEXT", ".EXE;.CMD")
		name += ".exe"
	}
	for _, dir := range append(append([]string{}, dirs...), launch) {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if got, err := (Env{Home: home, PATH: launch}).resolve("claude"); err != nil || got != filepath.Join(dirs[0], name) {
		t.Fatalf("home precedence: %q %v", got, err)
	}
	for _, dir := range append(append([]string{}, dirs...), launch) {
		got, err := resolveIn(dirs, launch, "claude")
		if err != nil || got != filepath.Join(dir, name) {
			t.Fatalf("lookup order: %q, %v", got, err)
		}
		if err := os.Remove(got); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := resolveIn(dirs, launch, "claude"); err != exec.ErrNotFound {
		t.Fatalf("%v", err)
	}
}
