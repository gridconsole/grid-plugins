package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/mdfile"
	"github.com/gridconsole/grid-plugins/internal/safefile"
)

// TopicsDir is the one place a topic is defined: topics/<topic>/TOPIC.md, under
// the plugin folder (the topics it ships) or under a brain level (a person's
// or a project's own). The older knowledge/<topic>/TOPIC.md is read only when
// the topics/ file is absent; nothing writes it.
const TopicsDir = "topics"

// TopicFile is the path of a topic definition below a root.
func TopicFile(name string) string { return TopicsDir + "/" + name + "/TOPIC.md" }

func readTopicFile(root *os.Root, name string) ([]byte, error) {
	b, err := safefile.Read(root, TopicFile(name))
	if errors.Is(err, fs.ErrNotExist) {
		return safefile.Read(root, "knowledge/"+name+"/TOPIC.md")
	}
	return b, err
}

// Brief limits keep an injected topic to a few lines: the summary, then
// pointers with their why.
const (
	briefPointers  = 6
	briefLineRunes = 240
)

// TopicBrief is the short text a hook puts in front of an agent for a topic:
// the topic's summary and its pointers, each with why it matters.
// The summary is the description in the front matter, else the first plain
// line of the body. A pointer is a body line that starts with "- " or "* ".
func TopicBrief(root *os.Root, name string) (string, error) {
	if !ValidName(name) {
		return "", fmt.Errorf("topic must be a plain name")
	}
	b, err := readTopicFile(root, name)
	if err != nil {
		return "", err
	}
	front, body, err := mdfile.Split(b)
	if err != nil {
		return "", err
	}
	var def struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := mdfile.Load(front, &def, false); err != nil {
		return "", err
	}
	if def.Name != name {
		return "", fmt.Errorf("topic %s must declare its name", name)
	}
	summary := strings.TrimSpace(def.Description)
	var pointers []string
	for _, l := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		t := strings.TrimSpace(l)
		switch {
		case strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* "):
			if len(pointers) < briefPointers {
				pointers = append(pointers, "- "+clip(strings.TrimSpace(t[2:])))
			}
		case summary == "" && t != "" && !strings.HasPrefix(t, "#"):
			summary = t
		}
	}
	if summary == "" {
		return "", fmt.Errorf("topic %s has no summary", name)
	}
	lines := append([]string{"Brain topic " + name + ": " + clip(summary)}, pointers...)
	return strings.Join(lines, "\n"), nil
}

// TopicBriefAt finds a topic's brief in the plugin folder first (the topics the
// plugin ships), then the project level, then home. A topic found nowhere
// returns fs.ErrNotExist. pluginRoot may be empty.
func (s Store) TopicBriefAt(pluginRoot, name string) (string, error) {
	if pluginRoot != "" {
		if root, err := os.OpenRoot(pluginRoot); err == nil {
			text, err := TopicBrief(root, name)
			root.Close()
			if err == nil || !errors.Is(err, fs.ErrNotExist) {
				return text, err
			}
		}
	}
	if s.GridHome == "" {
		return "", fs.ErrNotExist
	}
	for _, level := range s.levels() {
		if s.Vault() {
			text, err := s.vaultTopicBrief(level, name)
			if err == nil || !errors.Is(err, fs.ErrNotExist) {
				return text, err
			}
			continue
		}
		root, err := s.scope(level, false)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		text, err := TopicBrief(root, name)
		root.Close()
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return text, err
		}
	}
	return "", fs.ErrNotExist
}

// vaultTopicBrief is TopicBrief for a vault: the topic is the top-level folder
// <name>, its definition <name>/TOPIC.md, read as any note is (no front matter
// needed, no name key). The summary is the description or the first paragraph.
func (s Store) vaultTopicBrief(level, name string) (string, error) {
	if !ValidID(name) || strings.Contains(name, "/") || strings.HasPrefix(name, ".") || name == "Proposals" {
		return "", fs.ErrNotExist
	}
	root, _, err := s.vaultLevel(level)
	if err != nil {
		return "", err
	}
	defer root.Close()
	b, err := safefile.Read(root, name+"/TOPIC.md")
	if err != nil {
		return "", err
	}
	note, err := ParseVault(b)
	if err != nil {
		return "", err
	}
	summary := note.Description
	if summary == "" {
		summary = firstParagraph(note.Body)
	}
	if summary == "" {
		return "", fmt.Errorf("topic %s has no summary", name)
	}
	var pointers []string
	textLines(note.Body, func(l string) {
		if t := strings.TrimSpace(l); (strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ")) && len(pointers) < briefPointers {
			pointers = append(pointers, "- "+clip(strings.TrimSpace(t[2:])))
		}
	})
	return strings.Join(append([]string{"Brain topic " + name + ": " + clip(summary)}, pointers...), "\n"), nil
}

func clip(s string) string {
	r := []rune(s)
	if len(r) <= briefLineRunes {
		return s
	}
	return string(r[:briefLineRunes-1]) + "…"
}
