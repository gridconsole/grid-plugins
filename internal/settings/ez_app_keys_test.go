package settings

import (
	"bytes"
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// ezKeysAppKeys are the seven top-level keys the app's Settings page saves into
// <home>/settings.json (app internal/slices/settings/check.go:14), plus the account
// records the plugin already carries.
var ezKeysAppKeys = []string{"notifications", "models", "usageRules", "windowRules", "workspace", "accountSelection", "hiddenProjects", "accounts"}

// ezKeysAppFile is what the app writes: two-space indent, one key per save in the
// order the saves add them, numbers as plain JSON numbers (fullAt is 90, and one
// number is past 2^53 to show that nothing reads a value through a float).
func ezKeysAppFile() string {
	accounts := strings.TrimSuffix(strings.TrimPrefix(appAccountsFile, `{"accounts":`), `}`)
	return `{
  "notifications": {
    "reach": {
      "phone": true
    },
    "digestAt": "17:30",
    "quiet": true,
    "quietFrom": "22:00",
    "quietTo": "07:00",
    "stuck": "30"
  },
  "models": {
    "set": "balanced",
    "changed": {
      "grid-builder": [
        "opus",
        "high"
      ]
    }
  },
  "usageRules": {
    "rules": [
      {
        "when": "week",
        "over": 80,
        "do": "warn"
      }
    ],
    "default": {
      "limit": 9007199254740993
    }
  },
  "windowRules": {
    "swat": 90,
    "hold": false,
    "go": true
  },
  "workspace": {
    "warn": 80,
    "hold": false,
    "tasksFolder": "~/tasks",
    "transcriptsKeep": {
      "days": 30,
      "gb": 5
    }
  },
  "accountSelection": {
    "mode": "smart",
    "acct": "claude-ann",
    "fullAt": 90
  },
  "hiddenProjects": [
    "old"
  ],
  "accounts": ` + accounts + `
}
`
}

// ezKeysHome is a Grid home whose settings.json is the app's own file.
func ezKeysHome(t *testing.T) (Store, string, map[string]any) {
	t.Helper()
	s, _ := fixture(t)
	path := filepath.Join(s.Home, "settings.json")
	put(t, path, []byte(ezKeysAppFile()), 0o644)
	return s, path, ezKeysRead(t, []byte(ezKeysAppFile()))
}

// ezKeysRead reads a file the way the app does (app store.go:79-87): one object,
// numbers kept as written.
func ezKeysRead(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&m); err != nil || m == nil {
		t.Fatalf("not one JSON object: %v\n%s", err, data)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatalf("more than one JSON value in the file:\n%s", data)
	}
	return m
}

// ezKeysKept fails unless every app key reads back as it was written.
func ezKeysKept(t *testing.T, path string, want map[string]any) map[string]any {
	t.Helper()
	got := ezKeysRead(t, contents(t, path))
	for _, k := range ezKeysAppKeys {
		if !reflect.DeepEqual(got[k], want[k]) {
			t.Errorf("%s changed:\n got  %#v\n want %#v", k, got[k], want[k])
		}
	}
	var raw map[string]json.RawMessage
	must(t, json.Unmarshal(contents(t, path), &raw))
	wantAccounts, _ := json.Marshal(want["accounts"])
	if !sameJSON(t, raw["accounts"], wantAccounts) {
		t.Errorf("accounts changed: %s", raw["accounts"])
	}
	return got
}

func TestEzAppKeysEffectiveReadsTheAppFile(t *testing.T) {
	s, _, _ := ezKeysHome(t)
	got, err := s.Effective()
	if err != nil {
		t.Fatalf("a settings.json the app saved must read: %v", err)
	}
	blank, _ := fixture(t)
	want, err := blank.Effective()
	must(t, err)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("the app's keys must not change the defaults:\n got  %+v\n want %+v", got, want)
	}
	if out, _ := json.Marshal(got); strings.Contains(string(out), "notifications") || strings.Contains(string(out), "claude-ann") {
		t.Fatalf("the app's keys are not part of the effective values: %s", out)
	}
	if levels, err := ReadHookLevels(s.Home, ""); err != nil || levels["home-allow"] != defaultLevel("home-allow") {
		t.Fatalf("hook levels on the app file: %v, %v", levels, err)
	}
}

func TestEzAppKeysPluginWritesKeepThem(t *testing.T) {
	s, path, want := ezKeysHome(t)
	for _, step := range []struct {
		name  string
		write func() error
		check func(Document) bool
	}{
		{"SetAgent", func() error { return s.SetAgent("grid-builder", Agent{Model: "opus"}) }, func(d Document) bool { return d.Agents["grid-builder"].Model == "opus" }},
		{"SetSkill", func() error { return s.SetSkill("browser", false) }, func(d Document) bool { return !d.Skills["browser"] }},
		{"SetHook", func() error { return s.SetHook("home-allow", "off") }, func(d Document) bool { return d.Hooks["home-allow"] == "off" }},
		{"SetAgents", func() error {
			return s.SetAgents(map[string]Agent{"grid-planner": {Model: "haiku", Effort: "low"}}, "")
		}, func(d Document) bool { return d.Agents["grid-planner"] == Agent{Model: "haiku", Effort: "low"} }},
	} {
		if err := step.write(); err != nil {
			t.Fatalf("%s on the app file: %v", step.name, err)
		}
		got := ezKeysKept(t, path, want)
		for _, k := range []string{"agents", "skills", "hooks"} {
			if _, ok := got[k]; !ok {
				t.Errorf("%s: the plugin's %q sits beside the app keys, and is missing:\n%s", step.name, k, contents(t, path))
			}
		}
		eff, err := s.Effective()
		if err != nil || !step.check(eff) {
			t.Errorf("%s: the plugin's own change is not there: %v, %+v", step.name, err, eff)
		}
	}
	if keys := len(ezKeysRead(t, contents(t, path))); keys != len(ezKeysAppKeys)+3 {
		t.Errorf("the file holds %d keys, want the %d app keys and the plugin's agents, skills and hooks", keys, len(ezKeysAppKeys))
		t.Logf("%s", contents(t, path))
	}
}

func TestEzAppKeysBudgetApplyAndUndoKeepThem(t *testing.T) {
	o, _ := budgetFixture(t)
	path := filepath.Join(o.GridHome, "settings.json")
	put(t, path, []byte(ezKeysAppFile()), 0o644)
	want := ezKeysRead(t, []byte(ezKeysAppFile()))

	o.Preset = "high"
	if _, err := BudgetApply(o); err != nil {
		t.Fatalf("apply on the app file: %v", err)
	}
	if got := ezKeysKept(t, path, want); got["budget"] != "high" {
		t.Fatalf("apply did not record its preset:\n%s", contents(t, path))
	}
	must(t, (Store{Home: o.GridHome}).SetSkill("browser", false))
	if _, err := BudgetUndo(o); err != nil {
		t.Fatalf("undo on the app file: %v", err)
	}
	ezKeysKept(t, path, want)
}

func TestEzAppKeysResolveHookLevelAnswersTheSetLevel(t *testing.T) {
	s, path, _ := ezKeysHome(t)
	body := strings.Replace(ezKeysAppFile(), "{\n", "{\n  \"hooks\": {\n    \"home-allow\": \"off\",\n    \"plan-guard\": \"warn\"\n  },\n", 1)
	put(t, path, []byte(body), 0o644)
	resolver, query := where.Resolver{GridHome: s.Home}, where.Query{Cwd: t.TempDir()}
	for name, level := range map[string]string{"home-allow": "off", "plan-guard": "warn"} {
		if got, err := ResolveHookLevel(name, resolver, query); err != nil || got != level {
			t.Errorf("%s set to %s resolves to %q, %v", name, level, got, err)
		}
	}
}

func TestEzAppKeysOtherKeysAreStillRefused(t *testing.T) {
	for _, body := range []string{
		`{"unknown":1}`,
		`{"notifications":1,"unknown":2}`,
		`{"unknown":2,"notifications":{}}`,
		`{"notifications":{},"notifications":{}}`,
		`{"hiddenProjects":["old"],"hiddenproject":[]}`,
		`{"accountSelection":{"fullAt":90}}{"models":{}}`,
	} {
		if _, err := decode([]byte(body)); err == nil {
			t.Errorf("%s must still be refused", body)
		}
	}
	if _, err := decode([]byte(`{"notifications":1,"unknown":2}`)); err == nil || !strings.Contains(err.Error(), `unknown key "unknown"`) {
		t.Errorf("the refusal names the key that is not known: %v", err)
	}
	s, _ := fixture(t)
	put(t, filepath.Join(s.Home, "settings.json"), []byte(`{"notifications":{},"surprise":1}`), 0o644)
	if _, err := s.Effective(); err == nil || !strings.Contains(err.Error(), `unknown key "surprise"`) {
		t.Errorf("Effective on an unknown key beside an app key: %v", err)
	}
	if err := s.SetSkill("browser", false); err == nil {
		t.Errorf("a write must not go through a file with an unknown key")
	}
	if got := string(contents(t, filepath.Join(s.Home, "settings.json"))); got != `{"notifications":{},"surprise":1}` {
		t.Errorf("the refused file was changed: %s", got)
	}
}
