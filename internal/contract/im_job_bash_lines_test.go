package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gridplugins "github.com/gridconsole/grid-plugins"
)

// Every Job agent preloads the skills named in its front matter. Those skills
// carry the two Bash lines that keep a Job out of Grid's sandbox refusals: one
// command per Bash call, and never dangerouslyDisableSandbox. These tests pin
// the words of both lines, not the whole sentence, in every preloaded skill.

// imBashOneCommandWords must all sit on one line of the skill. The skills write
// command substitution as `$(...)`, so the pinned word is its opening "`$(".
var imBashOneCommandWords = []string{"one command per Bash call", "`;`", "`&&`", "pipe", "`$("}

// imBashSandboxWord sits on one line together with one of imBashNeverWords.
const imBashSandboxWord = "`dangerouslyDisableSandbox`"

var imBashNeverWords = []string{"Never", "Do not"}

// imBashAgentSkills returns the `skills:` list in the front matter of an agent
// file, and false when the front matter has no `skills:` key.
func imBashAgentSkills(text string) ([]string, bool) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, false
	}
	var names []string
	found, inList := false, false
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "---" {
			break
		}
		if inList {
			item := strings.TrimSpace(line)
			if strings.HasPrefix(item, "- ") {
				names = append(names, strings.TrimSpace(strings.TrimPrefix(item, "- ")))
				continue
			}
			if item == "" {
				continue
			}
			inList = false
		}
		if rest, ok := strings.CutPrefix(line, "skills:"); ok {
			found = true
			rest = strings.Trim(strings.TrimSpace(rest), "[]")
			if rest == "" {
				inList = true
				continue
			}
			for _, n := range strings.Split(rest, ",") {
				if n = strings.TrimSpace(n); n != "" {
					names = append(names, n)
				}
			}
		}
	}
	return names, found
}

// imBashMissing returns the words of want that line lacks.
func imBashMissing(line string, want []string) []string {
	var miss []string
	for _, w := range want {
		if !strings.Contains(line, w) {
			miss = append(miss, w)
		}
	}
	return miss
}

// imBashClosest returns the words of want missing from the line that lacks the
// fewest of them; nil means one line carries them all.
func imBashClosest(text string, want []string) []string {
	best := want
	for _, line := range strings.Split(text, "\n") {
		if miss := imBashMissing(line, want); len(miss) < len(best) {
			best = miss
		}
	}
	return best
}

// imBashSandboxLine reports whether one line carries imBashSandboxWord and one of
// imBashNeverWords, and otherwise which word the closest line lacks.
func imBashSandboxLine(text string) (bool, string) {
	sawWord := false
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, imBashSandboxWord) {
			continue
		}
		sawWord = true
		for _, n := range imBashNeverWords {
			if strings.Contains(line, n) {
				return true, ""
			}
		}
	}
	if !sawWord {
		return false, imBashSandboxWord
	}
	return false, strings.Join(imBashNeverWords, " or ")
}

func TestImBashEveryJobAgentSkillCarriesTheLines(t *testing.T) {
	dir := filepath.Join("..", "..", "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	agents := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		agents++
		file := "agents/" + e.Name()
		t.Run(e.Name(), func(t *testing.T) {
			text := readPluginFile(t, file)
			names, ok := imBashAgentSkills(text)
			if !ok || len(names) == 0 {
				t.Fatalf("%s: front matter has no `skills:` list, so no skill carries the Bash lines for this agent", file)
			}
			t.Logf("%s preloads skills %v", file, names)
			for _, name := range names {
				t.Run(name, func(t *testing.T) {
					skill, found := gridplugins.SkillFor(name)
					if !found {
						t.Fatalf("%s: skill %s is not in the plugin (skills/%s/SKILL.md)", file, name, name)
					}
					if miss := imBashClosest(skill, imBashOneCommandWords); len(miss) > 0 {
						t.Errorf("%s: skill %s has no one-command Bash line: the closest line lacks %s", file, name, strings.Join(miss, ", "))
					}
					if ok, miss := imBashSandboxLine(skill); !ok {
						t.Errorf("%s: skill %s has no line forbidding %s: missing %s", file, name, imBashSandboxWord, miss)
					}
				})
			}
		})
	}
	if agents == 0 {
		t.Fatalf("%s holds no agent .md file", dir)
	}
}
