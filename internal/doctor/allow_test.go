package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

func healthyAllowSettings(t *testing.T, home, gridHome string) string {
	t.Helper()
	rules, err := settings.ClaudeBlock(home, gridHome)
	if err != nil {
		t.Fatal(err)
	}
	allow := []string{}
	for _, rule := range rules {
		if rule.Section == "permissions" && rule.Key == "allow" {
			allow = append(allow, rule.Value)
		}
	}
	data, err := json.Marshal(map[string]any{"permissions": map[string]any{"allow": allow}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAllowBlockWithInjectedSettingsPath(t *testing.T) {
	home := t.TempDir()
	gridHome := filepath.Join(home, ".grid")
	if err := os.Mkdir(gridHome, 0700); err != nil {
		t.Fatal(err)
	}
	writeVersion(t, gridHome, "1")
	o := Options{Home: home, GridHome: gridHome, Label: "~/.grid", GitVersion: gitOK, HooksFile: healthyHooks(t), LookPath: gridOK,
		SettingsPath: healthyAllowSettings(t, home, gridHome)}
	assertChecks(t, Run(o), map[string]string{"allow block": ""}, true)
	if err := os.WriteFile(o.SettingsPath, []byte(`{"permissions":{"allow":["Bash(grid *)","Read(~/.grid/**)"]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	report := Run(o)
	assertChecks(t, report, map[string]string{"allow block": "!Edit(~/.grid/**)"}, false)
	for _, check := range report.Checks {
		if strings.Contains(check.Detail, o.SettingsPath) {
			t.Fatal("settings path leaked into report")
		}
		if check.Name == "allow block" && check.Fix != "Run grid setup to add the allow block; grid setup --remove takes it out." {
			t.Fatalf("unexpected hint: %+v", check)
		}
	}
}
