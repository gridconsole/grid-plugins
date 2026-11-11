package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every skill the catalog lists ships a SKILL.md whose frontmatter names it.
func TestEveryCatalogSkillHasSkillFile(t *testing.T) {
	names := SkillNames()
	if len(names) == 0 {
		t.Fatal("the catalog lists no skills")
	}
	for _, name := range names {
		path := filepath.Join("..", "..", "skills", name, "SKILL.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("catalog skill %s has no SKILL.md: %v", name, err)
			continue
		}
		want := "---\nname: " + name + "\ndescription: "
		if !strings.HasPrefix(string(data), want) {
			t.Errorf("%s does not start with %q", path, want)
		}
	}
}
