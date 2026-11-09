package guard

import (
	"fmt"
	"path/filepath"
	"testing"
)

// history.md is the task's receipt log, and the old prose "plan accepted" line in
// it releases a plan. An agent's Bash write to it is fenced as task.md's is.
func TestHostFencesBashWritesToTaskHistory(t *testing.T) {
	f := setup(t)
	f.env["GRID_PLAN_GUARD"] = "off"
	mine := filepath.Join(f.grid, "personal", "tasks", "T-02-mine")
	f.write(filepath.Join(f.task, "history.md"), "")
	f.write(filepath.Join(mine, "history.md"), "")
	const line = "- 2026-11-09T10:00 · plan accepted · x · by abdu"
	for _, dir := range []string{f.task, mine} {
		for _, path := range []string{"history.md", "./history.md", filepath.Join(dir, "history.md")} {
			for _, form := range []string{
				`echo '` + line + `' >> "%s"`, `echo x > "%s"`, `printf x >>%s`, `: > "%s"`,
				`echo x | tee "%s"`, `echo x | tee -a "%s"`,
				`cp /tmp/receipt "%s"`, `mv /tmp/receipt "%s"`, `sed -i s/a/b/ "%s"`, `dd of=%s`,
				`rm "%s"`, `truncate -s 0 "%s"`,
				"cat >> \"%s\" <<'EOF'\nx\nEOF",
				`python3 -c "open('%s', 'a').write('x')"`,
			} {
				command := fmt.Sprintf(form, path)
				t.Run(filepath.Base(dir)+"/"+command, func(t *testing.T) {
					i := f.input("Bash", command)
					i.Cwd = dir
					i.AgentType = "grid-builder"
					d := EvaluateHost(i, f.root, f.env)
					expect(t, d, "deny")
					if d.Reason != gateReason {
						t.Fatalf("reason = %q, want the shared gate reason", d.Reason)
					}
				})
			}
			for _, form := range []string{"cat %s", "head -n 2 %s", "tail -n 3 %s", "grep plan %s", "wc -l %s", "ls -l %s"} {
				command := fmt.Sprintf(form, path)
				t.Run(filepath.Base(dir)+"/"+command, func(t *testing.T) {
					i := f.input("Bash", command)
					i.Cwd = dir
					expect(t, EvaluateHost(i, f.root, f.env), "")
				})
			}
		}
		for _, command := range []string{
			"cd " + dir + " && echo x >> history.md",
			"cd " + dir + "; echo x > history.md",
			"echo x >> histor?.md",
			"echo x >> history.m[d]",
			"echo x >> hist*",
			`echo x >> "$TASK/history.md"`,
			`cp /tmp/receipt $D/histor?.md`,
			`mv /tmp/receipt "$D"/hist*`,
			`echo x >> ` + filepath.Join(dir, "..", filepath.Base(dir), "history.md"),
		} {
			t.Run(filepath.Base(dir)+"/"+command, func(t *testing.T) {
				i := f.input("Bash", command)
				i.Cwd = dir
				expect(t, EvaluateHost(i, f.root, f.env), "deny")
			})
		}
	}
	// Names that only look like it, and a write that is not to the task, stay open.
	for _, lookalike := range []string{"history.md.bak", "my-history.md", "history.json", "history.mdx", "histories.md"} {
		expect(t, EvaluateHost(f.input("Bash", "echo x > "+lookalike), f.root, f.env), "")
	}
	expect(t, EvaluateHost(f.input("Bash", "cp /tmp/receipt $D/notes?.md"), f.root, f.env), "")
	// The same line written to task.md or approval.json stays refused: the positive controls.
	for _, name := range []string{"task.md", "approval.json"} {
		expect(t, EvaluateHost(f.input("Bash", `echo '`+line+`' >> `+name), f.root, f.env), "deny")
	}
	// A note written through Grid's own writer names no file.
	expect(t, EvaluateHost(f.input("Bash", `grid history add "checked the plan" --task T-01`), f.root, f.env), "")
}
