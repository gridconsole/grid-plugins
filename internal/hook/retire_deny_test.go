package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

// oldSetup leaves what an earlier setup wrote under the fixture's home: three deny
// rules it no longer writes (one of them the person's approval identity), and the
// person's own, with a record that owns the Grid ones.
func oldSetup(t *testing.T, root string) (settingsFile, record string) {
	t.Helper()
	settingsFile = filepath.Join(root, ".claude", "settings.json")
	record = filepath.Join(root, "data", "setup", "claude-block.json")
	old := []string{"Edit(~/data/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**)", "Edit(~/data/setup/**)", "Edit(~/.grid-person/**)"}
	kept := "Edit(~/notes-i-keep/**)"
	rules := []settings.ClaudeRule{{Section: "permissions", Key: "deny", Value: old[0]}, {Section: "permissions", Key: "deny", Value: old[1]}, {Section: "permissions", Key: "deny", Value: old[2]}, {Section: "permissions", Key: "deny", Value: kept}}
	data, _ := json.Marshal(map[string]any{"path": settingsFile, "rules": rules})
	write := func(path, text string) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(record, string(data))
	write(settingsFile, `{"permissions":{"deny":["Read(~/.ssh/**)","`+old[0]+`","`+old[1]+`","`+old[2]+`","`+kept+`"]}}`)
	return settingsFile, record
}

func denyList(t *testing.T, file string) []string {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct{ Permissions struct{ Deny []string } }
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc.Permissions.Deny
}

func TestSessionStartRetiresOldDenyRules(t *testing.T) {
	o, out, root := fixture(t)
	file, record := oldSetup(t, root)
	run := func() {
		out.Reset()
		o.Stdin = strings.NewReader(`{"source":"startup"}`)
		if code := Run(o); code != 0 {
			t.Fatalf("exit %d", code)
		}
	}
	run()
	if got, want := denyList(t, file), []string{"Read(~/.ssh/**)", "Edit(~/notes-i-keep/**)"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("denies %v, want %v", got, want)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "knowledge") || strings.Contains(string(data), "setup/**") || strings.Contains(string(data), ".grid-person") || !strings.Contains(string(data), "notes-i-keep") {
		t.Fatalf("record: %s", data)
	}
	before, _ := os.Stat(file)
	past := before.ModTime().Add(-time.Hour)
	if err := os.Chtimes(file, past, past); err != nil {
		t.Fatal(err)
	}
	run()
	after, _ := os.Stat(file)
	if !after.ModTime().Equal(past) {
		t.Fatal("a session with nothing to retire rewrote the settings")
	}
}

func TestSessionStartRetireFollowsTheHookLevel(t *testing.T) {
	o, _, root := fixture(t)
	file, _ := oldSetup(t, root)
	if err := (settings.Store{Home: o.Resolver.GridHome}).SetHook("session-start", "off"); err != nil {
		t.Fatal(err)
	}
	o.Stdin = strings.NewReader(`{}`)
	Run(o)
	if got := denyList(t, file); len(got) != 5 {
		t.Fatalf("an off session start changed the settings: %v", got)
	}
}
