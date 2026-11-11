package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gbDocsNotVerbs are words after "`grid " in the READMEs that are not commands.
var gbDocsNotVerbs = map[string]string{
	// help is handled before the command table.
	"help": "grid help is answered before the commands() table",
	// home and on are doctor check labels ("grid home", "grid on PATH"), not verbs.
	"home": "doctor check label in contract/README.md, not a verb",
	"on":   "doctor check label (grid on PATH) in contract/README.md, not a verb",
}

var gbDocsVerbRe = regexp.MustCompile("`grid ([a-z][a-z0-9-]*)")
var gbDocsSchemaRe = regexp.MustCompile(`\(schemas/([^)\s]+)\)`)

func gbDocsRead(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func gbDocsCheckVerbs(t *testing.T, file, text string) {
	t.Helper()
	for _, m := range gbDocsVerbRe.FindAllStringSubmatch(text, -1) {
		w := m[1]
		if _, ok := gbDocsNotVerbs[w]; ok || Implemented(w) {
			continue
		}
		t.Errorf("%s names `grid %s`, which is not an Implemented command", file, w)
	}
}

func TestGbDocsVerbsAreImplemented(t *testing.T) {
	readme := gbDocsRead(t, "../../README.md")
	i := strings.Index(readme, "## Commands")
	if i < 0 {
		t.Fatal("README.md has no ## Commands section")
	}
	sec := readme[i+len("## Commands"):]
	if j := strings.Index(sec, "\n## "); j >= 0 {
		sec = sec[:j]
	}
	gbDocsCheckVerbs(t, "README.md", sec)
	gbDocsCheckVerbs(t, "contract/README.md", gbDocsRead(t, "../../contract/README.md"))
}

func TestGbDocsSchemaLinksExist(t *testing.T) {
	text := gbDocsRead(t, "../../contract/README.md")
	for _, m := range gbDocsSchemaRe.FindAllStringSubmatch(text, -1) {
		p := filepath.Join("..", "..", "contract", "schemas", filepath.FromSlash(m[1]))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("contract/README.md links schemas/%s, which is missing", m[1])
		}
	}
}
