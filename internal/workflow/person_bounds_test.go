package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPersonAndPlanPolicyBounds(t *testing.T) {
	for _, reader := range []string{"person", "plan"} {
		for _, kind := range []string{"oversize", "symlink"} {
			t.Run(reader+"/"+kind, func(t *testing.T) {
				dir := t.TempDir()
				body := "---\nperson_verbs: []\nplan_hash_token: any\n---\n"
				path := filepath.Join(dir, "workflow.md")
				if kind == "oversize" {
					body += strings.Repeat("x", 1<<20)
				} else {
					path = filepath.Join(dir, "target.md")
				}
				if err := os.WriteFile(path, []byte(body), 0600); err != nil {
					t.Fatal(err)
				}
				if kind == "symlink" {
					if err := os.Symlink("target.md", filepath.Join(dir, "workflow.md")); err != nil {
						t.Skip(err)
					}
				}
				var err error
				if reader == "person" {
					_, err = PersonVerbs(dir)
				} else {
					_, err = PlanHashToken(dir)
				}
				if err == nil {
					t.Fatal("unsafe workflow accepted")
				}
				if strings.Contains(err.Error(), dir) {
					t.Fatal("reader error exposed path")
				}
			})
		}
	}
}
