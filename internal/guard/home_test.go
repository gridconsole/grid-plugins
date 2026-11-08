package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

func TestHomeFileRules(t *testing.T) {
	for _, tool := range []string{"Read", "Write", "Edit", "MultiEdit", "NotebookEdit", "Glob", "Grep"} {
		for _, allow := range []bool{true, false} {
			t.Run(tool+map[bool]string{true: "/allow", false: "/defer"}[allow], func(t *testing.T) {
				f := setup(t)
				p := filepath.Join(f.grid, "notes.md")
				if !allow {
					p = filepath.Join(f.repo, "notes.md")
				}
				i := f.input(tool, p)
				i.Cwd = f.repo
				want := ""
				if allow {
					want = "allow"
				}
				d := EvaluateHome(i, f.root, f.env)
				expect(t, d, want)
				checkAdapter(t, HomeAllow, i, f.env, d)
			})
		}
	}
	for _, tt := range []struct {
		name   string
		change func(*fixture, *hookio.Input)
		allow  bool
	}{
		{"relative path", func(f *fixture, i *hookio.Input) { i.ToolInput["file_path"] = "notes.md" }, false},
		{"tilde path", func(f *fixture, i *hookio.Input) { i.ToolInput["file_path"] = "~/.grid/notes.md" }, false},
		{"dotdot escape", func(f *fixture, i *hookio.Input) { i.ToolInput["file_path"] = f.grid + "/../notes.md" }, false},
		{"relative cwd", func(f *fixture, i *hookio.Input) { i.Cwd = "repo" }, false},
		{"missing cwd", func(f *fixture, i *hookio.Input) { i.Cwd = "" }, false},
		{"relative home", func(f *fixture, i *hookio.Input) { f.env["GRID_HOME"] = "grid" }, false},
		{"root home", func(f *fixture, i *hookio.Input) { f.env["GRID_HOME"] = string(filepath.Separator) }, false},
		{"user home", func(f *fixture, i *hookio.Input) { f.env["GRID_HOME"] = f.home }, false},
		{"home ancestor", func(f *fixture, i *hookio.Input) { f.env["GRID_HOME"] = f.root }, false},
		{"cwd inside home", func(f *fixture, i *hookio.Input) { i.Cwd = f.task }, false},
		{"cwd equals home", func(f *fixture, i *hookio.Input) { i.Cwd = f.grid }, false},
		{"safe separate home", func(f *fixture, i *hookio.Input) {}, true},
		{"permission request", func(f *fixture, i *hookio.Input) { i.HookEventName = "PermissionRequest" }, true},
		{"other event", func(f *fixture, i *hookio.Input) { i.HookEventName = "Stop" }, false},
		{"default single home", func(f *fixture, i *hookio.Input) { delete(f.env, "GRID_HOME"); delete(f.env, "GRID_PROJECTS") }, true},
		{"projects alone", func(f *fixture, i *hookio.Input) { delete(f.env, "GRID_HOME") }, false},
		{"symlink escape", func(f *fixture, i *hookio.Input) {
			f.link(filepath.Join(f.grid, "link"), f.repo)
			i.ToolInput["file_path"] = filepath.Join(f.grid, "link", "new")
		}, false},
		{"dangling symlink escape", func(f *fixture, i *hookio.Input) {
			f.link(filepath.Join(f.grid, "link"), filepath.Join(f.repo, "missing"))
			i.ToolInput["file_path"] = filepath.Join(f.grid, "link", "new")
		}, false},
		{"outside alias inside", func(f *fixture, i *hookio.Input) {
			f.link(filepath.Join(f.root, "link"), f.grid)
			i.ToolInput["file_path"] = filepath.Join(f.root, "link", "new")
		}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)
			i := f.input("Read", filepath.Join(f.grid, "notes.md"))
			i.Cwd = f.repo
			tt.change(f, &i)
			want := ""
			if tt.allow {
				want = "allow"
			}
			d := EvaluateHome(i, f.root, f.env)
			expect(t, d, want)
			if i.HookEventName == "PermissionRequest" && tt.allow {
				if len(d.Directories) != 1 || d.Directories[0] != f.grid {
					t.Fatalf("session directory: %+v", d)
				}
			}
		})
	}
	for _, pattern := range []string{"**/*.md", "notes/*.md", "../*.md", "a/../*.md", `a\..\*.md`, "/absolute/*.md", "~/notes", "C:notes"} {
		t.Run("glob "+pattern, func(t *testing.T) {
			f := setup(t)
			i := f.input("Glob", f.grid)
			i.Cwd = f.repo
			i.ToolInput["pattern"] = pattern
			want := ""
			if pattern == "**/*.md" || pattern == "notes/*.md" {
				want = "allow"
			}
			expect(t, EvaluateHome(i, f.root, f.env), want)
		})
	}
}
func TestHomeScripts(t *testing.T) {
	cases := []struct {
		name, script, args, prefix, suffix string
		allow                              bool
	}{
		{name: "own no args", script: "plan/plan-hash.mjs", allow: true},
		{name: "home path", script: "plan/plan-hash.mjs", args: "{GRID}/plan.md", allow: true},
		{name: "outside path", script: "plan/plan-hash.mjs", args: "{REPO}/plan.md"},
		{name: "bare file", script: "plan/plan-hash.mjs", args: "plan.md"},
		{name: "flag equals path", script: "plan/plan-hash.mjs", args: "--task={REPO}/plan.md"},
		{name: "flag equals text", script: "plan/plan-hash.mjs", args: "--task=T-01", allow: true},
		{name: "json object", script: "signals/add-signal.mjs", args: `'{"url":"https://example.com"}'`, allow: true},
		{name: "json array", script: "signals/add-signal.mjs", args: `'["/outside"]'`, allow: true},
		{name: "quoted punctuation", script: "signals/add-signal.mjs", args: `'ordinary ; & | > < ( ) text'`, allow: true},
		{name: "shell operator", script: "plan/plan-hash.mjs", suffix: "; echo bad"},
		{name: "pipe", script: "plan/plan-hash.mjs", suffix: " | cat"},
		{name: "redirect", script: "plan/plan-hash.mjs", suffix: " > output"},
		{name: "dollar in quotes", script: "plan/plan-hash.mjs", args: `'$OUTPUT'`},
		{name: "backtick", script: "plan/plan-hash.mjs", args: "'`command`'"},
		{name: "backslash", script: "plan/plan-hash.mjs", args: `'a\b'`},
		{name: "newline", script: "plan/plan-hash.mjs", args: "'a\nb'"},
		{name: "tab", script: "plan/plan-hash.mjs", args: "a\tb"},
		{name: "unbalanced quote", script: "plan/plan-hash.mjs", args: "'text"},
		{name: "nothing before node", script: "plan/plan-hash.mjs", prefix: " "},
		{name: "env before node", script: "plan/plan-hash.mjs", prefix: "X=1 "},
		{name: "proof caption", script: "deliver/proof-add.mjs", args: `--caption 'path/to/file.go'`, allow: true},
		{name: "proof title", script: "deliver/proof-add.mjs", args: `--title 'version 1.2'`, allow: true},
		{name: "proof https", script: "deliver/proof-add.mjs", args: `--url https://example.com/proof`, allow: true},
		{name: "proof http", script: "deliver/proof-add.mjs", args: `--url http://example.com/proof`, allow: true},
		{name: "proof file URL", script: "deliver/proof-add.mjs", args: `--url file:///outside/file`},
		{name: "proof other flag", script: "deliver/proof-add.mjs", args: `--path {REPO}/file.png`},
		{name: "caption only exception on proof", script: "plan/plan-hash.mjs", args: `--caption 'path/to/file.go'`},
		{name: "pairs not searched", script: "deliver/proof-add.mjs", args: `--x value extra --caption {REPO}/file.go`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)
			parts := strings.Split(tt.script, "/")
			script := filepath.Join(f.plugin, "skills", parts[0], "scripts", parts[1])
			f.write(script, "// fixture\n")
			args := strings.NewReplacer("{GRID}", f.grid, "{REPO}", f.repo).Replace(tt.args)
			cmd := tt.prefix + "node " + script
			if args != "" {
				cmd += " " + args
			}
			cmd += tt.suffix
			i := f.input("Bash", cmd)
			i.Cwd = f.repo
			want := ""
			if tt.allow {
				want = "allow"
			}
			d := EvaluateHome(i, f.root, f.env)
			expect(t, d, want)
			checkAdapter(t, HomeAllow, i, f.env, d)
		})
	}
	for _, escape := range []bool{false, true} {
		t.Run(map[bool]string{true: "script symlink escape", false: "script symlink inside"}[escape], func(t *testing.T) {
			f := setup(t)
			script := filepath.Join(f.plugin, "skills", "plan", "scripts", "script.mjs")
			dest := filepath.Join(f.plugin, "skills", "plan", "scripts", "real.mjs")
			if escape {
				dest = filepath.Join(f.repo, "outside.mjs")
			}
			f.write(dest, "// fixture")
			f.write(filepath.Join(filepath.Dir(script), "placeholder"), "")
			f.link(script, dest)
			i := f.input("Bash", "node "+script)
			i.Cwd = f.repo
			want := "allow"
			if escape {
				want = ""
			}
			expect(t, EvaluateHome(i, f.root, f.env), want)
		})
	}
}
func TestBadInputAdapters(t *testing.T) {
	for name, fn := range map[string]Adapter{"plan": PlanGuard, "host": HostGuard, "brief": BriefLint, "home": HomeAllow} {
		for _, data := range []string{"", "null", "[]", "{", `{"tool_input":false}`, `{"tool_name":"Write","tool_input":{"file_path":7}}`} {
			t.Run(name+"/"+data, func(t *testing.T) {
				out, code := fn([]byte(data), nil)
				if len(out) != 0 || code != 0 {
					t.Fatalf("%s, %d", out, code)
				}
			})
		}
	}
}

func TestHomeRepoGlobsStaySilent(t *testing.T) {
	for _, role := range []string{"", "owner", "grid-owner", "worker", "builder", "grid-builder"} {
		for _, event := range []string{"PreToolUse", "PermissionRequest"} {
			for _, path := range []string{"omitted", "repo", "unsafe home"} {
				for _, pattern := range []string{"absolute", "../app/*.go", "{main}.go"} {
					t.Run(role+"/"+event+"/"+path+"/"+pattern, func(t *testing.T) {
						f := setup(t)
						f.env["GRID_ROLE"] = role
						in := f.input("Glob", "")
						in.Cwd, in.HookEventName = f.repo, event
						if path != "omitted" {
							in.ToolInput["path"] = f.repo
						}
						if path == "unsafe home" {
							f.env["GRID_HOME"] = f.home
						}
						if pattern == "absolute" {
							in.ToolInput["pattern"] = filepath.Join(f.repo, "*.go")
						} else {
							in.ToolInput["pattern"] = pattern
						}
						got := EvaluateHome(in, f.root, f.env)
						if got.Behavior != "" || len(got.Directories) != 0 {
							t.Fatalf("repo glob behavior=%q, directory grants=%d; want silence", got.Behavior, len(got.Directories))
						}
					})
				}
			}
		}
	}
}
