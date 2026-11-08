package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/plan"
)

func planFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "plan", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func planTask(h *harness) string {
	path := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-07-tag-filter")
	h.write(filepath.Join(path, "plan.md"), planFixture(h.t, "plan.md"))
	return path
}

func TestPlanReadGolden(t *testing.T) {
	for _, action := range []string{"hash", "items"} {
		for _, format := range []string{"plain", "json"} {
			t.Run(action+"/"+format, func(t *testing.T) {
				h := newHarness(t)
				task := planTask(h)
				h.write(filepath.Join(task, "items.md"), planFixture(t, "items.md"))
				h.write(filepath.Join(task, "subagents", "toolbar.md"), "---\nstatus: running\nitem: 2\n---\nBuild the toolbar.\n")
				args := []string{"plan", action}
				if format == "json" {
					args = append(args, "--json")
				}
				if code := h.run(task, args...); code != 0 {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				golden.Assert(t, "plan-cli-"+action+"-"+format, h.out.String())
			})
		}
	}
}

func TestPlanWriteAndDiffGolden(t *testing.T) {
	for _, format := range []string{"plain", "json"} {
		t.Run(format, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			source := filepath.Join(h.tmp, "draft.md")
			draft := planFixture(t, "revised.md")
			h.write(source, draft)
			args := []string{"plan", "write", "draft.md", "--task", task}
			if format == "json" {
				args = append(args, "--json")
			}
			if code := h.run(h.tmp, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, "plan-cli-write-"+format, h.out.String())
			data, err := os.ReadFile(source)
			if err != nil || string(data) != draft {
				t.Fatal("source file changed")
			}
			data, err = os.ReadFile(filepath.Join(task, "plan-revisions", "3.md"))
			if err != nil || string(data) != planFixture(t, "plan.md") {
				t.Fatal("previous revision not preserved")
			}
			args = []string{"plan", "diff", "3", "--task", task, "4"}
			if format == "json" {
				args = append(args, "--json")
			}
			if code := h.run(h.tmp, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, "plan-cli-diff-"+format, h.out.String())
		})
	}
}

func TestPlanTaskResolution(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	nested := filepath.Join(task, "checks", "R-1")
	h.mkdir(nested)
	for _, tc := range []struct {
		cwd  string
		args []string
	}{
		{nested, []string{"plan", "hash", "--json"}},
		{h.tmp, []string{"plan", "hash", "--task", task, "--json"}},
		{task, []string{"plan", "hash", "--task=T-07", "--json"}},
	} {
		if code := h.run(tc.cwd, tc.args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", tc.args, code, h.errb)
		}
		var got struct {
			Hash string `json:"hash"`
		}
		if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Hash != plan.Hash(planFixture(t, "plan.md")) {
			t.Fatalf("%s %v", h.out, err)
		}
	}
	if code := h.run(h.tmp, "plan", "hash"); code != 1 || !strings.Contains(h.errb.String(), "--task") {
		t.Fatalf("no task: %d %s", code, h.errb)
	}
	if code := h.run(h.tmp, "plan", "hash", "--task", h.tmp); code != 1 {
		t.Fatalf("outside Grid home: %d", code)
	}
}

func TestPlanUsageAndRefusals(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	for _, args := range [][]string{
		{"plan"}, {"plan", "bogus"}, {"plan", "hash", "extra"}, {"plan", "write"}, {"plan", "diff", "1"}, {"plan", "diff", "1", "2", "3"}, {"plan", "items", "--unknown"}, {"plan", "hash", "--task"},
	} {
		if code := h.run(task, args...); code != 2 {
			t.Errorf("%v: %d %s", args, code, h.errb)
		}
	}
	for _, args := range [][]string{{"plan", "--help"}, {"plan", "diff", "--help"}} {
		if code := h.run(task, args...); code != 0 {
			t.Errorf("help: %d", code)
		}
	}
	h.write(filepath.Join(task, "bad.md"), "# Plan\n- [ ] 1 No recall citation\n")
	if code := h.run(task, "plan", "write", "bad.md"); code != 1 || !strings.Contains(h.errb.String(), "Used:") {
		t.Fatalf("missing recall: %d %s", code, h.errb)
	}
	if code := h.run(task, "plan", "write", "missing.md"); code != 1 {
		t.Fatalf("missing input: %d", code)
	}
	h.write(filepath.Join(task, "-draft.md"), "# Plan\nRecall: nothing found\n- [ ] 1 First\n  jobs: []\n")
	if code := h.run(task, "plan", "write", "--", "-draft.md"); code != 0 {
		t.Fatalf("literal filename: %d %s", code, h.errb)
	}
	if code := h.run(task, "plan", "diff", "3", "99"); code != 1 {
		t.Fatalf("missing revision: %d", code)
	}
}

func TestPlanMigrate(t *testing.T) {
	for _, name := range []string{"form-a", "form-b", "form-b-revised"} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			old := planFixture(t, "legacy/"+name+".md")
			// The historical A example contains no citation; the owner supplies it.
			if !plan.HasRecall(old) {
				old += "\nRecall: nothing found\n"
			}
			h.write(filepath.Join(task, "plan.md"), old)
			rev, err := plan.Revision(old)
			if err != nil {
				t.Fatal(err)
			}
			if code := h.run(h.tmp, "plan", "migrate", "--task", task, "--json"); code != 0 {
				t.Fatalf("%d: %s", code, h.errb)
			}
			var got struct {
				Form     string
				Revision uint64
				Hash     string
			}
			if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			if got.Revision != rev+1 || got.Hash == plan.Hash(old) {
				t.Fatalf("%+v", got)
			}
			installed, err := os.ReadFile(filepath.Join(task, "plan.md"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := plan.Items(string(installed)); err != nil {
				t.Fatal(err)
			}
			archived, err := os.ReadFile(filepath.Join(task, "plan-revisions", fmt.Sprintf("%d.md", rev)))
			if err != nil || string(archived) != old {
				t.Fatal("archive is not the original plan")
			}
			if code := h.run(task, "plan", "migrate"); code != 0 || !strings.Contains(h.out.String(), "plan.md is already current") {
				t.Fatalf("%d: %s %s", code, h.out, h.errb)
			}
			unchanged, err := os.ReadFile(filepath.Join(task, "plan.md"))
			if err != nil || string(unchanged) != string(installed) {
				t.Fatal("second migration changed the plan")
			}
		})
	}
}

func TestPlanMigrateDryRun(t *testing.T) {
	for _, name := range []string{"form-a", "form-b", "form-b-revised", "form-c"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/json=%t", name, asJSON), func(t *testing.T) {
				h := newHarness(t)
				task := planTask(h)
				old := planFixture(t, "legacy/"+name+".md")
				h.write(filepath.Join(task, "plan.md"), old)
				before, err := os.ReadDir(task)
				if err != nil {
					t.Fatal(err)
				}
				want, form, err := plan.Upgrade(old)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"plan", "migrate", "--dry-run"}
				if asJSON {
					args = append(args, "--json")
				}
				if code := h.run(task, args...); code != 0 {
					t.Fatalf("%d: %s", code, h.errb)
				}
				if asJSON {
					var got struct{ Form, Text string }
					if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Text != want || got.Form != form {
						t.Fatalf("%s: %v", h.out, err)
					}
				} else if h.out.String() != "form: "+form+"\n"+want {
					t.Fatalf("unexpected preview: %s", h.out)
				}
				after, err := os.ReadDir(task)
				if err != nil || len(before) != len(after) {
					t.Fatal("dry run created files")
				}
				data, err := os.ReadFile(filepath.Join(task, "plan.md"))
				if err != nil || string(data) != old {
					t.Fatal("dry run changed plan")
				}
			})
		}
	}
}

func TestPlanMigrateApproval(t *testing.T) {
	for _, fixture := range []string{"form-b", "form-c"} {
		for _, approvalFile := range []string{"approval.json", "approval-ship.json"} {
			for _, tc := range []struct {
				name, hash string
				yes        bool
				code       int
			}{
				{"matching approval refused", "matching", false, 1},
				{"matching approval confirmed", "matching", true, 0},
				{"stale approval", "000000", false, 0},
			} {
				t.Run(fixture+"/"+approvalFile+"/"+tc.name, func(t *testing.T) {
					h := newHarness(t)
					task := planTask(h)
					old := planFixture(t, "legacy/"+fixture+".md") + "\nRecall: nothing found\n"
					h.write(filepath.Join(task, "plan.md"), old)
					hash := tc.hash
					if hash == "matching" {
						hash = plan.Hash(old)
					}
					approval := "{\"plan_hash\":\"" + hash + "\"}\n"
					h.write(filepath.Join(task, approvalFile), approval)
					args := []string{"plan", "migrate"}
					if tc.yes {
						args = append(args, "--yes")
					}
					if code := h.run(task, args...); code != tc.code {
						t.Fatalf("%d: %s", code, h.errb)
					}
					if tc.code == 1 {
						if !strings.Contains(h.errb.String(), "need approving again") {
							t.Fatal(h.errb.String())
						}
						data, err := os.ReadFile(filepath.Join(task, "plan.md"))
						if err != nil || string(data) != old {
							t.Fatal("refusal changed plan")
						}
						if _, err := os.Stat(filepath.Join(task, "plan-revisions")); !os.IsNotExist(err) {
							t.Fatal("refusal created archive")
						}
					}
					if tc.code == 0 {
						data, err := os.ReadFile(filepath.Join(task, "plan.md"))
						if err != nil || plan.Hash(string(data)) == plan.Hash(old) {
							t.Fatal("old approval still matches migrated plan")
						}
					}
					data, err := os.ReadFile(filepath.Join(task, approvalFile))
					if err != nil || string(data) != approval {
						t.Fatal("approval changed")
					}
				})
			}
		}
	}
}

func TestPlanMigrateMissingRecall(t *testing.T) {
	for _, name := range []string{"form-a", "form-b", "form-c"} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			old := planFixture(t, "legacy/"+name+".md")
			old = strings.ReplaceAll(old, "Used: brain:project/conventions/tag-matching\n", "")
			h.write(filepath.Join(task, "plan.md"), old)
			if code := h.run(task, "plan", "migrate"); code != 1 || !strings.Contains(h.errb.String(), "Used:") {
				t.Fatalf("%d: %s", code, h.errb)
			}
			data, err := os.ReadFile(filepath.Join(task, "plan.md"))
			if err != nil || string(data) != old {
				t.Fatal("refusal changed plan")
			}
		})
	}
}

func TestPlanLegacyHint(t *testing.T) {
	for _, name := range []string{"form-a", "form-b", "form-c"} {
		for _, action := range []string{"items", "check", "write"} {
			t.Run(name+"/"+action, func(t *testing.T) {
				h := newHarness(t)
				task := planTask(h)
				text := planFixture(t, "legacy/"+name+".md") + "\nRecall: nothing found\n"
				h.write(filepath.Join(task, "plan.md"), text)
				args := []string{"plan", action}
				if action == "write" {
					args = append(args, "plan.md")
				}
				if code := h.run(task, args...); code != 1 {
					t.Fatalf("exit %d", code)
				}
				if !strings.Contains(h.out.String()+h.errb.String(), "this looks like an older plan format; run grid plan migrate --dry-run") {
					t.Fatalf("missing hint: %s %s", h.out, h.errb)
				}
			})
		}
	}
}

func TestPlanMigrateCurrentNoOp(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	before, err := os.ReadDir(task)
	if err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "plan", "migrate", "--json"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
	var got struct{ Form, Message string }
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Form != "current" || got.Message != "plan.md is already current" {
		t.Fatalf("%s: %v", h.out, err)
	}
	after, err := os.ReadDir(task)
	if err != nil || len(before) != len(after) {
		t.Fatal("current migration created files")
	}
	data, err := os.ReadFile(filepath.Join(task, "plan.md"))
	if err != nil || string(data) != planFixture(t, "plan.md") {
		t.Fatal("current migration changed plan")
	}
}

func TestPlanMigrateLegacyFrontmatterRevision(t *testing.T) {
	h := newHarness(t)
	task := planTask(h)
	old := planFixture(t, "legacy/form-c.md") + "\nRecall: nothing found\n"
	h.write(filepath.Join(task, "plan.md"), old)
	// The ordinary write path must retain its revision requirement.
	converted, _, err := plan.Upgrade(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (plan.Store{TaskDir: task}).Write(converted); err == nil {
		t.Fatal("ordinary write accepted an unnumbered installed plan")
	}
	if code := h.run(task, "plan", "migrate"); code != 0 {
		t.Fatalf("%d: %s", code, h.errb)
	}
	data, err := os.ReadFile(filepath.Join(task, "plan-revisions", "1.md"))
	if err != nil || string(data) != old {
		t.Fatalf("original archive: %v", err)
	}
	installed, err := os.ReadFile(filepath.Join(task, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if rev, err := plan.Revision(string(installed)); err != nil || rev != 2 {
		t.Fatalf("revision %d: %v", rev, err)
	}
	if !strings.Contains(string(installed), converted) || plan.Hash(string(installed)) == plan.Hash(old) {
		t.Fatal("migration lost converted content or kept the old approval hash")
	}
	if code := h.run(task, "plan", "diff", "1", "2"); code != 0 {
		t.Fatalf("diff %d: %s", code, h.errb)
	}
	if code := h.run(task, "plan", "migrate"); code != 0 {
		t.Fatalf("second migration %d: %s", code, h.errb)
	}
	data, err = os.ReadFile(filepath.Join(task, "plan.md"))
	if err != nil || string(data) != string(installed) {
		t.Fatal("second migration changed plan")
	}
	entries, err := os.ReadDir(filepath.Join(task, "plan-revisions"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("archives: %v %v", entries, err)
	}
}

func TestPlanMigrateApprovalLocked(t *testing.T) {
	for _, name := range []string{"approval.json", "approval-ship.json"} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			task := planTask(h)
			old := planFixture(t, "legacy/form-b.md")
			h.write(filepath.Join(task, "plan.md"), old)
			h.write(filepath.Join(task, name), "incomplete JSON")
			done := make(chan int, 1)
			var early, started bool
			var code int
			err := (plan.Store{TaskDir: task}).WithHash(func(hash string) error {
				// Simulate approval installation while holding the same lock as Approve.
				started = true
				go func() { done <- h.run(task, "plan", "migrate") }()
				select {
				case code = <-done:
					early = true
				case <-time.After(100 * time.Millisecond):
				}
				return os.WriteFile(filepath.Join(task, name), []byte(`{"plan_hash":"`+hash+`"}`), 0600)
			})
			if started && !early {
				code = <-done
			}
			if err != nil {
				t.Fatal(err)
			}
			if early {
				t.Fatal("migration read approval before acquiring the plan lock")
			}
			if code != 1 || !strings.Contains(h.errb.String(), "need approving again") {
				t.Fatalf("%d: %s", code, h.errb)
			}
			data, err := os.ReadFile(filepath.Join(task, "plan.md"))
			if err != nil || string(data) != old {
				t.Fatal("approval refusal changed plan")
			}
		})
	}
}

func TestPlanLegacyInvalidHint(t *testing.T) {
	for name, text := range map[string]string{
		"unknown C field":   strings.Replace(planFixture(t, "legacy/form-c.md"), "items:", "unknown: value\nitems:", 1),
		"unknown B field":   "# Plan\n- [ ] 1 One\n  write: []\n- [ ] 2 Two\n  mystery: value\n",
		"blank A paragraph": "# Plan\n- [ ] 1 One\n  verify: inspect it\n\n  Detached paragraph.\n",
	} {
		for _, action := range []string{"items", "check", "write"} {
			t.Run(name+"/"+action, func(t *testing.T) {
				h := newHarness(t)
				task := planTask(h)
				h.write(filepath.Join(task, "plan.md"), text+"\nRecall: nothing found\n")
				args := []string{"plan", action}
				if action == "write" {
					args = append(args, "plan.md")
				}
				if code := h.run(task, args...); code != 1 {
					t.Fatalf("exit %d", code)
				}
				if strings.Contains(h.out.String()+h.errb.String(), "migrate --dry-run") {
					t.Fatalf("hint for an unconvertible plan: %s %s", h.out, h.errb)
				}
			})
		}
	}
}
