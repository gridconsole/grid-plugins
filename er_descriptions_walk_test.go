package gridplugins

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// erWalkFiles lists every shipped skill, agent and agent template file.
func erWalkFiles(t *testing.T) []string {
	t.Helper()
	var all []string
	for _, pat := range []string{"skills/*/SKILL.md", "agents/*.md", "agent-templates/*.md"} {
		got, err := filepath.Glob(pat)
		if err != nil || len(got) == 0 {
			t.Fatalf("no files match %s: %v", pat, err)
		}
		all = append(all, got...)
	}
	return all
}

// TestErDescWalkEveryFile holds the description rule on every shipped file, not
// on a list: at most two sentences, the last starting with "Use ".
func TestErDescWalkEveryFile(t *testing.T) {
	files := erWalkFiles(t)
	if len(files) < 29 {
		t.Fatalf("walked %d files, want at least the 29 that ship", len(files))
	}
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		desc, found := "", false
		for _, line := range strings.Split(string(data), "\n") {
			if v, ok := strings.CutPrefix(line, "description:"); ok {
				desc, found = strings.TrimSpace(v), true
				break
			}
		}
		if !found {
			t.Errorf("%s: no description line", p)
			continue
		}
		s := erDescSentences(desc)
		if len(s) > 2 {
			t.Errorf("%s: %d sentences, want at most 2: %s", p, len(s), desc)
		}
		if len(s) == 0 || !strings.HasPrefix(s[len(s)-1], "Use ") {
			t.Errorf("%s: last sentence does not start with \"Use \": %s", p, desc)
		}
	}
}

// The curator template keeps the meaning "asks no person" in its first sentence.
func TestErDescCuratorTemplateKeepsAsksNoPerson(t *testing.T) {
	d := strings.ToLower(erDescText(t, erDescCase{name: "grid-curator"}))
	if !strings.Contains(d, "no person") {
		t.Errorf("grid-curator description lost \"no person\": %s", d)
	}
}
