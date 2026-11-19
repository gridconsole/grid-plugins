package doctor

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

// exDenyEnv makes a user home with an existing Grid home and returns options
// whose SettingsPath is not set yet. Everything is under t.TempDir().
func exDenyEnv(t *testing.T) Options {
	t.Helper()
	home := t.TempDir()
	gridHome := filepath.Join(home, ".grid")
	if err := os.Mkdir(gridHome, 0700); err != nil {
		t.Fatal(err)
	}
	writeVersion(t, gridHome, "1")
	return Options{Home: home, GridHome: gridHome, Label: "~/.grid", GitVersion: gitOK, HooksFile: healthyHooks(t), LookPath: gridOK}
}

// exDenySettings writes a settings file with every allow and deny rule of the
// setup block except the deny rules named in drop; sandbox is the raw JSON of
// the sandbox object, or empty for none. It returns the path.
func exDenySettings(t *testing.T, o Options, sandbox string, drop ...string) string {
	t.Helper()
	rules, err := settings.ClaudeBlock(o.Home, o.GridHome)
	if err != nil {
		t.Fatal(err)
	}
	allow, deny := []string{}, []string{}
	for _, rule := range rules {
		switch {
		case rule.Section == "permissions" && rule.Key == "allow":
			allow = append(allow, rule.Value)
		case rule.Section == "permissions" && rule.Key == "deny":
			skip := false
			for _, d := range drop {
				skip = skip || d == rule.Value
			}
			if !skip {
				deny = append(deny, rule.Value)
			}
		}
	}
	doc := map[string]any{"permissions": map[string]any{"allow": allow, "deny": deny}}
	if sandbox != "" {
		doc["sandbox"] = json.RawMessage(sandbox)
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return exDenyWrite(t, data)
}

func exDenyWrite(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// exDenySnapshot is what doctor must leave alone: the bytes and the mtime.
type exDenySnapshot struct {
	data  []byte
	mtime time.Time
	err   bool
}

func exDenyTake(t *testing.T, path string) exDenySnapshot {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		return exDenySnapshot{err: true}
	}
	data, _ := os.ReadFile(path)
	return exDenySnapshot{data: data, mtime: info.ModTime()}
}

func exDenyUnchanged(t *testing.T, path string, before exDenySnapshot) {
	t.Helper()
	after := exDenyTake(t, path)
	if after.err != before.err || !bytes.Equal(after.data, before.data) || !after.mtime.Equal(before.mtime) {
		t.Fatalf("doctor changed the settings path %s", filepath.Base(path))
	}
}

// exDenyAge sets an old mtime so that any rewrite would show.
func exDenyAge(t *testing.T, path string) {
	t.Helper()
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func exDenyCheck(t *testing.T, rep Report, name string) Check {
	t.Helper()
	for _, c := range rep.Checks {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("no %q check in %+v", name, rep.Checks)
	return Check{}
}

func TestExDenyBlockHealthy(t *testing.T) {
	o := exDenyEnv(t)
	o.SettingsPath = exDenySettings(t, o, "")
	exDenyAge(t, o.SettingsPath)
	before := exDenyTake(t, o.SettingsPath)
	rep := Run(o)
	assertChecks(t, rep, map[string]string{"allow block": "", "deny block": "all setup deny rules are present", "sandbox": "off; grid setup --sandbox turns on"}, true)
	exDenyUnchanged(t, o.SettingsPath, before)
}

// Setup writes no deny rule now, so the deny block has nothing to miss: a settings
// file without any deny rule is healthy. Missing rules are reported by the allow
// block, which TestDoctorAllowBlockCLI (internal/cli/doctor_allow_test.go) covers.
func TestExDenyBlockNeedsNoDenyRules(t *testing.T) {
	o := exDenyEnv(t)
	o.SettingsPath = exDenySettings(t, o, "")
	exDenyAge(t, o.SettingsPath)
	before := exDenyTake(t, o.SettingsPath)
	rep := Run(o)
	assertChecks(t, rep, map[string]string{"allow block": "", "deny block": "all setup deny rules are present"}, true)
	deny := exDenyCheck(t, rep, "deny block")
	if !deny.OK || deny.Fix != "" {
		t.Fatalf("unexpected deny check: %+v", deny)
	}
	exDenyUnchanged(t, o.SettingsPath, before)
}

func TestExDenyBlockSameReasonsAsAllow(t *testing.T) {
	cases := []struct {
		name string
		path func(t *testing.T) string
	}{
		{"missing file", func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent.json") }},
		{"unreadable file", func(t *testing.T) string { return t.TempDir() }}, // a folder cannot be read as a file
		{"invalid JSON", func(t *testing.T) string { return exDenyWrite(t, []byte("{not json")) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := exDenyEnv(t)
			o.SettingsPath = tc.path(t)
			before := exDenyTake(t, o.SettingsPath)
			rep := Run(o)
			allow, deny := exDenyCheck(t, rep, "allow block"), exDenyCheck(t, rep, "deny block")
			// One cause, one failing check: the allow block fails with the
			// reason; the deny block carries the same reason but is not a
			// second failure.
			if allow.OK || allow.Detail == "" || allow.Fix == "" || !deny.OK || deny.Fix != "" {
				t.Fatalf("allow should fail with a reason and deny should not: allow %+v deny %+v", allow, deny)
			}
			if deny.Detail != "not checked: "+allow.Detail+"; the allow block reports it" {
				t.Fatalf("deny %+v does not carry the allow reason %+v", deny, allow)
			}
			failures := 0
			for _, c := range rep.Checks {
				if !c.OK {
					failures++
				}
			}
			if failures != 1 || rep.OK {
				t.Fatalf("one broken settings file gave %d failing checks: %+v", failures, rep.Checks)
			}
			if strings.Contains(deny.Detail, o.SettingsPath) {
				t.Fatal("settings path leaked into report")
			}
			exDenyUnchanged(t, o.SettingsPath, before)
		})
	}
}

func TestExDenyBlockSkipsUnavailableHome(t *testing.T) {
	for _, kind := range []string{"missing", "file"} {
		t.Run(kind, func(t *testing.T) {
			o := exDenyEnv(t)
			o.GridHome = filepath.Join(o.Home, ".absent")
			if kind == "file" {
				if err := os.WriteFile(o.GridHome, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			o.SettingsPath = exDenySettings(t, o, "")
			before := exDenyTake(t, o.SettingsPath)
			deny := exDenyCheck(t, Run(o), "deny block")
			if !deny.OK || deny.Detail != "skipped: Grid home is unavailable" || deny.Fix != "" {
				t.Fatalf("unexpected deny check: %+v", deny)
			}
			exDenyUnchanged(t, o.SettingsPath, before)
		})
	}
}

// A settings file written for the default projects folder is healthy for another one:
// no rule is anchored on the projects folder any more.
func TestExDenyBlockIgnoresProjects(t *testing.T) {
	o := exDenyEnv(t)
	o.SettingsPath = exDenySettings(t, o, "")
	o.Projects = filepath.Join(o.Home, "work")
	deny := exDenyCheck(t, Run(o), "deny block")
	if !deny.OK || deny.Fix != "" {
		t.Fatalf("a projects folder should need no rule: %+v", deny)
	}
}

func TestExDenySandboxLine(t *testing.T) {
	cases := []struct {
		name     string
		settings func(t *testing.T, o Options) string
		want     string
	}{
		{"on", func(t *testing.T, o Options) string { return exDenySettings(t, o, `{"enabled":true}`) }, "on (grid setup --sandbox)"},
		{"off", func(t *testing.T, o Options) string { return exDenySettings(t, o, `{"enabled":false}`) }, "off; grid setup --sandbox turns on Claude Code's sandbox"},
		{"absent", func(t *testing.T, o Options) string { return exDenySettings(t, o, "") }, "off; grid setup --sandbox turns on Claude Code's sandbox"},
		{"no file", func(t *testing.T, o Options) string { return filepath.Join(t.TempDir(), "absent.json") }, "off; grid setup --sandbox turns on Claude Code's sandbox"},
		{"no path", func(t *testing.T, o Options) string { return "" }, "unknown: cannot locate the Claude settings file"},
		{"invalid JSON", func(t *testing.T, o Options) string { return exDenyWrite(t, []byte("{not json")) }, "unknown: Claude settings file is not valid JSON or has an invalid structure"},
		{"enabled is not a boolean", func(t *testing.T, o Options) string { return exDenySettings(t, o, `{"enabled":"yes"}`) }, "unknown: "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := exDenyEnv(t)
			o.SettingsPath = tc.settings(t, o)
			before := exDenyTake(t, o.SettingsPath)
			sandbox := exDenyCheck(t, Run(o), "sandbox")
			if !sandbox.OK || sandbox.Fix != "" || !strings.HasPrefix(sandbox.Detail, tc.want) {
				t.Fatalf("sandbox check %+v, want %q", sandbox, tc.want)
			}
			if o.SettingsPath != "" {
				exDenyUnchanged(t, o.SettingsPath, before)
			}
		})
	}
}

// The sandbox line never changes whether the report is ok, whatever it says.
func TestExDenySandboxNeverFailsReport(t *testing.T) {
	o := exDenyEnv(t)
	o.SettingsPath = exDenyWrite(t, []byte("{not json"))
	rep := Run(o)
	if !exDenyCheck(t, rep, "sandbox").OK {
		t.Fatalf("sandbox check failed: %+v", rep.Checks)
	}
	o.SettingsPath = exDenySettings(t, o, `{"enabled":true}`)
	if rep := Run(o); !rep.OK {
		t.Fatalf("a healthy report with the sandbox on should be ok: %+v", rep.Checks)
	}
}

// A readable settings file with none of setup's rules is one cause: the allow
// block fails and the deny block says it was not checked.
func TestExDenyBlockNoSetupRulesIsOneFailure(t *testing.T) {
	o := exDenyEnv(t)
	o.SettingsPath = exDenyWrite(t, []byte("{}"))
	before := exDenyTake(t, o.SettingsPath)
	rep := Run(o)
	allow, deny := exDenyCheck(t, rep, "allow block"), exDenyCheck(t, rep, "deny block")
	if allow.OK || !deny.OK || !strings.HasPrefix(deny.Detail, "not checked: ") {
		t.Fatalf("allow %+v deny %+v", allow, deny)
	}
	failures := 0
	for _, c := range rep.Checks {
		if !c.OK {
			failures++
		}
	}
	if failures != 1 {
		t.Fatalf("%d failing checks: %+v", failures, rep.Checks)
	}
	exDenyUnchanged(t, o.SettingsPath, before)
}

// Setup's allow rules are in a readable file and no deny rule is: the deny block is
// still ok, and so is the allow block.
func TestExDenyBlockAllowRulesOnlyIsHealthy(t *testing.T) {
	o := exDenyEnv(t)
	o.SettingsPath = exDenySettings(t, o, "")
	rep := Run(o)
	check := exDenyCheck(t, rep, "deny block")
	if !rep.OK || !check.OK || check.Fix != "" {
		t.Fatalf("deny block %+v, report ok %v", check, rep.OK)
	}
	if allow := exDenyCheck(t, rep, "allow block"); !allow.OK {
		t.Fatalf("allow block %+v", allow)
	}
}

// The reader judges only deny rules, and setup has none: allow rules are not counted, and
// a deny rule an earlier setup wrote is neither present nor missing.
func TestExDenyReaderOnlyDenyRules(t *testing.T) {
	o := exDenyEnv(t)
	rules, err := settings.ClaudeBlock(o.Home, o.GridHome)
	if err != nil {
		t.Fatal(err)
	}
	allow := []string{}
	for _, r := range rules {
		if r.Section == "permissions" && r.Key == "deny" {
			t.Fatalf("setup writes a deny rule: %q", r.Value)
		}
		if r.Section == "permissions" && r.Key == "allow" {
			allow = append(allow, r.Value)
		}
	}
	data, _ := json.Marshal(map[string]any{"permissions": map[string]any{"allow": allow, "deny": []string{"Edit(~/.grid-person/**)"}}})
	status := settings.ReadClaudeDenyStatus(exDenyWrite(t, data), o.Home, o.GridHome, "")
	if status.Reason != "" || len(status.Present) != 0 || len(status.Missing) != 0 {
		t.Fatalf("deny status %+v", status)
	}
}
