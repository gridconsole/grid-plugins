package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// The curator and rebase Jobs Grid posts point at the task's request, which exists
// from the start; task.md holds only frontmatter until delivery, with no Outcome heading.
func TestPostedJobsPointAtSomethingThatExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("---\nid: T-01\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "intent.md"), []byte(task.IntentHead+"Do it.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflict := &mergeConflictError{Branch: "lane", Target: "main", Files: []string{"a.txt"}}
	for name, body := range map[string]map[string]any{
		"curator": curatorJobBody("J-1", "T-01", []string{"first-fact"}),
		"rebase":  rebaseJobBody("J-2", conflict, map[string]any{"kind": "file-exists", "path": "a.txt"}),
	} {
		t.Run(name, func(t *testing.T) {
			data, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range lint.CheckJSON(data, lint.Options{Resolver: &job.Files{Task: dir}}) {
				if f.Rule == "job.intent" || f.Rule == "job.pointer" {
					t.Errorf("finding %+v", f)
				}
			}
		})
	}
}
