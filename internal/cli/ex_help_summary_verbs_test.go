package cli

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// exHelpMissingVerbs returns, per built command in contract/commands.json, the
// verbs (first word of each non-empty subcommand name) its help summary leaves out.
// A verb counts as named when it is a whole word of the summary, a plural s allowed.
func exHelpMissingVerbs(t *testing.T) map[string][]string {
	t.Helper()
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Built       bool   `json:"built"`
			Subcommands []struct {
				Name string `json:"name"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatalf("contract/commands.json: %v", err)
	}
	summaries := map[string]string{}
	for _, c := range commands() {
		summaries[c.name] = c.summary
	}
	missing := map[string][]string{}
	for _, c := range doc.Commands {
		if !c.Built {
			continue
		}
		summary, ok := summaries[c.Name]
		if !ok {
			t.Errorf("%s: built in commands.json but not in the command table", c.Name)
			continue
		}
		seen := map[string]bool{}
		var verbs, absent []string
		for _, s := range c.Subcommands {
			fields := strings.Fields(s.Name)
			if len(fields) == 0 || seen[fields[0]] {
				continue
			}
			seen[fields[0]] = true
			verbs = append(verbs, fields[0])
			re := regexp.MustCompile(`(^|[^A-Za-z0-9-])` + regexp.QuoteMeta(fields[0]) + `s?($|[^A-Za-z0-9-])`)
			if !re.MatchString(summary) {
				absent = append(absent, fields[0])
			}
		}
		// A group whose verbs cannot all fit on one help line, even with the
		// shortest separator, may end its summary with a pointer to `grid <group> help`.
		if len(absent) > 0 && strings.HasSuffix(summary, "(grid "+c.Name+" help lists all)") && !exHelpVerbsFit(verbs) {
			continue
		}
		if len(absent) > 0 {
			missing[c.Name] = absent
		}
	}
	return missing
}

// exHelpLineLimit is the width a `grid help` command line must stay under.
const exHelpLineLimit = 120

// exHelpVerbsFit reports whether the verbs, joined by a one-column separator, fit on
// one `grid help` line after the prefix of two spaces, the padded name and two spaces.
func exHelpVerbsFit(verbs []string) bool {
	width := 0
	for _, c := range commands() {
		width = max(width, len(c.name))
	}
	n := 2 + width + 2 + max(len(verbs)-1, 0)
	for _, v := range verbs {
		n += len(v)
	}
	return n < exHelpLineLimit
}

// TestExHelpSummariesNameEveryVerb requires each built command's `grid help` line to
// name every verb the command has in contract/commands.json. A group whose verbs
// cannot all fit on one line may name the most used and end with "(grid <group> help lists all)".
func TestExHelpSummariesNameEveryVerb(t *testing.T) {
	for name, verbs := range exHelpMissingVerbs(t) {
		t.Errorf("grid help: %s leaves out verbs %v", name, verbs)
	}
}

// TestExHelpSetupSummaryNamesSettingsAndSandbox pins the setup line to what
// `grid setup` does beyond recording answers.
func TestExHelpSetupSummaryNamesSettingsAndSandbox(t *testing.T) {
	for _, c := range commands() {
		if c.name != "setup" {
			continue
		}
		for _, want := range []string{"Claude Code settings", "--sandbox"} {
			if !strings.Contains(c.summary, want) {
				t.Errorf("setup summary %q lacks %q", c.summary, want)
			}
		}
		return
	}
	t.Fatal("no setup command in the table")
}

// TestExHelpLinesFitOneLine runs `grid help` through the harness and requires every
// command line to stay under 120 columns.
func TestExHelpLinesFitOneLine(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "help"); code != 0 {
		t.Fatalf("grid help: exit %d: %s", code, h.errb)
	}
	lines := strings.Split(h.out.String(), "\n")
	seen := 0
	for _, c := range commands() {
		found := false
		for _, l := range lines {
			if strings.HasPrefix(l, "  "+c.name+" ") {
				found = true
				seen++
				if n := len([]rune(l)); n >= exHelpLineLimit {
					t.Errorf("help line for %s is %d columns: %q", c.name, n, l)
				}
			}
		}
		if !found {
			t.Errorf("grid help has no line for %s", c.name)
		}
	}
	if seen == 0 {
		t.Fatal("no command lines found in grid help")
	}
}
