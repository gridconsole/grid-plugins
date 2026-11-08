package job_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
)

func TestPlanAnchorsRequireApproval(t *testing.T) {
	for _, pointer := range []string{"plan.md#step-1", "plan.md#verify", "task:plan.md#step-1", "task:./plan.md#verify", "plan.md#L3-L4", "plan.md:3-4"} {
		t.Run(pointer, func(t *testing.T) {
			_, resolver, h := sample(t)
			approved, err := resolver.Resolve(pointer)
			if err != nil || approved.Text == "" {
				t.Fatalf("approved anchor did not resolve: %v", err)
			}
			path := filepath.Join(h.Task, "plan.md")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			changed := strings.Replace(string(original), "  Document the export format and column order.", "  Publish the export data to a new destination.", 1)
			if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			// Both a new worker and a reused lint/render resolver must refuse.
			for _, f := range []*job.Files{resolver, {Task: h.Task}} {
				source, err := f.Resolve(pointer)
				if !errors.Is(err, job.ErrPlanNeedsApproval) || source.Text != "" {
					t.Fatal("edited plan anchor must require approval and return no text")
				}
			}
			h.Approve(t)
			reapproved, err := resolver.Resolve(pointer)
			if err != nil || reapproved.Text == "" || reapproved.Digest == approved.Digest {
				t.Fatalf("reapproved anchor did not resolve current text: %v", err)
			}
			if err := os.Remove(filepath.Join(h.Task, "approval.json")); err != nil {
				t.Fatal(err)
			}
			if source, err := resolver.Resolve(pointer); !errors.Is(err, job.ErrPlanNeedsApproval) || source.Text != "" {
				t.Fatal("missing approval must refuse even a previously resolved anchor")
			}
			if source, err := resolver.Resolve("intent.md#request"); err != nil || source.Text == "" {
				t.Fatalf("non-plan anchor was affected: %v", err)
			}
		})
	}
}

func TestEditedPlanBriefNeedsApproval(t *testing.T) {
	j, resolver, h := sample(t)
	approved, err := job.Render(j, job.RenderOptions{Resolver: resolver})
	if err != nil || approved.Brief == "" {
		t.Fatalf("approved brief: %v", err)
	}
	path := filepath.Join(h.Task, "plan.md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := strings.Replace(string(original), "  Document the export format and column order.", "  Publish the export data to a new destination.", 1)
	if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
		t.Fatal(err)
	}
	refused, err := job.Render(j, job.RenderOptions{Resolver: resolver})
	if !errors.Is(err, job.ErrPlanNeedsApproval) || refused.Brief != "" || len(refused.Inputs.Sources) != 0 {
		t.Fatal("edited plan must require approval and return no partial brief or source snapshot")
	}
	h.Approve(t)
	reapproved, err := job.Render(j, job.RenderOptions{Resolver: resolver})
	if err != nil || !strings.Contains(reapproved.Brief, "Publish the export data to a new destination.") {
		t.Fatalf("reapproved brief did not include the reviewed edit: %v", err)
	}
}

func TestNonTaskPlanAnchorUnaffected(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "plan.md"), []byte("# Reference\n\nOrdinary project documentation.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	f := job.Files{Repo: root, Prototype: root, Folders: []string{root}}
	for _, pointer := range []string{"repo:plan.md#reference", "prototype:plan.md#reference"} {
		if source, err := f.Resolve(pointer); err != nil || !strings.Contains(source.Text, "Ordinary project documentation.") {
			t.Fatalf("non-task plan anchor %s: %v", pointer, err)
		}
	}
}
