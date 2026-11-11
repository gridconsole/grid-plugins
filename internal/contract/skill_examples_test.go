package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// The run-job and review-job skills show one example result body each. An agent
// copies that body into `grid job result --body`, so the example has to pass the
// result schema. These tests read the shipped skill text and check it.

func readPluginFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

var jsonBlockPattern = regexp.MustCompile("(?s)```json\n(.*?)\n```")

// firstJSONBlock returns the text of the first fenced json block.
func firstJSONBlock(text string) (string, bool) {
	m := jsonBlockPattern.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// asPost wraps a result body in the envelope that `grid job result` adds, so
// contract.Validate("result", ...) checks the body against the real schema.
func asPost(t *testing.T, body string) []byte {
	t.Helper()
	var b any
	if err := json.Unmarshal([]byte(body), &b); err != nil {
		t.Fatalf("example body is not JSON: %v", err)
	}
	post := map[string]any{
		"id": "01K9J000000000000000000001", "task": "01K9J000000000000000000001",
		"type": "result", "type_v": 2,
		"author": map[string]any{"id": "worker", "role": "worker"},
		"tier":   "agent", "worker": "worker@01K9J000000000000000000002",
		"lease": "J-1", "gen": 0, "fence": 2, "seq": 1,
		"at": "2026-11-08T10:00:00.000Z", "re": nil, "after": []any{},
		"job": "J-1", "attempt": 1, "wake": nil, "body": b,
	}
	out, err := json.Marshal(post)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func exampleBlock(t *testing.T, rel string) string {
	t.Helper()
	block, ok := firstJSONBlock(readPluginFile(t, rel))
	if !ok {
		t.Fatalf("%s: no fenced json block", rel)
	}
	return block
}

func TestSkillExamplesValidate(t *testing.T) {
	for _, rel := range []string{"skills/run-job/SKILL.md", "skills/review-job/SKILL.md"} {
		t.Run(rel, func(t *testing.T) {
			block := exampleBlock(t, rel)
			if err := Validate("result", asPost(t, block)); err != nil {
				t.Fatalf("example body fails the result schema: %v", err)
			}
		})
	}
	t.Run("review-job has must_fix", func(t *testing.T) {
		var body struct {
			MustFix []map[string]any `json:"must_fix"`
		}
		if err := json.Unmarshal([]byte(exampleBlock(t, "skills/review-job/SKILL.md")), &body); err != nil {
			t.Fatal(err)
		}
		if len(body.MustFix) == 0 {
			t.Fatal("the reviewer example has an empty must_fix")
		}
	})
}

func TestSkillExampleBrokenCopyFails(t *testing.T) {
	block := exampleBlock(t, "skills/run-job/SKILL.md")
	broken := strings.Replace(block, `"state": "met"`, `"state": "ok"`, 1)
	if broken == block {
		t.Fatal(`the run-job example has no "state": "met" row to break`)
	}
	if err := Validate("result", asPost(t, broken)); err == nil {
		t.Fatal("acceptance state ok passed the schema, so the schema check is not running")
	}
}

var (
	codeSpanPattern = regexp.MustCompile("`([^`\n]+)`")
	gridCmdPattern  = regexp.MustCompile(`^grid ([a-z][a-z-]*) ([a-z][a-z-]*)`)
	flagPattern     = regexp.MustCompile(`--[a-z][a-z-]*`)
)

func TestSkillAndAgentCommandsExist(t *testing.T) {
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	flags := map[string]map[string]bool{} // "group sub" -> flag set
	for _, c := range doc.Commands {
		for _, s := range c.Subcommands {
			set := map[string]bool{}
			for _, f := range s.Flags {
				set[f.Name] = true
			}
			flags[c.Name+" "+s.Name] = set
		}
	}
	files := []string{"skills/run-job/SKILL.md", "skills/review-job/SKILL.md", "agents/builder.md", "agents/reviewer.md"}
	for _, rel := range files {
		named := map[string]bool{} // commands named in this file
		var bare []string          // flags named on their own
		for _, m := range codeSpanPattern.FindAllStringSubmatch(readPluginFile(t, rel), -1) {
			span := m[1]
			if c := gridCmdPattern.FindStringSubmatch(span); c != nil {
				key := c[1] + " " + c[2]
				set, ok := flags[key]
				if !ok {
					t.Errorf("%s: `%s` is not in contract/commands.json", rel, key)
					continue
				}
				named[key] = true
				for _, f := range flagPattern.FindAllString(span, -1) {
					if !set[f] {
						t.Errorf("%s: `grid %s` has no flag %s in contract/commands.json", rel, key, f)
					}
				}
			} else if strings.HasPrefix(span, "--") {
				bare = append(bare, flagPattern.FindString(span))
			}
		}
		for _, f := range bare {
			found := false
			for key := range named {
				found = found || flags[key][f]
			}
			if !found {
				t.Errorf("%s: flag %s is not a flag of any command the file names", rel, f)
			}
		}
	}
}

func TestRunJobSkillHoldsTheRules(t *testing.T) {
	text := readPluginFile(t, "skills/run-job/SKILL.md")
	if !strings.HasPrefix(text, "---\nname: run-job\ndescription: ") {
		t.Fatal("run-job front matter does not start with name and description")
	}
	for _, want := range []string{
		"post a result before you end, also when blocked or when nothing changed",
		"grid brain show",
		"grid brain propose",
		"--source job:",
		"supersedes",
	} {
		if !strings.Contains(strings.ToLower(text), want) {
			t.Errorf("run-job skill lacks %q", want)
		}
	}
}
