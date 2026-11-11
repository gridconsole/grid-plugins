package plan

import (
	"os"
	"strings"
	"testing"
)

const imLintSkillPath = "../../skills/plan-jobs/SKILL.md"

// imLintRefuseTail is the error tail internal/cli/orchestrator.go:471 prints after a post.
const imLintRefuseTail = "render will refuse this Job"

func imLintSkill(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(imLintSkillPath)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

// imLintParagraphs splits text into runs of non-blank lines.
func imLintParagraphs(text string) []string {
	var out, cur []string
	for _, l := range strings.Split(text, "\n") {
		if strings.TrimSpace(l) == "" {
			if len(cur) > 0 {
				out = append(out, strings.Join(cur, "\n"))
				cur = nil
			}
			continue
		}
		cur = append(cur, l)
	}
	if len(cur) > 0 {
		out = append(out, strings.Join(cur, "\n"))
	}
	return out
}

func TestImLintSkillQuotesTheAnchorError(t *testing.T) {
	plan := "## Steps\n\n### Step 1: Update install.md\n\nWrite it.\n\n## Verify\n\ngo test ./...\n"
	_, err := Anchor(plan, "step-1")
	if err == nil {
		t.Fatal("step-1 on a ### Step 1 heading resolved; want an error")
	}
	quoted := "`" + err.Error() + "`"
	if !strings.Contains(imLintSkill(t), quoted) {
		t.Fatalf("skills/plan-jobs/SKILL.md does not quote the error Anchor prints: missing %s", quoted)
	}
}

func TestImLintSkillSaysLintBeforePost(t *testing.T) {
	words := []string{"`grid job lint", "--task", "no errors", imLintRefuseTail}
	var best []string
	bestHits := -1
	for _, p := range imLintParagraphs(imLintSkill(t)) {
		var missing []string
		for _, w := range words {
			if !strings.Contains(p, w) {
				missing = append(missing, w)
			}
		}
		if len(missing) == 0 {
			return
		}
		if hits := len(words) - len(missing); hits > bestHits {
			bestHits, best = hits, missing
		}
	}
	t.Fatalf("no paragraph of skills/plan-jobs/SKILL.md holds all of %q; the closest one is missing %q", words, best)
}
