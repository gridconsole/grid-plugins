package settings

import (
	"path/filepath"
	"testing"
)

func TestPersonDirectoryDenyRules(t *testing.T) {
	home := t.TempDir()
	rules, err := ClaudeBlock(home, filepath.Join(home, ".grid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range []string{"Edit", "Write"} {
		found := false
		for _, rule := range rules {
			if rule.Section == "permissions" && rule.Key == "deny" && rule.Value == tool+"(~/.grid-person/**)" {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s person-directory deny", tool)
		}
	}
}
