package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// retireHandleHome leaves what an earlier setup of the Grid home ~/.grid/abdullahi (a
// handle's folder directly below the session's) wrote: its own record, and a settings
// file holding its retired deny rules, a rule only the person has and a rule the
// record owns that Grid never wrote.
func retireHandleHome(t *testing.T, h *harness) (settingsFile, record string, retired []string) {
	t.Helper()
	retired = []string{
		"Edit(~/.grid/abdullahi/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**)",
		"Edit(~/.grid/abdullahi/setup/**)",
		"Write(~/.grid/abdullahi/setup/**)",
	}
	settingsFile = filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json")
	record = filepath.Join(h.gridHome, "abdullahi", "setup", "claude-block.json")
	var rules []map[string]string
	for _, value := range append(slices.Clone(retired), "Edit(~/notes-i-keep/**)") {
		rules = append(rules, map[string]string{"section": "permissions", "key": "deny", "value": value})
	}
	data, err := json.Marshal(map[string]any{"path": settingsFile, "rules": rules})
	if err != nil {
		t.Fatal(err)
	}
	h.write(record, string(data))
	// The settings file may already hold the session home's own rules.
	doc := map[string]any{}
	if old, err := os.ReadFile(settingsFile); err == nil {
		if err := json.Unmarshal(old, &doc); err != nil {
			t.Fatal(err)
		}
	}
	permissions, _ := doc["permissions"].(map[string]any)
	if permissions == nil {
		permissions = map[string]any{}
	}
	deny, _ := permissions["deny"].([]any)
	for _, value := range append(slices.Clone(retired), "Read(~/.ssh/**)", "Edit(~/notes-i-keep/**)") {
		deny = append(deny, value)
	}
	permissions["deny"] = deny
	doc["permissions"] = permissions
	data, err = json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	h.write(settingsFile, string(data))
	return settingsFile, record, retired
}

// A setup from one Grid home retires the rules another home's setup wrote, as the
// session start does: a person's one settings file holds every home's forms, and a
// retired deny rule left there still blocks the agents of the other home.
func TestSetupRetiresTheOtherHomesRules(t *testing.T) {
	alreadySetUp := func(t *testing.T, h *harness) {
		// The home's own rules are in place; the other home's forms are put back.
		if code := h.run(h.home, "setup", "--yes"); code != 0 {
			t.Fatalf("code=%d: %s", code, h.errb)
		}
	}
	for _, tt := range []struct {
		name   string
		before func(*testing.T, *harness) // runs before the setup under test
		args   []string
		retire bool
	}{
		{"fresh setup", func(*testing.T, *harness) {}, []string{"setup", "--yes"}, true},
		{"setup already done", alreadySetUp, []string{"setup", "--yes"}, true},
		{"declined", func(*testing.T, *harness) {}, []string{"setup"}, false},
		// Nothing to write, so no question is asked and nothing is confirmed.
		{"setup already done, no --yes and no terminal", alreadySetUp, []string{"setup"}, false},
		{"setup already done, a preview", alreadySetUp, []string{"setup", "--json"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := claudeHarness(t)
			tt.before(t, h)
			file, record, retired := retireHandleHome(t, h)
			before, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			if code := h.run(h.home, tt.args...); code != 0 {
				t.Fatalf("code=%d: %s", code, h.errb)
			}
			after, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct{ Permissions struct{ Deny []string } }
			if err := json.Unmarshal(after, &doc); err != nil {
				t.Fatal(err)
			}
			if !tt.retire {
				if string(after) != string(before) {
					t.Fatalf("a setup nobody confirmed changed the settings: %s", after)
				}
				return
			}
			for _, rule := range retired {
				if slices.Contains(doc.Permissions.Deny, rule) {
					t.Errorf("permissions.deny still holds %s", rule)
				}
			}
			for _, want := range []string{"Read(~/.ssh/**)", "Edit(~/notes-i-keep/**)"} {
				if !slices.Contains(doc.Permissions.Deny, want) {
					t.Errorf("permissions.deny lost %s: %q", want, doc.Permissions.Deny)
				}
			}
			data, err := os.ReadFile(record)
			if err != nil {
				t.Fatal(err)
			}
			var owned struct{ Rules []struct{ Value string } }
			if err := json.Unmarshal(data, &owned); err != nil {
				t.Fatal(err)
			}
			var values []string
			for _, rule := range owned.Rules {
				values = append(values, rule.Value)
			}
			if !slices.Equal(values, []string{"Edit(~/notes-i-keep/**)"}) {
				t.Errorf("the other home's record owns %q, want only the rule that was never retired", values)
			}
		})
	}
}
