package persongate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

type fakeProcs struct {
	tree map[int]Process
	err  error
}

func (f fakeProcs) Snapshot() (map[int]Process, error) { return f.tree, f.err }
func chain(names ...string) map[int]Process {
	tree := map[int]Process{}
	for i, n := range names {
		pid := 100 + i
		parent := pid + 1
		if i == len(names)-2 && (names[i+1] == "launchd" || names[i+1] == "systemd") {
			parent = 1
		}
		if i == len(names)-1 && (n == "launchd" || n == "systemd") {
			pid, parent = 1, 0
		}
		tree[pid] = Process{PID: pid, PPID: parent, Exe: n, Args: []string{n}}
	}
	return tree
}
func request(t *testing.T, names ...string) Request {
	t.Helper()
	home := t.TempDir()
	return Request{Verb: []string{"check"}, Argv: []string{"check"}, PID: 100, Getenv: func(string) string { return "" }, Procs: fakeProcs{tree: chain(names...)}, PersonDir: func() (string, error) { return filepath.Join(home, ".grid-person"), nil }, Clock: func() time.Time { return time.Unix(42, 0) }}
}
func TestProcessGate(t *testing.T) {
	for _, tc := range []struct {
		name        string
		names       []string
		reason, via string
		env         map[string]string
		args        []string
	}{
		{"env_i", []string{"grid", "zsh", "claude"}, "agent_ancestor", "", nil, nil},
		{"renamed", []string{"g", "zsh", "claude"}, "agent_ancestor", "", nil, nil},
		{"node_claude", []string{"grid", "bash", "node"}, "agent_ancestor", "", nil, []string{"node", "claude-code/cli.js"}},
		{"bun_codex", []string{"grid", "bash", "bun"}, "agent_ancestor", "", nil, []string{"bun", "@openai/codex/bin.js"}},
		{"native_claude", []string{"grid", "bash", "claude/versions/2.1.319", "Terminal"}, "agent_ancestor", "", nil, nil},
		{"codex", []string{"grid", "bash", "Codex.EXE"}, "agent_ancestor", "", nil, nil},
		{"codex_variant", []string{"grid", "bash", "codex-linux"}, "agent_ancestor", "", nil, nil},
		{"worker", []string{"grid", "bash", "grid-worker"}, "agent_ancestor", "", nil, nil},
		{"agent_above_terminal", []string{"grid", "tmux", "claude"}, "agent_ancestor", "", nil, nil},
		{"detached_shell", []string{"grid", "bash", "launchd"}, "detached", "", nil, nil},
		{"detached_make", []string{"grid", "make", "launchd"}, "detached", "", nil, nil},
		{"detached_cron", []string{"grid", "sh", "cron"}, "detached", "", nil, nil},
		{"windows_gone", []string{"grid.exe", "cmd.exe"}, "detached", "", nil, nil},
		{"child_session", []string{"grid", "zsh", "Terminal"}, "agent_env", "", map[string]string{"CLAUDE_CODE_CHILD_SESSION": "1"}, nil},
		{"terminal", []string{"grid", "zsh", "login", "Terminal", "launchd"}, "", "terminal", nil, nil},
		{"ssh_session", []string{"grid", "bash", "sshd-session", "sshd", "systemd"}, "", "terminal", nil, nil},
		{"ssh_session_only", []string{"grid", "bash", "sshd-session", "systemd"}, "", "terminal", nil, nil},
		{"ssh_session_agent_env", []string{"grid", "bash", "sshd-session", "sshd", "systemd"}, "agent_env", "", map[string]string{"GRID_AGENT": "builder"}, nil},
		{"agent_above_ssh_session", []string{"grid", "bash", "sshd-session", "codex", "systemd"}, "agent_ancestor", "", nil, nil},
		{"ide", []string{"grid", "zsh", "Code Helper"}, "", "terminal", map[string]string{"CLAUDECODE": "1"}, nil},
		// T1 gap: a person-owned tmux server can launch a detached agent request.
		{"tmux_T1_gap", []string{"grid", "zsh", "tmux"}, "", "terminal", nil, nil},
		{"desktop", []string{"grid", "grid-desktop", "launchd"}, "", "app", nil, nil},
		{"server", []string{"grid", "grid-server", "systemd"}, "", "app", nil, nil},
		{"plain_node", []string{"grid", "node", "Terminal"}, "", "terminal", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(t, tc.names...)
			if tc.name == "renamed" {
				tree := r.Procs.(fakeProcs).tree
				self := tree[100]
				self.Exe = "grid"
				self.Args = []string{"g"}
				tree[100] = self
			}
			if tc.env != nil {
				r.Getenv = func(k string) string { return tc.env[k] }
			}
			if tc.args != nil {
				tree := r.Procs.(fakeProcs).tree
				p := tree[102]
				p.Args = tc.args
				tree[102] = p
			}
			d, err := Check(r)
			if d.Reason != tc.reason || d.Via != tc.via || d.Allow != (tc.reason == "") {
				t.Fatalf("decision %+v error %v", d, err)
			}
		})
	}
	t.Run("snapshot_error", func(t *testing.T) {
		r := request(t, "grid")
		r.Procs = fakeProcs{err: errors.New("unreadable")}
		d, _ := Check(r)
		if d.Reason != "proc_unreadable" {
			t.Fatal(d)
		}
	})
	t.Run("launcher", func(t *testing.T) {
		r := request(t, "grid", "foo-term")
		dir, _ := r.PersonDir()
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "launchers"), []byte("foo-term\n"), 0600); err != nil {
			t.Fatal(err)
		}
		d, e := Check(r)
		if !d.Allow || d.Via != "terminal" {
			t.Fatalf("%+v %v", d, e)
		}
	})
}
func TestPersonDirectory(t *testing.T) {
	for _, kind := range []string{"fresh", "unwritable", "group_writable", "symlink", "log_symlink", "lock_symlink"} {
		t.Run(kind, func(t *testing.T) {
			if runtime.GOOS == "windows" && (kind == "unwritable" || kind == "group_writable") {
				t.Skip("Unix mode test")
			}
			r := request(t, "grid", "Terminal")
			dir, _ := r.PersonDir()
			if kind != "fresh" {
				if kind == "symlink" {
					if err := os.Symlink(t.TempDir(), dir); err != nil {
						t.Skip(err)
					}
				} else {
					if err := os.Mkdir(dir, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			switch kind {
			case "unwritable":
				if err := os.Chmod(dir, 0500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0700) })
			case "group_writable":
				if err := os.Chmod(dir, 0770); err != nil {
					t.Fatal(err)
				}
			case "log_symlink", "lock_symlink":
				n := "gate.log"
				if kind == "lock_symlink" {
					n = "gate.lock"
				}
				target := filepath.Join(t.TempDir(), "target")
				if err := os.WriteFile(target, []byte("unchanged"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(dir, n)); err != nil {
					t.Skip(err)
				}
			}
			d, e := Check(r)
			if kind != "fresh" {
				if d.Allow || d.Reason != "person_dir" {
					t.Fatalf("%+v %v", d, e)
				}
				return
			}
			if !d.Allow || e != nil {
				t.Fatalf("%+v %v", d, e)
			}
			info, e := os.Stat(dir)
			if e != nil {
				t.Fatal(e)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0700 {
				t.Fatal(info.Mode())
			}
			b, e := os.ReadFile(filepath.Join(dir, "gate.log"))
			if e != nil {
				t.Fatal(e)
			}
			if strings.Count(string(b), "\n") != 1 {
				t.Fatalf("receipt %s", b)
			}
			var receipt map[string]any
			if e = json.Unmarshal(b, &receipt); e != nil {
				t.Fatal(e)
			}
			if receipt["verb"] != "check" || receipt["via"] != "terminal" || len(receipt["argv_sha256"].(string)) != 64 || receipt["time"] != "1970-01-01T00:00:42Z" {
				t.Fatal(receipt)
			}
		})
	}
}
func TestEnvironmentMarkers(t *testing.T) {
	for _, k := range []string{"GRID_AGENT", "GRID_WORKER_NEW", "GRID_LEASE_NEW", "GRID_JOB", "GRID_ROLE", "CLAUDE_CODE_CHILD_SESSION"} {
		t.Run(k, func(t *testing.T) {
			r := request(t, "grid", "Terminal")
			r.Getenv = func(key string) string {
				if key == k {
					return "agent"
				}
				return ""
			}
			r.Environ = func() []string { return []string{k + "=agent"} }
			d, _ := Check(r)
			if d.Reason != "agent_env" {
				t.Fatal(d)
			}
		})
	}
}
func TestPolicyFloor(t *testing.T) {
	r := request(t, "grid", "Terminal")
	dir, _ := r.PersonDir()
	if !Required("approve", nil) || Required("version", nil) || !Required("version", []string{"version"}) {
		t.Fatal("floor/additions")
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "policy"), []byte("remove approve\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if !Required("approve", nil) {
		t.Fatal("person removal bypassed env and R1")
	}
}
func TestSystemSnapshot(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Linux/Darwin smoke")
	}
	tree, e := (SystemProcs{}).Snapshot()
	if e != nil {
		t.Fatal(e)
	}
	p, ok := tree[os.Getpid()]
	if !ok || p.PPID != os.Getppid() {
		t.Fatalf("self %+v", p)
	}
	if _, ok := tree[p.PPID]; !ok {
		t.Fatal("parent missing")
	}
}

func TestAccountHomeIgnoresEnvironment(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("Unix account home")
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	dir, err := PersonDir()
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(dir, os.Getenv("HOME")) || strings.HasPrefix(dir, os.Getenv("XDG_CONFIG_HOME")) {
		t.Fatal("person directory used environment home")
	}
}

func TestNamespaceRootCannotBeLauncher(t *testing.T) {
	r := request(t, "grid")
	r.Procs = fakeProcs{tree: map[int]Process{100: {PID: 100, PPID: 1, Exe: "grid"}, 1: {PID: 1, Exe: "Terminal"}}}
	d, err := Check(r)
	if d.Reason != "detached" || d.Allow {
		t.Fatalf("%+v %v", d, err)
	}
}

func TestPersonAnswerRoles(t *testing.T) {
	for _, role := range []string{"guest", "lead"} {
		for _, verb := range Floor() {
			t.Run(role+"/"+verb, func(t *testing.T) {
				r := request(t, "grid", "bash", "Terminal")
				r.Verb = strings.Fields(verb)
				r.Getenv = func(k string) string {
					if k == "GRID_ROLE" {
						return role
					}
					return ""
				}
				d, err := Check(r)
				answer := verb == "answer" || verb == "question answer"
				if err != nil || d.Allow != answer || (!answer && d.Reason != "agent_env") {
					t.Fatalf("decision %+v error %v", d, err)
				}
			})
		}
	}
}

func TestJetBrainsTerminals(t *testing.T) {
	for _, exe := range []string{"idea", "goland", "pycharm", "webstorm", "clion", "rider", "rubymine", "phpstorm", "datagrip", "rustrover"} {
		for _, host := range []string{exe, strings.ToUpper(exe) + ".EXE"} {
			t.Run(host, func(t *testing.T) {
				d, err := Check(request(t, "grid", "bash", host, "launchd"))
				if err != nil || !d.Allow || d.Via != "terminal" {
					t.Fatalf("decision %+v error %v", d, err)
				}
			})
		}
	}
}

func TestPolicyRemovalScope(t *testing.T) {
	for _, tc := range []struct {
		name, host, marker, reason string
		extra                      bool
	}{
		{name: "detached_person", host: "cron"},
		{name: "agent_ancestor", host: "codex", reason: "agent_ancestor"},
		{name: "agent_env", host: "Terminal", marker: "builder", reason: "agent_env"},
		{name: "workflow_retains_R2", host: "cron", extra: true, reason: "detached"},
		{name: "workflow_retains_receipt", host: "Terminal", extra: true, reason: "person_dir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request(t, "grid", "bash", tc.host)
			r.Verb = []string{"approve"}
			if tc.extra {
				r.ExtraVerbs = []string{"approve"}
			}
			r.Getenv = func(k string) string {
				if k == "GRID_AGENT" {
					return tc.marker
				}
				return ""
			}
			dir, _ := r.PersonDir()
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "policy"), []byte("remove approve\n"), 0600); err != nil {
				t.Fatal(err)
			}
			// A directory at the receipt path cannot be appended to. Policy may
			// skip it, but workflow additions must still require a valid receipt.
			if err := os.Mkdir(filepath.Join(dir, "gate.log"), 0700); err != nil {
				t.Fatal(err)
			}
			d, err := Check(r)
			if d.Allow != (tc.reason == "") || d.Reason != tc.reason {
				t.Fatalf("decision %+v error %v", d, err)
			}
		})
	}
}
