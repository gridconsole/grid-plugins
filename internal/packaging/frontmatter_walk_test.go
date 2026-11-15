package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readTemplatesAndTopics reads agent-templates/*.md (checked like agents) and
// topics/README.md plus topics/*/TOPIC.md (read for their grid commands).
func readTemplatesAndTopics(t *testing.T) (templates, docs map[string]string) {
	t.Helper()
	templates, docs = map[string]string{}, map[string]string{}
	paths, err := filepath.Glob(filepath.Join("..", "..", "agent-templates", "*.md"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no agent-templates/*.md found: %v", err)
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		templates["agent-templates/"+filepath.Base(p)] = string(data)
	}
	topicPaths, err := filepath.Glob(filepath.Join("..", "..", "topics", "*", "TOPIC.md"))
	if err != nil || len(topicPaths) == 0 {
		t.Fatalf("no topics/*/TOPIC.md found: %v", err)
	}
	topicPaths = append(topicPaths, filepath.Join("..", "..", "topics", "README.md"))
	for _, p := range topicPaths {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		rel, err := filepath.Rel(filepath.Join("..", ".."), p)
		if err != nil {
			t.Fatal(err)
		}
		docs[filepath.ToSlash(rel)] = string(data)
	}
	return templates, docs
}

// Row 17: the agent templates and the topics quote only grid commands and flags
// that commands.json has, and every template is well formed like an agent.
func TestShippedTemplatesAndTopicsAreWellFormed(t *testing.T) {
	_, _, skillDirs := readPluginFiles(t)
	templates, docs := readTemplatesAndTopics(t)
	problems, checked := checkPluginFilesWith(templates, nil, docs, skillDirs, loadContractSurface(t))
	for _, p := range problems {
		t.Error(p)
	}
	if checked == 0 {
		t.Fatal("no `grid ...` commands were found in templates or topics; the scan is broken")
	}
	t.Logf("%d templates, %d topic files, %d grid commands checked", len(templates), len(docs), checked)
}

// Row 17: a command in a fenced block, and one after a pipe, is compared with
// the contract too; a bad one is named with its file and line.
func TestPluginFileChecksReadFencesAndPipes(t *testing.T) {
	surface := loadContractSurface(t)
	skills := map[string]string{
		"skills/ok/SKILL.md": "---\nname: ok\ndescription: fine\n---\n" +
			"```text\n" +
			"grid job show <task> <job>\n" +
			"grid job show --nosuchflag\n" +
			"```\n" +
			"Run `printf '%s' \"a | b\" | grid job show --pipedflag`.\n" +
			"Run `printf x | grid job show <task> <job>`.\n",
	}
	docs := map[string]string{
		"topics/x/TOPIC.md": "- Run `grid nogroup x` first.\n```\ngrid job fetch <job>\n```\n",
	}
	problems, checked := checkPluginFilesWith(nil, skills, docs, map[string]bool{"ok": true}, surface)
	all := strings.Join(problems, "\n")
	for _, want := range []string{
		"skills/ok/SKILL.md:7: `grid job show --nosuchflag` uses flag --nosuchflag",
		"skills/ok/SKILL.md:9: `grid job show --pipedflag` uses flag --pipedflag",
		"topics/x/TOPIC.md:1: `grid nogroup x` names group \"nogroup\"",
		"topics/x/TOPIC.md:3: `grid job fetch <job>` names verb \"fetch\"",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("fixture did not produce %q; problems were:\n%s", want, all)
		}
	}
	if len(problems) != 4 {
		t.Errorf("fixture produced %d problems, want 4:\n%s", len(problems), all)
	}
	if checked != 6 {
		t.Errorf("fixture checked %d commands, want 6", checked)
	}
}
