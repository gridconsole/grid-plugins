package lint_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/lint"
)

const (
	skillPath        = "../../../skills/plan-jobs/SKILL.md"
	orchestratorPath = "../../../agents/orchestrator.md"
)

func readText(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(rel))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// firstYAMLBlock returns the body of the first fenced yaml block, or "" if none.
func firstYAMLBlock(text string) string {
	var out []string
	in := false
	for _, line := range strings.Split(text, "\n") {
		trim := strings.TrimSpace(line)
		switch {
		case !in && trim == "```yaml":
			in = true
		case in && trim == "```":
			return strings.Join(out, "\n") + "\n"
		case in:
			out = append(out, line)
		}
	}
	return ""
}

// brief wraps the example's fields the way `grid job post` renders a Job: the
// run fields the CLI fills in, then the section headings the legacy reader wants.
func asBrief(block string) string {
	head := "---\n" + block + "status: briefed\nfollow_up: none\n---\n\n"
	for _, s := range []string{"Intent", "Job", "Context", "Out of scope", "Open", "Acceptance", "Outcome", "Verdict", "Follow-up"} {
		head += "## " + s + "\n\n"
	}
	return head
}

// exampleOptions builds the Resolver as lint_test.go does, plus the repo file the example reads.
func exampleOptions(t *testing.T) lint.Options {
	t.Helper()
	f := testFiles(t)
	var lines []string
	for i := 1; i <= 40; i++ {
		lines = append(lines, fmt.Sprintf("line %d of the export format", i))
	}
	if err := os.MkdirAll(filepath.Join(f.Repo, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.Repo, "docs", "export.md"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return lint.Options{Resolver: f}
}

func exampleBlock(t *testing.T) string {
	t.Helper()
	block := firstYAMLBlock(readText(t, skillPath))
	if block == "" {
		t.Fatalf("%s has no fenced yaml block", skillPath)
	}
	return block
}

func TestSkillExampleLintsClean(t *testing.T) {
	fs := lint.Check(asBrief(exampleBlock(t)), exampleOptions(t))
	if lint.HasErrors(fs) {
		t.Fatalf("example Job has error findings: %+v", fs)
	}
}

func TestSkillExampleTeachesPointers(t *testing.T) {
	j, _, err := job.DecodeBrief(asBrief(exampleBlock(t)))
	if err != nil {
		t.Fatal(err)
	}
	brainLink := false
	for _, l := range j.Links {
		if strings.HasPrefix(l.To, "brain:project/") && len(strings.TrimSpace(l.Why)) >= 8 {
			brainLink = true
		}
	}
	if !brainLink {
		t.Errorf("no links entry to brain:project/<id> with a why: %+v", j.Links)
	}
	ranged := false
	for _, p := range j.Read {
		if regexp.MustCompile(`:[0-9]+-[0-9]+$`).MatchString(p) {
			ranged = true
		}
	}
	if !ranged {
		t.Errorf("no read entry with a line range: %v", j.Read)
	}
	if len(j.After) == 0 {
		t.Error("example has no after")
	}
	if len(j.Acceptance) == 0 || j.Acceptance[0].ID != "A1" {
		t.Errorf("acceptance rows must start at A1: %+v", j.Acceptance)
	}
	if j.Verify.Command == "" && j.Verify.Kind == "" {
		t.Error("example has no verify")
	}
	intent, _ := json.Marshal(j.Intent)
	if strings.Contains(string(intent), "```") || len(intent) >= 400 {
		t.Errorf("intent must hold no fenced block and stay under 400 characters (%d): %s", len(intent), intent)
	}
}

// A copy of the example whose link why is cut to "see" must get a finding, so
// the clean result above is lint's, not an absent check.
func TestSkillExampleWithShortWhyIsFound(t *testing.T) {
	block := exampleBlock(t)
	re := regexp.MustCompile(`(?m)^(\s*(?:- )?)why:.*$`)
	cut := false
	var out []string
	for _, line := range strings.Split(block, "\n") {
		if !cut && strings.Contains(line, "why:") && !strings.Contains(line, "job:") {
			line = re.ReplaceAllString(line, `${1}why: see`)
			cut = true
		}
		out = append(out, line)
	}
	if !cut {
		t.Fatal("example has no why line to cut")
	}
	fs := lint.Check(asBrief(strings.Join(out, "\n")), exampleOptions(t))
	if !has(fs, "job.link-why") || !lint.HasErrors(fs) {
		t.Fatalf("short why not found: %+v", fs)
	}
}

// grid names every verb and subcommand in backticks; each must be in the contract.
var gridCommand = regexp.MustCompile("`grid ([a-z][a-z-]*)(?: ([a-z][a-z-]*))?")

func contractCommands(t *testing.T) map[string]map[string]bool {
	t.Helper()
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name string `json:"name"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	m := map[string]map[string]bool{}
	for _, c := range doc.Commands {
		m[c.Name] = map[string]bool{}
		for _, s := range c.Subcommands {
			if s.Name != "" {
				m[c.Name][s.Name] = true
			}
		}
	}
	return m
}

func unknownCommands(known map[string]map[string]bool, text string) []string {
	var bad []string
	for _, m := range gridCommand.FindAllStringSubmatch(text, -1) {
		subs, ok := known[m[1]]
		switch {
		case !ok:
			bad = append(bad, "grid "+m[1])
		case len(subs) > 0 && m[2] != "" && !subs[m[2]]:
			bad = append(bad, "grid "+m[1]+" "+m[2])
		case len(subs) > 0 && m[2] == "":
			bad = append(bad, "grid "+m[1]+" (no subcommand)")
		}
	}
	return bad
}

func TestSkillAndOrchestratorNameOnlyContractCommands(t *testing.T) {
	known := contractCommands(t)
	if bad := unknownCommands(known, "`grid job frobnicate` and `grid nosuch run`"); len(bad) != 2 {
		t.Fatalf("an unknown verb or subcommand must be found, got %v", bad)
	}
	for _, p := range []string{skillPath, orchestratorPath} {
		text := readText(t, p)
		if bad := unknownCommands(known, text); len(bad) > 0 {
			t.Errorf("%s names commands the contract lacks: %v", p, bad)
		}
	}
	if !strings.Contains(readText(t, skillPath), "`grid brain search") {
		t.Error("skill must tell the orchestrator to run grid brain search first")
	}
}

func TestOrchestratorNamesTheOrphanKinds(t *testing.T) {
	text := readText(t, orchestratorPath)
	for _, kind := range []string{"never_picked_up", "never_acked", "no_exit", "`grid job tree"} {
		if !strings.Contains(text, kind) {
			t.Errorf("%s does not name %s", orchestratorPath, kind)
		}
	}
}
