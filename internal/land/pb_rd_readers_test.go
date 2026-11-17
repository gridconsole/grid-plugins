package land

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

const (
	pbRdDeploy   = "---\nlands:\n  target: local\n  branch: main\n  command: printf landed > outcome\n  needs-approve: true\n---\n"
	pbRdWorkflow = "---\nland-approval: none\n---\n# Workflows\n"
)

// pbRdStore is a land Store whose user home carries a Grid config naming vault as
// the brain folder of gridHome ("" writes no config: the key is absent).
func pbRdStore(t *testing.T, gridHome, vault string) Store {
	t.Helper()
	userHome := t.TempDir()
	none := func(string) string { return "" }
	if vault != "" {
		cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
		if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, none, userHome), cfg); err != nil {
			t.Fatal(err)
		}
	}
	return Store{Resolver: where.Resolver{GridHome: gridHome, Home: userHome}, Getenv: none}
}

// TestPbRdReaders walks the land rows of the reader table (brain-obsidian.md 2.2,
// land.go and approval.go): the deploy rule and the workflow gate are read from
// projects/<p>/grid/ of the brain folder when the config names one, and from
// projects/<p>/knowledge/ of the Grid home when the key is absent. Each layout
// holds the other's file as a decoy that must stay unread.
func TestPbRdReaders(t *testing.T) {
	t.Run("key absent", func(t *testing.T) {
		home, vault := t.TempDir(), t.TempDir()
		write(t, filepath.Join(home, "projects", "sample", "knowledge", "deploy", "lands.md"), pbRdDeploy)
		write(t, filepath.Join(home, "projects", "sample", "knowledge", "workflows", "quick.md"), pbRdWorkflow)
		write(t, filepath.Join(vault, "projects", "other", "grid", "deploy", "lands.md"), pbRdDeploy)
		s := pbRdStore(t, home, "")
		rule, err := s.rule("sample")
		if err != nil || rule.Target != "local" || rule.Command == "" {
			t.Fatalf("rule = %+v, %v", rule, err)
		}
		gate, err := s.gate(task.Task{Project: "sample", Workflow: "quick"})
		if err != nil || gate != "none" {
			t.Fatalf("gate = %q, %v", gate, err)
		}
	})
	t.Run("key set", func(t *testing.T) {
		home, vault := t.TempDir(), t.TempDir()
		write(t, filepath.Join(vault, "projects", "sample", "grid", "deploy", "lands.md"), pbRdDeploy)
		write(t, filepath.Join(vault, "projects", "sample", "grid", "workflows", "quick.md"), pbRdWorkflow)
		// The old folders hold other projects only, so a read there finds nothing.
		write(t, filepath.Join(home, "projects", "decoy", "knowledge", "deploy", "lands.md"), pbRdDeploy)
		s := pbRdStore(t, home, vault)
		rule, err := s.rule("sample")
		if err != nil || rule.Target != "local" || rule.Command == "" {
			t.Fatalf("rule = %+v, %v", rule, err)
		}
		gate, err := s.gate(task.Task{Project: "sample", Workflow: "quick"})
		if err != nil || gate != "none" {
			t.Fatalf("gate = %q, %v", gate, err)
		}
		// A project that has its rule only in the old folder is not read there.
		if _, err := s.rule("decoy"); !errors.Is(err, ErrNoDeployTopic) {
			t.Fatalf("rule(decoy) = %v, want ErrNoDeployTopic: the old knowledge/ folder was read", err)
		}
		if _, err := os.Stat(filepath.Join(home, "projects", "decoy", "grid")); !os.IsNotExist(err) {
			t.Fatalf("a read created grid/ in the Grid home: %v", err)
		}
	})
}
