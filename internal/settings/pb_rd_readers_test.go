package settings

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// pbRdUser is a user home whose Grid config names vault as the brain folder of
// the workspace whose data folder is gridHome; vault "" writes no config.
func pbRdUser(t *testing.T, gridHome, vault string) (userHome string) {
	t.Helper()
	userHome = t.TempDir()
	t.Setenv("HOME", userHome)
	t.Setenv("USERPROFILE", userHome)
	t.Setenv("APPDATA", "")
	t.Setenv("GRID_HOME", "")
	if vault == "" {
		return userHome
	}
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, os.Getenv, userHome), cfg); err != nil {
		t.Fatal(err)
	}
	return userHome
}

func pbRdHas(rules []ClaudeRule, value string) bool {
	return slices.ContainsFunc(rules, func(r ClaudeRule) bool {
		return r.Section == "permissions" && r.Key == "allow" && r.Value == value
	})
}

// TestPbRdSettingsGlobCoversRoot is row 2: with the brain key set, Claude Code's
// permission rules cover <Root>/** (the brain folder itself), whether the folder
// is under the user's home (~/ anchor) or outside it (// absolute anchor); with
// the key absent the rules are the ones of the old layout and name no brain folder.
func TestPbRdSettingsGlobCoversRoot(t *testing.T) {
	t.Run("brain folder under the user home", func(t *testing.T) {
		userHome := t.TempDir()
		gridHome := filepath.Join(userHome, ".grid")
		vault := filepath.Join(userHome, "Documents", "Vault")
		pbRdUser(t, gridHome, "")
		cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
		if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, userHome), cfg); err != nil {
			t.Fatal(err)
		}
		rules, err := claudeBrainRules(userHome, gridHome, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, verb := range []string{"Read", "Edit"} {
			if want := verb + "(~/Documents/Vault/**)"; !pbRdHas(rules, want) {
				t.Errorf("claudeBrainRules lacks %s: %+v", want, rules)
			}
		}
		if len(rules) != 2 {
			t.Errorf("claudeBrainRules = %+v, want the Read and Edit allow of the brain folder only", rules)
		}
		for _, r := range rules {
			if r.Key == "deny" {
				t.Errorf("a deny rule on the brain folder: %+v", r)
			}
		}
	})
	t.Run("brain folder outside the user home", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("the // anchor of a drive path is covered by the claude_test anchors")
		}
		userHome, vault := t.TempDir(), t.TempDir()
		gridHome := filepath.Join(userHome, ".grid")
		cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
		if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, userHome), cfg); err != nil {
			t.Fatal(err)
		}
		rules, err := claudeBrainRules(userHome, gridHome, nil)
		if err != nil {
			t.Fatal(err)
		}
		glob := "//" + strings.TrimLeft(filepath.ToSlash(gridhome.Real(vault)), "/") + "/**"
		for _, verb := range []string{"Read", "Edit"} {
			if !pbRdHas(rules, verb+"("+glob+")") {
				t.Errorf("claudeBrainRules lacks %s(%s): %+v", verb, glob, rules)
			}
		}
	})
	t.Run("ClaudeBlock carries the brain rules", func(t *testing.T) {
		userHome := t.TempDir()
		gridHome := filepath.Join(userHome, ".grid")
		vault := filepath.Join(userHome, "Vault")
		pbRdUser(t, gridHome, "")
		cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
		if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, os.Getenv, userHome), cfg); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HOME", userHome)
		t.Setenv("USERPROFILE", userHome)
		rules, err := ClaudeBlock(userHome, gridHome)
		if err != nil {
			t.Fatal(err)
		}
		if !pbRdHas(rules, "Read(~/Vault/**)") || !pbRdHas(rules, "Edit(~/Vault/**)") {
			t.Errorf("ClaudeBlock lacks the brain folder allow: %+v", rules)
		}
	})
	t.Run("key absent", func(t *testing.T) {
		userHome := pbRdUser(t, "", "")
		gridHome := filepath.Join(userHome, ".grid")
		rules, err := claudeBrainRules(userHome, gridHome, nil)
		if err != nil || rules != nil {
			t.Fatalf("claudeBrainRules = %+v, %v with the key absent", rules, err)
		}
		block, err := ClaudeBlock(userHome, gridHome)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range block {
			if strings.Contains(r.Value, "Vault") || strings.Contains(r.Value, "Documents") {
				t.Errorf("a rule names a brain folder with the key absent: %+v", r)
			}
		}
		if !pbRdHas(block, "Read(~/.grid/**)") {
			t.Errorf("the Grid home allow is gone: %+v", block)
		}
	})
	t.Run("key naming the Grid home itself", func(t *testing.T) {
		userHome := t.TempDir()
		gridHome := filepath.Join(userHome, ".grid")
		pbRdUser(t, gridHome, gridHome)
		cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: gridHome}}}
		if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, userHome), cfg); err != nil {
			t.Fatal(err)
		}
		if rules, err := claudeBrainRules(userHome, gridHome, nil); err != nil || rules != nil {
			t.Fatalf("claudeBrainRules = %+v, %v; the key names the Grid home, which is the old layout", rules, err)
		}
	})
}

// TestPbRdBudgetNotePath walks the budget row of the reader table: the budget
// note is grid/budget/agent-budget.md of the brain folder with the key set and
// knowledge/budget/agent-budget.md of the Grid home with it absent.
func TestPbRdBudgetNotePath(t *testing.T) {
	userHome := t.TempDir()
	gridHome := filepath.Join(userHome, ".grid")
	vault := filepath.Join(userHome, "Vault")
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: gridHome, Brain: vault}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, userHome), cfg); err != nil {
		t.Fatal(err)
	}
	if got, want := budgetNoteFor(gridHome, userHome, nil), filepath.Join(vault, "grid", "budget", "agent-budget.md"); got != want {
		t.Errorf("key set: budget note %q, want %q", got, want)
	}
	other := t.TempDir()
	if got, want := budgetNoteFor(gridHome, other, nil), filepath.Join(gridHome, "knowledge", "budget", "agent-budget.md"); got != want {
		t.Errorf("key absent: budget note %q, want %q", got, want)
	}
}
