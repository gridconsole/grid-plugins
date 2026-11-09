package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var knownCheckTypes = map[string]bool{"file_exists": true, "text_contains": true, "command_passes": true, "llm_judge": true}

// parentRef finds a ".." path segment in a shell command.
var parentRef = regexp.MustCompile(`(^|[\s'"=:/])\.\.(/|$|[\s'"])`)

// auditCase reads one case folder on its own, so a bad case is named even when LoadSuite would also refuse it.
func auditCase(dir string) []error {
	var errs []error
	for _, name := range []string{"prompt.md", "setup.sh"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || strings.TrimSpace(string(data)) == "" {
			errs = append(errs, fmt.Errorf("%s is missing or empty", name))
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "checks.json"))
	if err != nil {
		return append(errs, err)
	}
	var checks []Check
	if err := json.Unmarshal(data, &checks); err != nil {
		return append(errs, fmt.Errorf("checks.json: %w", err))
	}
	for _, c := range checks {
		if !knownCheckTypes[c.Type] {
			errs = append(errs, fmt.Errorf("check %s: unknown type %q", c.ID, c.Type))
		}
		if c.Path != "" && (!filepath.IsLocal(c.Path) || strings.Contains(c.Path, `\`)) {
			errs = append(errs, fmt.Errorf("check %s: path %q leaves the task folder", c.ID, c.Path))
		}
		for _, arg := range c.Command {
			if parentRef.MatchString(arg) {
				errs = append(errs, fmt.Errorf("check %s: command reaches outside the task folder", c.ID))
			}
		}
	}
	return errs
}

func TestEveryShippedCaseIsSound(t *testing.T) {
	root := "../../evals/tasks"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	cases := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cases++
		for _, err := range auditCase(filepath.Join(root, e.Name())) {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
	tasks, err := LoadSuite(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != cases {
		t.Fatalf("loaded %d tasks from %d case folders", len(tasks), cases)
	}
}

func TestAuditCaseRefusals(t *testing.T) {
	good := func(t *testing.T) string {
		dir := t.TempDir()
		for name, body := range map[string]string{
			"prompt.md":   "do it\n",
			"setup.sh":    "set -eu\n",
			"checks.json": `[{"id":"a","type":"file_exists","path":"out.txt"}]`,
		} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	if errs := auditCase(good(t)); len(errs) != 0 {
		t.Fatalf("good case refused: %v", errs)
	}
	for name, mutate := range map[string]func(dir string) error{
		"setup":  func(dir string) error { return os.Remove(filepath.Join(dir, "setup.sh")) },
		"prompt": func(dir string) error { return os.Remove(filepath.Join(dir, "prompt.md")) },
		"type":   func(dir string) error { return writeChecks(dir, `[{"id":"a","type":"tool_used"}]`) },
		"escape": func(dir string) error { return writeChecks(dir, `[{"id":"a","type":"file_exists","path":"../x"}]`) },
		"absolute": func(dir string) error {
			return writeChecks(dir, `[{"id":"a","type":"file_exists","path":"/etc/passwd"}]`)
		},
		"command": func(dir string) error {
			return writeChecks(dir, `[{"id":"a","type":"command_passes","command":["sh","-c","cat ../x"]}]`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			dir := good(t)
			if err := mutate(dir); err != nil {
				t.Fatal(err)
			}
			if len(auditCase(dir)) == 0 {
				t.Fatal("bad case passed")
			}
		})
	}
}

func writeChecks(dir, body string) error {
	return os.WriteFile(filepath.Join(dir, "checks.json"), []byte(body), 0o644)
}
