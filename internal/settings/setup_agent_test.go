package settings

import (
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func TestRecordAgentSetupHoldsOnlyNewCommands(t *testing.T) {
	_, s := fixture(t)
	got, err := s.RecordAgentSetup([]byte(`{"test":"make test","deploys":"Deploy from this machine","deployCommand":"make deploy"}`))
	must(t, err)
	if got.Held["deployCommand"] != "make deploy" || got.Recorded["deployCommand"] != "" || got.Hash == "" {
		t.Fatalf("first record = %+v", got)
	}
	answers, pending, hash, err := s.PendingSetup()
	must(t, err)
	if answers["deployCommand"] != "" || pending["deployCommand"] != "make deploy" || hash != got.Hash {
		t.Fatalf("pending = %v %v %s", answers, pending, hash)
	}
	if _, _, err := s.ApproveSetupCommands("stale"); err == nil {
		t.Fatal("a stale hash was approved")
	}
	approved, answers, err := s.ApproveSetupCommands(got.Hash)
	must(t, err)
	if approved["deployCommand"] != "make deploy" || answers["deployCommand"] != "make deploy" || answers["test"] != "make test" {
		t.Fatalf("approve = %v %v", approved, answers)
	}
	if _, _, err := s.ApproveSetupCommands(got.Hash); err == nil {
		t.Fatal("approved twice")
	}

	// The approved command again is not a new one; nothing waits.
	got, err = s.RecordAgentSetup([]byte(`{"test":"make test","deploys":"Deploy from this machine","deployCommand":"make deploy"}`))
	must(t, err)
	if len(got.Held) != 0 || got.Hash != "" {
		t.Fatalf("an approved command was held again: %+v", got)
	}

	// Moving off local deploys takes the approved command away instead of failing.
	_, err = s.RecordAgentSetup([]byte(`{"test":"make test","deploys":"CI/CD deploys"}`))
	must(t, err)
	answers, _, _, err = s.PendingSetup()
	must(t, err)
	if answers["deployCommand"] != "" || answers["deploys"] != "CI/CD deploys" {
		t.Fatalf("after the move answers = %v", answers)
	}
}

func TestRecordAgentSetupRefusesWithoutAProject(t *testing.T) {
	personal, s := fixture(t)
	if _, err := personal.RecordAgentSetup([]byte(`{"test":"make test"}`)); err == nil {
		t.Fatal("recorded setup answers without a project")
	}
	if _, err := s.RecordAgentSetup([]byte(`{"deploys":"CI/CD deploys","deployCommand":"make deploy"}`)); err == nil {
		t.Fatal("recorded a deployCommand without local deploys")
	}
	for _, path := range []string{"projects/example/setup-answers.json", "projects/example/setup-pending.json", ".settings.lock"} {
		if _, err := os.Stat(filepath.Join(s.Home, path)); !os.IsNotExist(err) {
			t.Errorf("a refused record wrote %s", path)
		}
	}
}

// A project's setup answers and held commands are an agent's to write: the owner let
// agents approve, so setup denies neither file any more. The deny rules earlier setups
// wrote for them are retired, in the projects folder GRID_PROJECTS names and in
// <Grid home>/projects/<name>/ where the store writes both files.
func TestClaudeBlockLeavesTheSetupAnswerFilesToAgents(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	gridHome := filepath.Join(home, ".grid")
	block, err := claudeBlock(home, gridHome)
	must(t, err)
	for _, projects := range []string{"", filepath.Join(home, "work", "projects"), gridhome.Real(t.TempDir())} {
		retired, err := claudeRetired(home, gridHome, projects)
		must(t, err)
		for _, name := range []string{"setup-answers.json", "setup-pending.json"} {
			file := filepath.ToSlash(filepath.Join(gridHome, "projects", "example", name))
			for _, tool := range []string{"Edit", "Write"} {
				if claudeDenies(block, home, tool, file) {
					t.Errorf("projects folder %q: setup denies %s of %s", projects, tool, file)
				}
				if !claudeDenies(retired, home, tool, file) {
					t.Errorf("projects folder %q: no retired %s deny rule covers %s", projects, tool, file)
				}
			}
		}
	}
}

// claudeDenies reports whether a permissions.deny rule of tool matches file, reading `~/` as
// home and `//` as the filesystem root, as Claude Code anchors them.
func claudeDenies(rules []ClaudeRule, home, tool, file string) bool {
	for _, r := range rules {
		pattern, ok := strings.CutPrefix(r.Value, tool+"(")
		if r.Section != "permissions" || r.Key != "deny" || !ok {
			continue
		}
		pattern = strings.TrimSuffix(pattern, ")")
		if rest, ok := strings.CutPrefix(pattern, "~/"); ok {
			pattern = filepath.ToSlash(home) + "/" + rest
		} else {
			pattern = "/" + strings.TrimLeft(pattern, "/")
		}
		if matched, _ := path.Match(pattern, file); matched {
			return true
		}
	}
	return false
}
