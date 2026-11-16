package routine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SkillLayer is the --settings layer, as inline JSON, for an agent Grid itself
// starts: Claude Code's bundled skills off (disableBundledSkills), and each skill
// of the person's own listed by name only (skillOverrides "name-only"), so the
// skill listing holds the plugin's skills with their descriptions and only the
// names of the rest. A skill named in keep, the skills a Job or workflow step
// names, keeps its full entry. Nothing is turned off: a name-only skill can still
// be called by name. The layer exists only for that run; no settings file is
// written, so the person's own sessions are unchanged. Plugin skills are not
// affected by skillOverrides (Claude Code docs, skills page).
//
// home is the person's home: <home>/.claude/skills/<name>/SKILL.md and
// <home>/.claude/commands/<name>.md. dir is the folder the agent starts in: its
// .claude/skills/<name>/SKILL.md. A folder that cannot be read adds no names.
func SkillLayer(home, dir string, keep []string) (string, error) {
	kept := map[string]bool{}
	for _, name := range keep {
		kept[strings.TrimPrefix(name, "grid:")] = true
	}
	overrides := map[string]string{}
	add := func(name string) {
		if name != "" && !kept[name] {
			overrides[name] = "name-only"
		}
	}
	for _, root := range []string{home, dir} {
		if root == "" {
			continue
		}
		for _, name := range skillDirs(filepath.Join(root, ".claude", "skills")) {
			add(name)
		}
	}
	if home != "" {
		if entries, err := os.ReadDir(filepath.Join(home, ".claude", "commands")); err == nil {
			for _, entry := range entries {
				if name, ok := strings.CutSuffix(entry.Name(), ".md"); ok && !entry.IsDir() {
					add(name)
				}
			}
		}
	}
	layer := struct {
		DisableBundledSkills bool              `json:"disableBundledSkills"`
		SkillOverrides       map[string]string `json:"skillOverrides,omitempty"`
	}{true, overrides}
	b, err := json.Marshal(layer)
	return string(b), err
}

// skillDirs names the folders of root that hold a SKILL.md, and the front
// matter name of each when it differs from its folder.
func skillDirs(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join(root, entry.Name(), "SKILL.md"))
		if err != nil {
			continue
		}
		names = append(names, entry.Name())
		if name := frontMatterName(string(b)); name != "" && name != entry.Name() {
			names = append(names, name)
		}
	}
	return names
}

// frontMatterName is the name: value of a SKILL.md front matter, or "".
func frontMatterName(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	rest, ok := strings.CutPrefix(text, "---\n")
	if !ok {
		return ""
	}
	for _, line := range strings.Split(rest, "\n") {
		if line == "---" {
			break
		}
		if v, ok := strings.CutPrefix(line, "name:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return ""
}
