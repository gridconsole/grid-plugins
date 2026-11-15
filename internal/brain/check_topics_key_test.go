package brain

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// topics/README.md documents `topics: <topic>` in a skill's front matter; it
// names a topic and starts nothing, so a skill that carries it is clean.
func TestCheckSkillTopicsKeyIsClean(t *testing.T) {
	s := chkStore(t)
	chkSkill(t, s, "name: demo\ndescription: d\ntopics: delivering\n", "# Demo\n")
	r, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	chkWant(t, r)
}

var topKey = regexp.MustCompile(`^([a-z_][a-z0-9_-]*):`)

// Every key in the front matter of a shipped skill is one the check allows, so
// a shipped skill filed in the brain is not a finding.
func TestShippedSkillKeysAreAllowedKeys(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "skills", "*", "SKILL.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no skills found: %v", err)
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 || lines[0] != "---" {
			t.Fatalf("%s does not open with ---", p)
		}
		for _, l := range lines[1:] {
			if l == "---" {
				break
			}
			if m := topKey.FindStringSubmatch(l); m != nil && !skillFields[m[1]] && !runningSkillFields[m[1]] {
				t.Errorf("%s: front matter key %q is outside the keys the brain check allows", p, m[1])
			}
			if m := topKey.FindStringSubmatch(l); m != nil && runningSkillFields[m[1]] {
				t.Errorf("%s: front matter key %q can run code", p, m[1])
			}
		}
	}
}
