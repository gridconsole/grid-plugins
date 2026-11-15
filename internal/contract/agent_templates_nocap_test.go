package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func nocapTemplates(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "agent-templates", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no templates found: %v", err)
	}
	return files
}

// Owner rule (10 Nov): no Grid-made guards, caps or limits. No template carries
// a maxTurns line in its front matter.
func TestTemplatesCarryNoMaxTurns(t *testing.T) {
	for _, f := range nocapTemplates(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "maxTurns") {
				t.Errorf("%s:%d carries %q", filepath.Base(f), i+1, line)
			}
		}
	}
}

// The run-job skill allows no shell redirect, so no template tells an agent to
// write output into a file made with mktemp.
func TestTemplatesNeedNoRedirectFile(t *testing.T) {
	for _, f := range nocapTemplates(t) {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "mktemp") {
			t.Errorf("%s tells the agent to use mktemp", filepath.Base(f))
		}
	}
}
