package brain

import (
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// SkillFor is the text of knowledge/skills/<name>/SKILL.md at the first level
// that holds it (project, then home), and the absolute path of that file. A name
// is a plain folder name, never a path: one with "/", "\" or "." answers false,
// as gridplugins.SkillFor does. Line endings come back as \n.
func (s Store) SkillFor(name string) (md, path string, ok bool) {
	if name == "" || strings.ContainsAny(name, "/\\.") {
		return "", "", false
	}
	for _, lv := range s.levels() {
		if text, p, found := s.sbSkillAt(lv, name); found {
			return text, p, true
		}
	}
	return "", "", false
}

// sbSkillAt reads the SKILL.md of one skill at one level, never through a symlink.
func (s Store) sbSkillAt(level, name string) (md, path string, ok bool) {
	root, err := s.scope(level, false)
	if err != nil {
		return "", "", false
	}
	defer root.Close()
	prefix, err := s.prefix(level)
	if err != nil {
		return "", "", false
	}
	rel := filepath.Join("knowledge", "skills", name, "SKILL.md")
	b, err := safefile.Read(root, rel)
	if err != nil {
		return "", "", false
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n"), filepath.Join(s.GridHome, prefix, rel), true
}
