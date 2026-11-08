package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func operandFixture(t *testing.T, verb string) (*harness, task.Task) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.env.Machine = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
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

func TestTaskOperandPlainOutput(t *testing.T) {
	for _, verb := range []string{"show", "approve", "refusal", "decide", "close", "stop", "reopen"} {
		t.Run(verb, func(t *testing.T) {
			var want string
			for _, folder := range []bool{false, true} {
				h, f := operandFixture(t, verb)
				operand := f.ID
				if folder {
					operand = filepath.Base(f.Path)
				}
				args := operandArgs(verb, operand)
				if code := h.run(h.home, args[:len(args)-1]...); code != 0 {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				got := taskWriteTime.ReplaceAllString(h.norm(h.out.String()), "<TIME>")
				if folder && got != want {
					t.Errorf("folder output %q differs from id output %q", got, want)
				}
				want = got
			}
		})
	}
}

func TestTaskOperandPolicyAndExactFolder(t *testing.T) {
	h, personal := operandFixture(t, "show")
	projectDir := filepath.Join(h.root, "example-project")
	h.mkdir(projectDir)
	shared, err := taskWriter(h.env).New(task.NewOptions{Cwd: projectDir, Words: "Shared task"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(h.gridHome, "projects", shared.Project, "project-roles.json"), `{"people":[{"id":"example-member","perm":"Client"}]}`)
	h.env.Cwd = h.home
	for _, role := range []string{"owner", "member", "guest"} {
		h.env.Getenv = func(key string) string {
			switch key {
			case "GRID_ROLE":
				return role
			case "GRID_MEMBER":
				return "example-member"
			}
			return ""
		}
		for _, f := range []task.Task{personal, shared} {
			for _, operand := range []string{f.ID, filepath.Base(f.Path), f.Path} {
				got, err := resolveTaskOperand(h.env, operand)
				allowed := f.ID == shared.ID || role == "owner"
				if allowed && (err != nil || got.ID != f.ID || got.Path != f.Path) {
					t.Errorf("%s resolving %s: %+v, %v", role, f.ID, got, err)
				}
				if !allowed && err == nil {
					t.Errorf("%s resolved Personal through %s", role, operand)
				}
			}
		}
	}
	h.env.Getenv = func(string) string { return "" }
	for _, operand := range []string{personal.ID + "-rename", personal.ID + "-stale", personal.Path + "/artifacts"} {
		if _, err := resolveTaskOperand(h.env, operand); err == nil {
			t.Errorf("accepted inexact folder %q", operand)
		}
	}
	link := filepath.Join(filepath.Dir(shared.Path), "T-99-link")
	if err := os.Symlink(personal.Path, link); err != nil {
		t.Fatal(err)
	}
	for _, operand := range []string{link, filepath.Base(link)} {
		if _, err := resolveTaskOperand(h.env, operand); !errors.Is(err, errTaskOperand) {
			t.Errorf("cross-project link: %v", err)
		}
	}
}

func TestTaskOperandHelp(t *testing.T) {
	for _, command := range []string{"approve", "refusal", "decide"} {
		t.Run(command, func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.home, command, "--help"); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, command+"-help", h.out.String()+h.errb.String())
		})
	}
}

func TestOtherTaskVerbFolderOperands(t *testing.T) {
	for _, tc := range []struct {
		verb string
		rest []string
	}{
		{"status", []string{"waiting"}},
		{"history", nil},
		{"rename", []string{"--title", "A new title"}},
		{"decline", []string{"--reason", "No longer needed"}},
		{"attach", []string{"--session", "example-session"}},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			h, f := operandFixture(t, tc.verb)
			args := append([]string{"task", tc.verb, filepath.Base(f.Path)}, tc.rest...)
			if code := h.run(h.home, append(args, "--json")...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
		})
	}
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
