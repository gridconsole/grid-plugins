package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/artifact"
	"github.com/gridconsole/grid-plugins/internal/golden"
)

func artifactJSON(s string) string {
	s = regexp.MustCompile(`A-[0-9a-f]{32}`).ReplaceAllString(s, "<ID>")
	return regexp.MustCompile(`"created":"[^"]+"`).ReplaceAllString(s, `"created":"<CREATED>"`)
}

func TestArtifactGolden(t *testing.T) {
	h := newHarness(t)
	task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-01-report")
	h.mkdir(task)
	h.write(filepath.Join(task, "report.txt"), "Example report.\n")
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"artifact-import-json", []string{"artifact", "import", "report.txt", "--title", "A report", "--json"}},
		{"artifact-list-json", []string{"artifact", "list", "--json"}},
	} {
		if code := h.run(task, tc.args...); code != 0 {
			t.Fatalf("exit %d: %s", code, h.errb)
		}
		golden.Assert(t, tc.name, artifactJSON(h.norm(h.out.String())))
	}
	var records []artifact.Record
	if err := json.Unmarshal(h.out.Bytes(), &records); err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatal(records)
	}
	if code := h.run(h.home, "artifact", "publish", records[0].ID, "--task", task, "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	golden.Assert(t, "artifact-publish-json", artifactJSON(h.norm(h.out.String())))
	if code := h.run(task, "artifact", "link", "https://example.com/report", "--title", "Report link", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	golden.Assert(t, "artifact-link-json", artifactJSON(h.out.String()))
	if data, err := os.ReadFile(filepath.Join(task, "report.txt")); err != nil || string(data) != "Example report.\n" {
		t.Fatal("source changed")
	}
}

func TestArtifactCLI(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		outside bool
		code    int
		message string
	}{
		{"empty", []string{"list", "--json"}, false, 0, "[]\n"},
		{"no-task", []string{"list"}, true, 1, "--task"},
		{"http", []string{"link", "http://example.com"}, false, 1, "HTTPS"},
		{"unknown", []string{"remove"}, false, 2, "unknown"},
		{"missing-operand", []string{"import", "--json"}, false, 2, "expected"},
		{"extra-operand", []string{"list", "extra"}, false, 2, "expected"},
		{"missing-value", []string{"link", "https://example.com", "--title"}, false, 2, "needs an argument"},
		{"bad-flag", []string{"list", "--bad"}, false, 2, "not defined"},
		{"help", []string{"import", "--help"}, false, 0, "Usage"},
		{"traversal", []string{"publish", "../task.md"}, false, 1, "invalid artifact id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-01-report")
			h.mkdir(task)
			cwd := task
			if tc.outside {
				cwd = h.home
			}
			code := h.run(cwd, append([]string{"artifact"}, tc.args...)...)
			if code != tc.code || !strings.Contains(h.out.String()+h.errb.String(), tc.message) {
				t.Fatalf("exit %d, out %s, err %s", code, h.out, h.errb)
			}
		})
	}
}
