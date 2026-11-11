package hook

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// topicContext is the brain text PreToolUse hands an agent without being asked:
// the brief of each topic that the tool call names. A Skill call names topics in
// the skill's `topics` front matter; a file tool names one by a path under
// topics/<topic>/ or knowledge/<topic>/. Each topic is injected once per session
// and agent. It never decides anything, and a topic without a definition adds
// nothing.
func topicContext(o Options, in hookio.Input) string {
	if in.HookEventName != "PreToolUse" || !o.hookEnabled("brain-recall", in) {
		return ""
	}
	plugin := o.Getenv("CLAUDE_PLUGIN_ROOT")
	var names []string
	if in.ToolName == "Skill" {
		names = skillTopics(plugin, str(in.ToolInput["skill"]))
	} else {
		for _, key := range []string{"file_path", "path", "notebook_path"} {
			if p := str(in.ToolInput[key]); p != "" {
				names = append(names, pathTopics(absolute(in.Cwd, p))...)
			}
		}
	}
	if len(names) == 0 {
		return ""
	}
	store := brain.Store{GridHome: o.Resolver.GridHome}
	if dir, err := taskDir(o, in); err == nil && dir != "" {
		store = brainStore(o, dir)
	}
	var briefs []string
	done := map[string]bool{}
	for _, name := range names {
		if done[name] {
			continue
		}
		done[name] = true
		text, err := store.TopicBriefAt(plugin, name)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				o.log("topic brief", err)
			}
			continue
		}
		if in.SessionID != "" {
			first := false
			key := "topic:" + in.AgentID + ":" + name
			err := (session.Store{TaskDir: o.Resolver.GridHome}).UpdateActivity(in.SessionID, func(a *session.Activity) error {
				first = !a.Seen[key]
				a.Seen[key] = true
				return nil
			})
			if err != nil {
				o.log("topic brief", err)
				continue
			}
			if !first {
				continue
			}
		}
		briefs = append(briefs, text)
	}
	return strings.Join(briefs, "\n")
}

// skillTopics reads the topics a plugin skill declares in its front matter.
func skillTopics(plugin, skill string) []string {
	skill = strings.TrimPrefix(skill, "grid:")
	if plugin == "" || !brain.ValidName(skill) {
		return nil
	}
	b, err := os.ReadFile(filepath.Join(plugin, "skills", skill, "SKILL.md"))
	if err != nil {
		return nil
	}
	front, _, err := mdfile.Split(b)
	if err != nil {
		return nil
	}
	var def struct {
		Topics any `yaml:"topics"`
	}
	if mdfile.Load(front, &def, false) != nil {
		return nil
	}
	var names []string
	add := func(s string) {
		if s = strings.TrimSpace(s); brain.ValidName(s) {
			names = append(names, s)
		}
	}
	switch v := def.Topics.(type) {
	case string:
		for _, s := range strings.Split(v, ",") {
			add(s)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				add(s)
			}
		}
	}
	return names
}

// pathTopics names the topic a path lies in: the folder after topics/ or knowledge/.
func pathTopics(path string) []string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for i := 0; i+2 < len(parts); i++ {
		if (parts[i] == brain.TopicsDir || parts[i] == "knowledge") && brain.ValidName(parts[i+1]) {
			return []string{parts[i+1]}
		}
	}
	return nil
}
