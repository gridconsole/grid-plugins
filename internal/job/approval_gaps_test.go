package job

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// A symlink alias to plan.md resolves the current plan text, edited or not; no
// approval is involved.
func TestProjectPlanSymlinkAliasReadsCurrentPlan(t *testing.T) {
	for _, kind := range []string{"repo", "prototype"} {
		t.Run(kind, func(t *testing.T) {
			h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
			path := filepath.Join(h.Task, "plan.md")
			name := "steps.md"
			target, err := filepath.Rel(h.GridHome, path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, filepath.Join(h.GridHome, name)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			f := &Files{Task: h.Task, Repo: h.GridHome, Prototype: h.GridHome, Folders: []string{h.GridHome}}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.ReplaceAll(string(b), "Document the export format", "Changed instructions")
			if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			for _, suffix := range []string{"#L3-L4", ":3-4", "#step-1", "#verify"} {
				want, err := (&Files{Task: h.Task}).Resolve("plan.md" + suffix)
				if err != nil {
					t.Fatal(err)
				}
				got, err := f.Resolve(kind + ":" + name + suffix)
				if err != nil || got.Text == "" || got.Text != want.Text || got.Digest != want.Digest {
					t.Errorf("%s: alias must select the current plan: %v", suffix, err)
				}
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
				t.Fatal(err)
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
