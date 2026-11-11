package contract

import (
	"regexp"
	"strings"
	"testing"
)

// The reviewer agent runs under a Job lease, where feed_post is refused, so a
// finding has to go out as a `found` record with `grid job post --type found`.
// These tests read agents/reviewer.md and pin that route.

const kbRaFile = "agents/reviewer.md"

var (
	kbRaStepPattern = regexp.MustCompile(`^(\d+)\. `)
	kbRaFindingRe   = regexp.MustCompile(`(?i)\ba finding\b`)
)

// kbRaParagraphs splits the agent text into blank-line separated paragraphs,
// with each numbered step on its own line kept as its own paragraph.
func kbRaParagraphs(text string) []string {
	var out []string
	for _, p := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n\n") {
		for _, line := range strings.Split(p, "\n") {
			if strings.TrimSpace(line) != "" {
				out = append(out, line)
			}
		}
	}
	return out
}

// kbRaStepWith returns the number of the first numbered step that holds a code
// span passing ok, and 0 when no step does.
func kbRaStepWith(t *testing.T, text string, ok func(span string) bool) int {
	t.Helper()
	for _, line := range kbRaParagraphs(text) {
		m := kbRaStepPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		for _, s := range codeSpanPattern.FindAllStringSubmatch(line, -1) {
			if ok(s[1]) {
				n := 0
				for _, c := range m[1] {
					n = n*10 + int(c-'0')
				}
				return n
			}
		}
	}
	return 0
}

func TestKbRaAgentPostsFound(t *testing.T) {
	text := readPluginFile(t, kbRaFile)
	post := kbRaStepWith(t, text, func(s string) bool {
		return strings.HasPrefix(s, "grid job post") &&
			strings.Contains(s, "--type found") &&
			strings.Contains(s, "--job <review-job>")
	})
	if post == 0 {
		t.Fatalf("%s: no numbered step holds a code span starting `grid job post` with `--type found` and `--job <review-job>`", kbRaFile)
	}
	result := kbRaStepWith(t, text, func(s string) bool { return strings.HasPrefix(s, "grid job result") })
	if result == 0 {
		t.Fatalf("%s: no numbered step holds a `grid job result` code span", kbRaFile)
	}
	if post >= result {
		t.Errorf("%s: the found step (%d) must come before the result step (%d)", kbRaFile, post, result)
	}
}

func TestKbRaBashLineAllowsFound(t *testing.T) {
	text := readPluginFile(t, kbRaFile)
	for _, p := range kbRaParagraphs(text) {
		if !strings.Contains(p, "Use Bash only") {
			continue
		}
		if !strings.Contains(p, "`grid job post --type found`") {
			t.Fatalf("%s: the \"Use Bash only\" sentence does not name `grid job post --type found`, so the agent's own rule forbids the post: %s", kbRaFile, p)
		}
		return
	}
	t.Fatalf("%s: no sentence says \"Use Bash only\"", kbRaFile)
}

func TestKbRaFindingNotOnFeedPost(t *testing.T) {
	text := readPluginFile(t, kbRaFile)
	seen := false
	for _, p := range kbRaParagraphs(text) {
		if !strings.Contains(p, "feed_post") {
			continue
		}
		seen = true
		if kbRaFindingRe.MatchString(p) {
			t.Errorf("%s: a paragraph sends \"a finding\" to feed_post, which a Job lease refuses: %s", kbRaFile, p)
		}
		if !strings.Contains(p, "grid job result") {
			t.Errorf("%s: the paragraph that names feed_post no longer names `grid job result`: %s", kbRaFile, p)
		}
	}
	if !seen {
		t.Fatalf("%s: no paragraph names feed_post", kbRaFile)
	}
}
