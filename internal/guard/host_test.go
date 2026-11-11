package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestHostRules(t *testing.T) {
	for _, tool := range []string{"Write", "Edit", "MultiEdit", "NotebookEdit"} {
		for _, deny := range []bool{false, true} {
			t.Run(tool+map[bool]string{true: "/deny", false: "/allow"}[deny], func(t *testing.T) {
				f := setup(t)
				folder := filepath.Join(filepath.Dir(f.grid), ".HOST")
				if !deny {
					folder += "-other"
				}
				i := f.input(tool, filepath.Join(folder, "approval.json"))
				f.env["GRID_PLAN_GUARD"] = "off"
				want := ""
				if deny {
					want = "deny"
				}
				d := EvaluateHost(i, f.root, f.env)
				expect(t, d, want)
				checkAdapter(t, HostGuard, i, f.env, d)
			})
		}
	}
	t.Run("single home keeps .host inside it", func(t *testing.T) {
		f := setup(t)
		for _, rel := range []string{".host/approval", ".host", ".host/queue/one"} {
			expect(t, EvaluateHost(f.input("Write", filepath.Join(f.grid, rel)), f.root, f.env), "deny")
		}
		expect(t, EvaluateHost(f.input("Write", filepath.Join(f.grid, ".hosting", "x")), f.root, f.env), "")
		expect(t, EvaluateHost(f.input("Write", filepath.Join(f.grid, "notes.host")), f.root, f.env), "")
		i := f.input("Write", filepath.Join(f.grid, ".host", "approval"))
		if d := EvaluateHost(i, f.root, f.env); !strings.Contains(d.Reason, filepath.Join(f.grid, ".host")) {
			t.Fatalf("reason names another folder: %s", d.Reason)
		}
	})
	t.Run("single home .host spelled in another case", func(t *testing.T) {
		f := setup(t)
		skipUnlessFolding(t, f)
		f.write(filepath.Join(f.grid, ".host", "existing"), "")
		for _, p := range []string{alias(filepath.Join(f.grid, ".host", "approval"), ".grid"), alias(filepath.Join(f.grid, ".host", "existing"), ".host")} {
			expect(t, EvaluateHost(f.input("Write", p), f.root, f.env), "deny")
		}
	})
	for _, dangling := range []bool{false, true} {
		t.Run(map[bool]string{false: "linked folder", true: "dangling link"}[dangling], func(t *testing.T) {
			f := setup(t)
			host := hostFolder(f.projects)
			if !dangling {
				f.write(filepath.Join(host, "existing"), "")
			}
			link := filepath.Join(f.root, "alias")
			f.link(link, host)
			expect(t, EvaluateHost(f.input("Write", filepath.Join(link, "new")), f.root, f.env), "deny")
		})
	}
	t.Run("linked projects both host readings", func(t *testing.T) {
		f := setup(t)
		actual := filepath.Join(f.root, "elsewhere", "data", "projects")
		f.write(filepath.Join(actual, "placeholder"), "")
		alias := filepath.Join(f.root, "alias-projects")
		f.link(alias, actual)
		f.env["GRID_PROJECTS"] = alias
		for _, p := range []string{hostFolder(alias), hostFolder(actual)} {
			expect(t, EvaluateHost(f.input("Write", filepath.Join(p, "new")), f.root, f.env), "deny")
		}
	})
	for _, rel := range []string{"feed", "feed/writer.0.jsonl", "FEED/bodies/record.json"} {
		t.Run(rel, func(t *testing.T) {
			f := setup(t)
			expect(t, EvaluateHost(f.input("Write", filepath.Join(f.task, rel)), f.root, f.env), "deny")
			expect(t, EvaluateHost(f.input("Write", filepath.Join(f.task, "feed-notes.md")), f.root, f.env), "")
		})
	}
	t.Run("no folders", func(t *testing.T) {
		f := setup(t)
		expect(t, EvaluateHost(f.input("Bash", "echo .host"), f.root, nil), "")
	})
	t.Run("fallback does not fence shell", func(t *testing.T) {
		f := setup(t)
		delete(f.env, "GRID_PROJECTS")
		delete(f.env, "GRID_HOME")
		expect(t, EvaluateHost(f.input("Bash", "echo .host"), f.root, f.env), "")
		expect(t, EvaluateHost(f.input("Write", filepath.Join(hostFolder(f.projects), "new")), f.root, f.env), "deny")
	})
}
