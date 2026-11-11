package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The catalog lists task, setup and signals; each must ship a SKILL.md whose
// frontmatter name is the catalog name and whose description says when it runs.
func TestCatalogSkillsHaveFiles(t *testing.T) {
	for _, name := range []string{"task", "setup", "signals"} {
		found := false
		for _, listed := range SkillNames() {
			found = found || listed == name
		}
		if !found {
			t.Fatalf("%s is not in the skill catalog", name)
		}
		data, err := os.ReadFile(filepath.Join("..", "..", "skills", name, "SKILL.md"))
		if err != nil {
			t.Fatalf("skill %s has no file: %v", name, err)
		}
		text := string(data)
		if !strings.HasPrefix(text, "---\nname: "+name+"\ndescription: ") {
			t.Errorf("skills/%s/SKILL.md must start with name %q and a description", name, name)
		}
		if strings.Count(text, "\n---\n") < 1 {
			t.Errorf("skills/%s/SKILL.md frontmatter is not closed", name)
		}
	}
}
