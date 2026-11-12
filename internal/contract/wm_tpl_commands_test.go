package contract

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// The agent templates in agent-templates/ are synced into the user's agents at
// SessionStart. Each backticked `grid <group> <sub> ...` in them is a command
// line an agent runs, so each must name a command and flags the contract holds.

// wmTplFlags reads contract/commands.json into "group sub" -> flag set.
func wmTplFlags(t *testing.T) map[string]map[string]bool {
	t.Helper()
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	flags := map[string]map[string]bool{}
	for _, c := range doc.Commands {
		for _, s := range c.Subcommands {
			set := map[string]bool{}
			for _, f := range s.Flags {
				set[f.Name] = true
			}
			flags[c.Name+" "+s.Name] = set
		}
	}
	return flags
}

// wmTplCheck returns one error for each backticked grid span of text whose command
// or flag the contract lacks.
func wmTplCheck(flags map[string]map[string]bool, text string) []string {
	var errs []string
	for _, m := range codeSpanPattern.FindAllStringSubmatch(text, -1) {
		span := m[1]
		c := gridCmdPattern.FindStringSubmatch(span)
		if c == nil {
			continue
		}
		key := c[1] + " " + c[2]
		set, ok := flags[key]
		if !ok {
			errs = append(errs, "`"+key+"` is not in contract/commands.json")
			continue
		}
		for _, f := range flagPattern.FindAllString(span, -1) {
			if !set[f] {
				errs = append(errs, "`grid "+key+"` has no flag "+f+" in contract/commands.json")
			}
		}
	}
	return errs
}

func TestWmTplCommandsExist(t *testing.T) {
	flags := wmTplFlags(t)
	t.Run("a flag the command lacks is one error", func(t *testing.T) {
		errs := wmTplCheck(flags, "`grid job post --task <task> --no-such-flag`")
		if len(errs) != 1 {
			t.Fatalf("want one error, got %d: %v", len(errs), errs)
		}
	})
	t.Run("a command the contract lacks is one error", func(t *testing.T) {
		if errs := wmTplCheck(flags, "`grid job no-such-sub --task <task>`"); len(errs) != 1 {
			t.Fatalf("want one error, got %d: %v", len(errs), errs)
		}
	})
	t.Run("a good line is clean", func(t *testing.T) {
		if errs := wmTplCheck(flags, "`grid job post --task <task> --type job --body <json>`"); len(errs) != 0 {
			t.Fatalf("want no error, got %v", errs)
		}
	})
	files, err := filepath.Glob(filepath.Join("..", "..", "agent-templates", "grid-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(files)
	if len(files) < 10 {
		t.Fatalf("agent-templates/grid-*.md matches %d files, want at least 10: %v", len(files), files)
	}
	spans := 0
	for _, file := range files {
		rel := "agent-templates/" + filepath.Base(file)
		text := readPluginFile(t, rel)
		for _, m := range codeSpanPattern.FindAllStringSubmatch(text, -1) {
			if gridCmdPattern.MatchString(m[1]) {
				spans++
			}
		}
		for _, e := range wmTplCheck(flags, text) {
			t.Errorf("%s: %s", rel, e)
		}
	}
	if spans == 0 {
		t.Fatal("no grid command span in agent-templates")
	}
}

// Each template that names `grid proof add` gives --task: the proof goes to the
// Job's task, and the command runs outside the task folder.
func TestWmTplProofAddNamesTheTask(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "agent-templates", "grid-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		rel := "agent-templates/" + filepath.Base(file)
		for _, m := range codeSpanPattern.FindAllStringSubmatch(readPluginFile(t, rel), -1) {
			if strings.HasPrefix(m[1], "grid proof add ") && !strings.Contains(m[1], "--task <task>") {
				t.Errorf("%s: `%s` has no --task <task>", rel, m[1])
			}
		}
	}
}
