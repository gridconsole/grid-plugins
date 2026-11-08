package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func operandFixture(t *testing.T, verb string) (*harness, task.Task) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	f := taskFixture(t, h)
	renamed := filepath.Join(filepath.Dir(f.Path), f.ID+"-renamed")
	if err := os.Rename(f.Path, renamed); err != nil {
		t.Fatal(err)
	}
	f.Path = renamed
	if verb == "approve" || verb == "reopen" {
		action, state := "status", "waiting"
		if verb == "reopen" {
			action = "close"
		}
		if _, err := taskWriter(h.env).Change(task.WriteOptions{ID: f.ID, Action: action, State: state}); err != nil {
			t.Fatal(err)
		}
	}
	h.write(filepath.Join(f.Path, "plan.md"), approvedPlan)
	h.write(filepath.Join(f.Path, "refusals", "R-1.json"), `{"id":"R-1","command":"echo example"}`)
	return h, f
}

func operandArgs(verb, operand string) []string {
	switch verb {
	case "approve":
		return []string{"approve", operand, "--plan-hash", plan.Hash(approvedPlan), "--json"}
	case "refusal":
		return []string{"refusal", "decide", operand, "R-1", "allow", "--json"}
	case "decide":
		return []string{"decide", operand, "R-1", "allow", "--json"}
	default:
		return []string{"task", verb, operand, "--json"}
	}
}

func TestTaskOperandForms(t *testing.T) {
	for _, verb := range []string{"show", "approve", "refusal", "decide", "close", "stop", "reopen"} {
		t.Run(verb, func(t *testing.T) {
			var want string
			for _, form := range []string{"folder", "id", "absolute", "relative"} {
				t.Run(form, func(t *testing.T) {
					h, f := operandFixture(t, verb)
					operand := filepath.Base(f.Path)
					switch form {
					case "id":
						operand = f.ID
					case "absolute":
						operand = f.Path
					case "relative":
						var err error
						operand, err = filepath.Rel(h.home, f.Path)
						if err != nil {
							t.Fatal(err)
						}
					}
					if code := h.run(h.home, operandArgs(verb, operand)...); code != 0 {
						t.Fatalf("%s %s: exit %d, want 0: %s", verb, form, code, h.norm(h.errb.String()))
					}
					got := taskWriteTime.ReplaceAllString(h.norm(h.out.String()), "<TIME>")
					if want == "" {
						want = got
					} else if got != want {
						t.Errorf("%s output differs from folder form:\n%s\nwant:\n%s", form, got, want)
					}
					var result map[string]any
					if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
						t.Fatal(err)
					}
					idKey := "task"
					if verb == "show" {
						idKey = "id"
					}
					if result[idKey] != f.ID || result["folder"] != filepath.Base(f.Path) {
						t.Errorf("JSON must name resolved id and folder: %s", got)
					}
				})
			}
		})
	}
}

func TestTaskOperandEscapes(t *testing.T) {
	for _, verb := range []string{"show", "approve", "refusal", "decide", "close", "stop", "reopen"} {
		for _, form := range []string{"outside", "symlink", "parent", "embedded-parent"} {
			t.Run(verb+"/"+form, func(t *testing.T) {
				h, f := operandFixture(t, verb)
				before, err := os.ReadFile(filepath.Join(f.Path, "task.md"))
				if err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(h.root, filepath.Base(f.Path))
				h.mkdir(outside)
				h.write(filepath.Join(outside, "task.md"), string(before))
				h.write(filepath.Join(outside, "intent.md"), task.IntentHead+f.Intent)
				operand := outside
				switch form {
				case "symlink":
					operand = filepath.Join(filepath.Dir(f.Path), "T-99-escape")
					if err := os.Symlink(outside, operand); err != nil {
						t.Fatal(err)
					}
				case "parent":
					operand = ".."
				case "embedded-parent":
					operand = f.Path + string(filepath.Separator) + ".." + string(filepath.Separator) + filepath.Base(f.Path)
				}
				if code := h.run(h.home, operandArgs(verb, operand)...); code != 2 {
					t.Errorf("%s %s: exit %d, want 2: %s", verb, form, code, h.norm(h.errb.String()))
				}
				after, err := os.ReadFile(filepath.Join(f.Path, "task.md"))
				if err != nil || string(after) != string(before) {
					t.Fatal("rejected operand changed task.md")
				}
				if h.out.Len() != 0 || strings.TrimSpace(h.errb.String()) == "" {
					t.Fatal("expected only a refusal diagnostic")
				}
			})
		}
	}
}
