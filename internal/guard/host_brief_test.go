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
	for _, tt := range []struct {
		command string
		deny    bool
	}{
		{"echo yes > .host/approval", true}, {"cat '/data/.HOST/approval'", true}, {"echo .host", true},
		{"echo .host-old", false}, {"echo file.host", false}, {"echo .host.json", false},
		{"grid approve T-01", true}, {"grid answer Q-01 yes", true}, {"grid job result J-01", false},
		{"grid feed read", false},
		{"grid feed post --type post --body '{}'", false},
		{"grid feed read; echo yes >> feed/writer.0.jsonl", true},
		{"echo yes >> feed/writer.0.jsonl", true}, {"echo yes >> feedback.txt", false},
	} {
		t.Run(tt.command, func(t *testing.T) {
			f := setup(t)
			i := f.input("Bash", tt.command)
			want := ""
			if tt.deny {
				want = "deny"
			}
			expect(t, EvaluateHost(i, f.root, f.env), want)
		})
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
func completeBrief() string {
	s := "---\n"
	for _, k := range briefKeys {
		s += k + ": value\n"
	}
	s += "---\n"
	for _, h := range briefSections {
		s += "## " + h + "\n\n"
	}
	return s
}
func TestBriefRules(t *testing.T) {
	for _, field := range briefKeys {
		for _, missing := range []bool{false, true} {
			t.Run("field "+field+map[bool]string{true: "/deny", false: "/allow"}[missing], func(t *testing.T) {
				f := setup(t)
				text := completeBrief()
				if missing {
					text = strings.Replace(text, field+": value\n", field+": ''\n", 1)
				}
				p := filepath.Join(f.task, "subagents", "01-work.md")
				f.write(p, text)
				i := f.input("Agent", "Read "+p)
				want := ""
				if missing {
					want = "deny"
				}
				d := EvaluateBrief(i, f.root, f.env)
				expect(t, d, want)
				checkAdapter(t, BriefLint, i, f.env, d)
			})
		}
	}
	for _, section := range briefSections {
		for _, missing := range []bool{false, true} {
			t.Run("section "+section+map[bool]string{true: "/deny", false: "/allow"}[missing], func(t *testing.T) {
				f := setup(t)
				text := completeBrief()
				if missing {
					text = strings.Replace(text, "## "+section+"\n", "", 1)
				}
				p := filepath.Join(f.task, "subagents", "01-work.md")
				f.write(p, text)
				i := f.input("Task", "Read subagents/01-work.md")
				want := ""
				if missing {
					want = "deny"
				}
				expect(t, EvaluateBrief(i, f.root, f.env), want)
			})
		}
	}
	for _, tt := range []struct {
		name, sub, prompt, text string
		deny                    bool
	}{
		{"missing frontmatter", "subagents", "subagents/01-work.md", "text", true},
		{"complete", "subagents", "subagents/01-work.md", completeBrief(), false},
		{"variable root", "subagents", "${GRID_PROJECTS}/example/tasks/T-01-example/subagents/01-work.md", "text", true},
		{"windows separators", "subagents", `$GRID_PROJECTS\example\tasks\T-01-example\subagents\01-work.md`, "text", true},
		{"app steps", "app-steps/one", "projects/example/tasks/T-01-example/app-steps/one/01-work.md", "text", true},
		{"not named", "subagents", "work on the task", "text", false},
		{"outside", "subagents", "outside/01-work.md", "text", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			f := setup(t)
			f.write(filepath.Join(f.task, tt.sub, "01-work.md"), tt.text)
			i := f.input("Agent", tt.prompt)
			want := ""
			if tt.deny {
				want = "deny"
			}
			expect(t, EvaluateBrief(i, f.root, f.env), want)
		})
	}
	t.Run("gone file", func(t *testing.T) {
		f := setup(t)
		expect(t, EvaluateBrief(f.input("Agent", "subagents/missing.md"), f.root, f.env), "")
	})
	t.Run("unreadable directory", func(t *testing.T) {
		f := setup(t)
		f.write(filepath.Join(f.task, "subagents", "01-work.md", "nested"), "text")
		expect(t, EvaluateBrief(f.input("Agent", "subagents/01-work.md"), f.root, f.env), "")
	})
	t.Run("symlink outside", func(t *testing.T) {
		f := setup(t)
		f.write(filepath.Join(f.repo, "01-work.md"), "broken")
		f.write(filepath.Join(f.task, "subagents", "placeholder"), "")
		f.link(filepath.Join(f.task, "subagents", "01-work.md"), filepath.Join(f.repo, "01-work.md"))
		expect(t, EvaluateBrief(f.input("Agent", "subagents/01-work.md"), f.root, f.env), "")
	})
}
