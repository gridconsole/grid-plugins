package contract

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// The questions skill asks through `grid question ask`, which posts a person Job,
// and the contract lists that verb with the same flags as `question add`.

func TestSkillQuestionsAskText(t *testing.T) {
	text := readPluginFile(t, "skills/questions/SKILL.md")
	for _, want := range []string{"grid question ask", "gate:Q-"} {
		if !strings.Contains(text, want) {
			t.Errorf("skills/questions/SKILL.md does not contain %q", want)
		}
	}
	for _, bad := range []string{"Not built", "only way work waits"} {
		if strings.Contains(text, bad) {
			t.Errorf("skills/questions/SKILL.md still contains %q", bad)
		}
	}
}

func TestSkillQuestionsAskContract(t *testing.T) {
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Args  []string
				Flags []struct {
					Name     string `json:"name"`
					Takes    string `json:"takes"`
					Required bool   `json:"required"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal([]byte(readPluginFile(t, "contract/commands.json")), &doc); err != nil {
		t.Fatal(err)
	}
	type flag struct {
		Name     string
		Takes    string
		Required bool
	}
	flags := map[string][]flag{}
	found := map[string]bool{}
	for _, c := range doc.Commands {
		if c.Name != "question" {
			continue
		}
		for _, s := range c.Subcommands {
			found[s.Name] = true
			for _, f := range s.Flags {
				flags[s.Name] = append(flags[s.Name], flag{f.Name, f.Takes, f.Required})
			}
		}
	}
	if !found["add"] {
		t.Fatal("contract/commands.json lists no `question add`")
	}
	if !found["ask"] {
		t.Fatal("contract/commands.json lists no `question ask`")
	}
	if !reflect.DeepEqual(flags["ask"], flags["add"]) {
		t.Errorf("`question ask` flags %v differ from `question add` flags %v", flags["ask"], flags["add"])
	}
}
