package cli

import (
	"encoding/json"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// ltReadmeCommandsSection returns the README.md "## Commands" section.
func ltReadmeCommandsSection(t *testing.T) string {
	t.Helper()
	readme := gbDocsRead(t, "../../README.md")
	i := strings.Index(readme, "## Commands")
	if i < 0 {
		t.Fatal("README.md has no ## Commands section")
	}
	sec := readme[i+len("## Commands"):]
	if j := strings.Index(sec, "\n## "); j >= 0 {
		sec = sec[:j]
	}
	return sec
}

// TestLtReadmeNamesEveryCommand checks the other way round from
// TestGbDocsVerbsAreImplemented: every command in contract/commands.json is
// named as `grid <name>` in README.md's ## Commands section.
func TestLtReadmeNamesEveryCommand(t *testing.T) {
	var doc struct {
		Commands []struct {
			Name string `json:"name"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatalf("contract/commands.json: %v", err)
	}
	if len(doc.Commands) == 0 {
		t.Fatal("contract/commands.json has no commands")
	}
	named := map[string]bool{}
	for _, m := range gbDocsVerbRe.FindAllStringSubmatch(ltReadmeCommandsSection(t), -1) {
		named[m[1]] = true
	}
	for _, c := range doc.Commands {
		if !named[c.Name] {
			t.Errorf("README.md ## Commands does not name `grid %s`", c.Name)
		}
	}
}
