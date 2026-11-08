package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHomePersonalToolsAndAliases(t *testing.T) {
	for _, tool := range []string{"Read", "Write", "Edit", "MultiEdit", "NotebookEdit", "Glob", "Grep"} {
		for _, event := range []string{"PreToolUse", "PermissionRequest"} {
			for _, level := range []string{"off", "warn", "block"} {
				t.Run(tool+"/"+event+"/"+level, func(t *testing.T) {
					f := setup(t)
					f.env["GRID_ROLE"], f.env["GRID_MEMBER"], f.env["GRID_PROJECT"] = "guest", "gus", "example"
					f.write(filepath.Join(f.projects, "example", "project.md"), "---\nid: example-id\n---\n")
					f.write(filepath.Join(f.grid, "settings.json"), `{"hooks":{"home-allow":"`+level+`"}}`)
					personal := filepath.Join(f.grid, "personal")
					f.write(filepath.Join(personal, "notes.md"), "private")
					alias := filepath.Join(f.projects, "example", "alias")
					f.link(alias, personal)
					for _, p := range []string{personal, filepath.Join(personal, "missing.md"), alias, filepath.Join(alias, "notes.md"), f.grid, f.projects} {
						in := f.input(tool, p)
						in.Cwd, in.HookEventName = f.repo, event
						expect(t, EvaluateHome(in, f.root, f.env), "deny")
					}
					rel, err := filepath.Rel(f.repo, filepath.Join(personal, "notes.md"))
					if err != nil {
						t.Fatal(err)
					}
					in := f.input(tool, rel)
					in.Cwd, in.HookEventName = f.repo, event
					expect(t, EvaluateHome(in, f.root, f.env), "deny")
				})
			}
		}
	}
}

func TestHomePersonalLegacyScripts(t *testing.T) {
	for _, role := range []string{"guest", "unknown", "owner", "worker", "grid-worker"} {
		f := setup(t)
		f.env["GRID_ROLE"], f.env["GRID_PROJECT"] = role, "example"
		f.write(filepath.Join(f.projects, "example", "project.md"), "---\nid: example-id\n---\n")
		script := filepath.Join(f.plugin, "skills", "plan", "scripts", "plan-hash.mjs")
		f.write(script, "// fixture")
		for _, suffix := range []string{"", " " + filepath.Join(f.grid, "personal", "notes.md")} {
			in := f.input("Bash", "node "+script+suffix)
			in.Cwd = f.repo
			want := "allow"
			if role == "guest" || role == "unknown" {
				want = "deny"
			}
			expect(t, EvaluateHome(in, f.root, f.env), want)
		}
	}
}

func TestHomePersonalGlobCannotEscape(t *testing.T) {
	f := setup(t)
	f.env["GRID_ROLE"], f.env["GRID_PROJECT"] = "guest", "example"
	f.write(filepath.Join(f.projects, "example", "project.md"), "---\nid: example-id\n---\n")
	for _, pattern := range []string{"../../personal/**", filepath.Join(f.grid, "personal", "*")} {
		in := f.input("Glob", filepath.Join(f.projects, "example"))
		in.Cwd = f.repo
		in.ToolInput["pattern"] = pattern
		expect(t, EvaluateHome(in, f.root, f.env), "deny")
	}
}

func TestHomeRoleAccess(t *testing.T) {
	for _, row := range []struct {
		name, role, member string
		home, personal     bool
		directory          bool
	}{
		{"owner", "owner", "gus", true, true, true},
		{"member without identity", "member", "", true, false, false},
		{"member with identity", "member", "gus", true, false, false},
		{"lead", "lead", "gus", true, false, false},
		{"lead without identity", "lead", "", true, false, false},
		{"worker", "worker", "gus", true, true, true},
		{"guest", "guest", "gus", false, false, true},
		{"unknown", "unknown", "gus", false, false, false},
	} {
		for _, prefix := range []string{"", "grid-"} {
			for _, event := range []string{"PreToolUse", "PermissionRequest"} {
				t.Run(row.name+"/"+prefix+"/"+event, func(t *testing.T) {
					f := setup(t)
					f.env["GRID_ROLE"], f.env["GRID_MEMBER"], f.env["GRID_PROJECT"] = prefix+row.role, row.member, "example"
					f.write(filepath.Join(f.projects, "example", "project.md"), "---\nid: example-id\n---\n")
					script := filepath.Join(f.plugin, "skills", "plan", "scripts", "plan-hash.mjs")
					f.write(script, "// fixture")
					for _, target := range []struct {
						name, tool, path string
						allow            bool
					}{
						{"active project", "Read", filepath.Join(f.projects, "example", "notes.md"), row.role != "unknown"},
						{"another project", "Read", filepath.Join(f.projects, "beta", "notes.md"), row.home},
						{"knowledge", "Read", filepath.Join(f.grid, "knowledge", "x.md"), row.home},
						{"settings", "Read", filepath.Join(f.grid, "settings.json"), row.home},
						{"Personal", "Read", filepath.Join(f.grid, "personal", "notes.md"), row.personal},
						{"own script", "Bash", "node " + script, row.home},
						{"script project argument", "Bash", "node " + script + " " + filepath.Join(f.projects, "example", "notes.md"), row.home},
						{"script Personal argument", "Bash", "node " + script + " " + filepath.Join(f.grid, "personal", "notes.md"), row.personal},
					} {
						t.Run(target.name, func(t *testing.T) {
							in := f.input(target.tool, target.path)
							in.Cwd, in.HookEventName = f.repo, event
							got := EvaluateHome(in, f.root, f.env)
							want := "deny"
							if target.allow {
								want = "allow"
							}
							if got.Behavior != want {
								t.Fatalf("behavior=%q, want %q", got.Behavior, want)
							}
							wantDirs := 0
							if event == "PermissionRequest" && target.allow && row.directory {
								wantDirs = 1
							}
							if len(got.Directories) != wantDirs {
								t.Fatalf("directory grants=%d, want %d", len(got.Directories), wantDirs)
							}
						})
					}
				})
			}
		}
	}
}

func TestHomeGlobStructure(t *testing.T) {
	for _, role := range []string{"owner", "member", "guest"} {
		for _, event := range []string{"PreToolUse", "PermissionRequest"} {
			f := setup(t)
			f.env["GRID_ROLE"], f.env["GRID_PROJECT"] = role, "example"
			f.write(filepath.Join(f.projects, "example", "project.md"), "---\nid: example-id\n---\n")
			root := filepath.Join(f.projects, "example")
			f.link(filepath.Join(root, "escape"), f.repo)
			for _, row := range []struct {
				name, pattern string
				allow         bool
			}{
				{"recursive", "**/*.md", true},
				{"braces", "{notes,docs}/**", true},
				{"nested safe braces", "{notes,{docs,src}}/**", true},
				{"absolute inside", filepath.Join(root, "notes", "*.md"), true},
				{"parent", "../personal/**", false},
				{"brace parent", "{..,x}/personal/**", false},
				{"brace assembled parent", "{.,x}./personal/**", false},
				{"nested parent", "{x,{.,y}}./personal/**", false},
				{"split parent", "{.,x}{.,y}/personal/**", false},
				{"empty alternative parent", "..{,x}/personal/**", false},
				{"backslash parent", `{..,x}\personal\**`, false},
				{"inner parent", "notes/{..,x}/**", false},
				{"absolute outside", filepath.Join(f.repo, "*.md"), false},
				{"absolute mixed braces", "{" + root + "," + f.repo + "}/**", false},
				{"absolute symlink escape", filepath.Join(root, "escape", "*.md"), false},
				{"unbalanced open", "{notes,docs/**", false},
				{"unbalanced close", "notes,docs}/**", false},
				{"expansion bound", strings.Repeat("{a,b}", 9) + "/**", false},
			} {
				t.Run(role+"/"+event+"/"+row.name, func(t *testing.T) {
					in := f.input("Glob", root)
					in.Cwd, in.HookEventName = f.repo, event
					in.ToolInput["pattern"] = row.pattern
					got := EvaluateHome(in, f.root, f.env)
					want := "deny"
					if row.allow {
						want = "allow"
					}
					if got.Behavior != want {
						t.Fatalf("glob behavior=%q, want %q", got.Behavior, want)
					}
				})
			}
		}
	}
}
