package gridplugins

import (
	"os"
	"strings"
	"testing"
)

type erDescCase struct {
	name   string
	skill  bool
	phrase string
}

var erDescCases = []erDescCase{
	{"brain-keeper", true, "no person"},
	{"plan-jobs", true, "the job is the brief"},
	{"setup", true, ""},
	{"signals", true, "never"},
	{"grid-builder", false, "stops and reports"},
	{"grid-checker", false, "never changes"},
	{"grid-designer", false, "does not build"},
	{"grid-library", false, "read-only"},
}

// erDescText is the description: value from the front matter of one file.
func erDescText(t *testing.T, c erDescCase) string {
	t.Helper()
	var body string
	if c.skill {
		s, ok := SkillFor(c.name)
		if !ok {
			t.Fatalf("no skill %s", c.name)
		}
		body = s
	} else {
		b, err := os.ReadFile("agent-templates/" + c.name + ".md")
		if err != nil {
			t.Fatal(err)
		}
		body = string(b)
	}
	for _, line := range strings.Split(body, "\n") {
		if v, ok := strings.CutPrefix(line, "description:"); ok {
			return strings.TrimSpace(v)
		}
	}
	t.Fatalf("%s has no description line", c.name)
	return ""
}

// erDescSentences splits on ". " and a final "."; a "." inside a word does not end one.
func erDescSentences(d string) []string {
	var out []string
	for _, p := range strings.Split(d, ". ") {
		p = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(p), "."))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func TestErDescTwoSentences(t *testing.T) {
	for _, c := range erDescCases {
		d := erDescText(t, c)
		s := erDescSentences(d)
		if len(s) > 2 {
			t.Errorf("%s: %d sentences, want at most 2: %s", c.name, len(s), d)
		}
		if len(s) == 0 || !strings.HasPrefix(s[len(s)-1], "Use ") {
			t.Errorf("%s: last sentence does not start with \"Use \": %s", c.name, d)
		}
	}
	erDescKept(t)
}

// erDescKept checks the meaning each description keeps, by phrase.
func erDescKept(t *testing.T) {
	t.Helper()
	for _, c := range erDescCases {
		d := strings.ToLower(erDescText(t, c))
		if c.phrase != "" && !strings.Contains(d, c.phrase) {
			t.Errorf("%s: description lost %q: %s", c.name, c.phrase, d)
		}
		if c.name == "setup" && strings.Contains(d, "first run in a new folder") {
			t.Errorf("setup still says \"first run in a new folder\": %s", d)
		}
	}
}
