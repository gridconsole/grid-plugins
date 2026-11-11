package contract

import (
	"strings"
	"testing"
)

// A builder under a Job lease cannot use feed_post, so a security problem it
// meets goes out as a `found` record with `grid job post`, and the id goes in
// the result's `found`. These tests read the shipped skill and agent text.

const (
	kbRjSkill = "skills/run-job/SKILL.md"
	kbRjAgent = "agents/builder.md"
)

// kbRjParagraphs splits a file into paragraphs with LF line ends.
func kbRjParagraphs(t *testing.T, rel string) []string {
	t.Helper()
	text := strings.ReplaceAll(readPluginFile(t, rel), "\r\n", "\n")
	return strings.Split(text, "\n\n")
}

// kbRjFoundSpan reports whether paragraph holds a code span that starts with
// `grid job post` and carries --type found and --job <job>.
func kbRjFoundSpan(paragraph string) bool {
	for _, m := range codeSpanPattern.FindAllStringSubmatch(paragraph, -1) {
		span := m[1]
		if strings.HasPrefix(span, "grid job post") &&
			strings.Contains(span, "--type found") &&
			strings.Contains(span, "--job <job>") {
			return true
		}
	}
	return false
}

func TestKbRjSkillPostsSecurityFound(t *testing.T) {
	for _, paragraph := range kbRjParagraphs(t, kbRjSkill) {
		if kbRjFoundSpan(paragraph) &&
			strings.Contains(paragraph, "security") &&
			strings.Contains(paragraph, "`found`") {
			return
		}
	}
	t.Fatalf("%s: no paragraph holds a `grid job post ... --type found ... --job <job>` span and also names security and the result's `found`", kbRjSkill)
}

func TestKbRjBuilderAgentPointsAtFound(t *testing.T) {
	paragraphs := kbRjParagraphs(t, kbRjAgent)
	hasSpan := false
	for _, paragraph := range paragraphs {
		if kbRjFoundSpan(paragraph) {
			hasSpan = true
		}
		if strings.Contains(paragraph, "a finding") && strings.Contains(paragraph, "feed_post") {
			t.Errorf("%s: a paragraph still sends a finding to feed_post: %q", kbRjAgent, paragraph)
		}
	}
	if !hasSpan {
		t.Fatalf("%s: no `grid job post ... --type found ... --job <job>` span", kbRjAgent)
	}
}
