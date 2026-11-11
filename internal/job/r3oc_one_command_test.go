package job_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The words of internal/contract/im_job_bash_lines_test.go:19, all on one line.
var r3ocWords = []string{"one command per Bash call", "`;`", "`&&`", "pipe", "`$("}

var r3ocToolsLine = regexp.MustCompile(`(?m)^tools:(.*)$`)

func r3ocInstructions(brief string) string {
	_, rest, ok := strings.Cut(brief, "### Instructions\n")
	if !ok {
		return ""
	}
	if i := strings.Index(rest, "\n### "); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

func TestR3ocEveryBashAgentCarriesTheLine(t *testing.T) {
	entries, err := os.ReadDir(filepath.Join("..", "..", "agents"))
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("..", "..", "agents", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		m := r3ocToolsLine.FindStringSubmatch(string(raw))
		if m == nil || !strings.Contains(m[1], "Bash") {
			continue
		}
		seen++
		role := strings.TrimSuffix(e.Name(), ".md")
		section := r3ocInstructions(renderRole(t, role).Brief)
		if section == "" {
			t.Errorf("agents/%s: brief has no Instructions section", e.Name())
			continue
		}
		best := r3ocWords
		for _, line := range strings.Split(section, "\n") {
			var miss []string
			for _, w := range r3ocWords {
				if !strings.Contains(line, w) {
					miss = append(miss, w)
				}
			}
			if len(miss) < len(best) {
				best = miss
			}
		}
		if len(best) > 0 {
			t.Errorf("agents/%s: Instructions hold no one-command Bash line; closest line lacks %q", e.Name(), best)
		}
	}
	if seen == 0 {
		t.Fatal("no agent lists Bash in tools")
	}
}

func TestR3ocOrchestratorReadsSplit(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "agents", "orchestrator.md"))
	if err != nil {
		t.Fatal(err)
	}
	for i, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "grid task show") && strings.Contains(line, "grid plan items") {
			t.Errorf("agents/orchestrator.md:%d names both `grid task show` and `grid plan items`", i+1)
		}
	}
}
