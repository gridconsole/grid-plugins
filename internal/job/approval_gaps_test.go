package job

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func TestProjectPlanAliasesNeedApproval(t *testing.T) {
	for _, kind := range []string{"repo", "prototype"} {
		for _, link := range []string{"hardlink", "symlink"} {
			t.Run(kind+"/"+link, func(t *testing.T) {
				h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
				path := filepath.Join(h.Task, "plan.md")
				name := "copy.md"
				// The project root contains the task and both link endpoints.
				if link == "symlink" {
					name = "steps.md"
					target, err := filepath.Rel(h.GridHome, path)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(target, filepath.Join(h.GridHome, name)); err != nil {
						t.Skipf("symlinks unavailable: %v", err)
					}
				} else if err := os.Link(path, filepath.Join(h.GridHome, name)); err != nil {
					t.Fatal(err)
				}
				newFiles := func() *Files {
					return &Files{Task: h.Task, Repo: h.GridHome, Prototype: h.GridHome, Folders: []string{h.GridHome}}
				}
				f := newFiles()
				pointer := kind + ":" + name + "#L3-L4"
				if _, err := f.Resolve(pointer); err != nil {
					t.Fatal(err)
				}
				b, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				// Preserve the inode so the aliases still identify this plan.
				changed := strings.ReplaceAll(string(b), "Document the export format", "INJECTED instructions")
				if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
					t.Fatal(err)
				}
				for _, resolver := range []*Files{f, newFiles()} {
					for _, suffix := range []string{"#L3-L4", ":3-4", "#step-1", "#verify"} {
						s, err := resolver.Resolve(kind + ":" + name + suffix)
						if !errors.Is(err, ErrPlanNeedsApproval) || s.Text != "" {
							t.Errorf("%s: want approval refusal with no text; got %v", suffix, err)
						}
					}
				}
				h.Approve(t)
				for _, suffix := range []string{"#L3-L4", ":3-4", "#step-1", "#verify"} {
					want, err := (&Files{Task: h.Task}).Resolve("plan.md" + suffix)
					if err != nil {
						t.Fatal(err)
					}
					got, err := f.Resolve(kind + ":" + name + suffix)
					if err != nil || got.Text != want.Text || got.Digest != want.Digest {
						t.Errorf("%s: reapproved alias must select the current plan: %v", suffix, err)
					}
				}
			})
		}
	}
}

func TestPlanFirstLineNeedsApproval(t *testing.T) {
	for _, first := range []string{"heading", "step"} {
		t.Run(first, func(t *testing.T) {
			h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
			path := filepath.Join(h.Task, "plan.md")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := string(b)
			if first == "step" {
				text = strings.SplitN(text, "\n", 3)[2]
			}
			if err := os.WriteFile(path, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			h.Approve(t)
			if _, err := LoadPlan(h.Task, "1"); err != nil {
				t.Fatal(err)
			}
			line, rest, _ := strings.Cut(text, "\n")
			if err := os.WriteFile(path, []byte(line+" INJECTED\n"+rest), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadPlan(h.Task, "1"); !errors.Is(err, ErrPlanNeedsApproval) {
				t.Errorf("LoadPlan: want approval refusal; got %v", err)
			}
			if _, err := (&Files{Task: h.Task}).Resolve("plan.md#L1"); !errors.Is(err, ErrPlanNeedsApproval) {
				t.Errorf("line 1 pointer: want approval refusal; got %v", err)
			}
			h.Approve(t)
			if _, err := LoadPlan(h.Task, "1"); err != nil {
				t.Fatalf("reapproved first line: %v", err)
			}
		})
	}
}

func TestSourceSwapUsesIdentifiedHandle(t *testing.T) {
	for _, original := range []string{"external", "plan"} {
		t.Run(original, func(t *testing.T) {
			h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
			name := "plan.md"
			if original == "external" {
				name = "source.md"
				if err := os.WriteFile(filepath.Join(h.Task, name), []byte("Original source\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(h.Task, name)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f := &Files{Task: h.Task}
			f.afterIdentify = func() {
				if err := os.Rename(path, path+".original"); err != nil {
					t.Fatal(err)
				}
				if original == "external" {
					if err := os.Link(filepath.Join(h.Task, "plan.md"), path); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(path, []byte("INJECTED replacement\n"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			pointer := name + "#L1"
			j := Job{Intent: Intent{Owner: pointer, Item: pointer, Goal: pointer}}
			rendered, err := Render(j, RenderOptions{Resolver: f})
			if err != nil {
				if !errors.Is(err, ErrPlanNeedsApproval) {
					t.Fatal(err)
				}
				return // Refusal is also safe; replacement text must never escape.
			}
			if len(rendered.Inputs.Sources) != 1 {
				t.Fatalf("rendered %d sources, want one", len(rendered.Inputs.Sources))
			}
			s := rendered.Inputs.Sources[0]
			want, _, _ := strings.Cut(string(before), "\n")
			if s.Text != want || s.Digest != fmt.Sprintf("%x", sha256.Sum256(before)) {
				t.Errorf("path swap returned replacement bytes instead of identified snapshot")
			}
		})
	}
}

func TestApprovedCRLFPlanAnchor(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	path := filepath.Join(h.Task, "plan.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(b), "\n", "\r\n")), 0600); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	if _, err := LoadPlan(h.Task, "1"); err != nil {
		t.Fatal(err)
	}
	s, err := (&Files{Task: h.Task}).Resolve("plan.md#step-1")
	if err != nil || !strings.Contains(s.Text, "Document the export format") {
		t.Fatalf("approved CRLF snapshot must render: %v", err)
	}
}
