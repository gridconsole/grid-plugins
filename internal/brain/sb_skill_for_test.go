package brain

import (
	"os"
	"path/filepath"
	"testing"
)

func sbWriteSkill(t *testing.T, dir, name, text string) string {
	t.Helper()
	p := filepath.Join(dir, "knowledge", "skills", name, "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSbSkillForOrder(t *testing.T) {
	home := t.TempDir()
	s := Store{GridHome: home, Project: "demo"}
	sbWriteSkill(t, home, "house-style", "home text\n")
	sbWriteSkill(t, home, "only-home", "only home\n")
	projPath := sbWriteSkill(t, filepath.Join(home, "projects", "demo"), "house-style", "project text\n")

	md, path, ok := s.SkillFor("house-style")
	if !ok || md != "project text\n" || path != projPath {
		t.Fatalf("want the project skill %q at %s, got %q %q %v", "project text\n", projPath, md, path, ok)
	}
	md, path, ok = s.SkillFor("only-home")
	if want := filepath.Join(home, "knowledge", "skills", "only-home", "SKILL.md"); !ok || md != "only home\n" || path != want {
		t.Fatalf("want the home skill at %s, got %q %q %v", want, md, path, ok)
	}
	if _, _, ok := (Store{GridHome: home}).SkillFor("house-style"); !ok {
		t.Fatal("a store with no project must still read the home skill")
	}
	if md, _, _ := (Store{GridHome: home}).SkillFor("house-style"); md != "home text\n" {
		t.Fatalf("a store with no project read %q, want the home text", md)
	}
	for _, name := range []string{"", "../x", "a/b", "a.b", `a\b`, "missing"} {
		if md, path, ok := s.SkillFor(name); ok || md != "" || path != "" {
			t.Errorf("SkillFor(%q) = %q %q %v, want false", name, md, path, ok)
		}
	}
}
