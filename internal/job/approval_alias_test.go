package job_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func TestPlanAliasesNeedApproval(t *testing.T) {
	for _, alias := range []string{"symlink", "hardlink", "case"} {
		t.Run(alias, func(t *testing.T) {
			h := jobtest.New(t, testdata())
			path := filepath.Join(h.Task, "plan.md")
			name := "steps.md"
			switch alias {
			case "symlink":
				if err := os.Symlink("plan.md", filepath.Join(h.Task, name)); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(h.Task, name)); err != nil {
					t.Fatal(err)
				}
			case "case":
				name = "PLAN.md"
				canonical, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				variant, err := os.Stat(filepath.Join(h.Task, name))
				if os.IsNotExist(err) {
					t.Skip("filesystem is case-sensitive")
				}
				if err != nil {
					t.Fatal(err)
				}
				if !os.SameFile(canonical, variant) {
					t.Fatal("case variant is a different file")
				}
			}
			pointers := []string{name + "#step-1", name + "#verify", name + "#L3-L4", "task:" + name + ":3-4"}
			refused := func(f *job.Files) {
				t.Helper()
				for _, pointer := range pointers {
					s, err := f.Resolve(pointer)
					if !errors.Is(err, job.ErrPlanNeedsApproval) || s.Text != "" {
						t.Errorf("%s: want approval refusal with no text; got %v, text %q", pointer, err, s.Text)
					}
				}
			}
			if err := os.Remove(filepath.Join(h.Task, "approval.json")); err != nil {
				t.Fatal(err)
			}
			f := &job.Files{Task: h.Task}
			refused(f)
			h.Approve(t)
			canonical := &job.Files{Task: h.Task}
			for i, pointer := range pointers {
				wantPointer := []string{"plan.md#step-1", "plan.md#verify", "plan.md#L3-L4", "task:plan.md:3-4"}[i]
				want, err := canonical.Resolve(wantPointer)
				if err != nil {
					t.Fatal(err)
				}
				got, err := f.Resolve(pointer)
				if err != nil || got.Text != want.Text || got.Digest != want.Digest {
					t.Errorf("%s: approved alias must select the canonical plan text: %v", pointer, err)
				}
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Write in place so the hard link still names the edited plan.
			if err := os.WriteFile(path, append(b, []byte("\nUnreviewed instructions.\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			refused(f)
			refused(&job.Files{Task: h.Task})
		})
	}
}
