package guard

import (
	"path/filepath"
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
