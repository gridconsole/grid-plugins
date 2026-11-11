package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ezKeysAppKeys are the seven keys the app's Settings page saves into
// <home>/settings.json, and the account records the plugin already carries.
var ezKeysAppKeys = []string{"notifications", "models", "usageRules", "windowRules", "workspace", "accountSelection", "hiddenProjects", "accounts"}

// ezKeysAppFile is the app's own output: two-space indent, one key per save, and
// accountSelection.fullAt as a plain JSON number.
const ezKeysAppFile = `{
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
  "accounts": [
    {
      "id": "claude-ann",
      "kind": "claude",
      "name": "ann@example.com",
      "email": "ann@example.com",
      "dir": "~/.claude-ann",
      "added": "auto"
    },
    {
      "id": "codex-lab",
      "kind": "codex",
      "name": "Lab",
      "email": "",
      "dir": "~/.codex-lab",
      "added": "person"
    }
  ]
}
`

// ezKeysRead reads settings.json the way the app does: one object, numbers as written.
func ezKeysRead(t *testing.T, data string) map[string]any {
	t.Helper()
	var m map[string]any
	d := json.NewDecoder(bytes.NewReader([]byte(data)))
	d.UseNumber()
	if err := d.Decode(&m); err != nil || m == nil {
		t.Fatalf("not one JSON object: %v\n%s", err, data)
	}
	if err := d.Decode(new(any)); err != io.EOF {
		t.Fatalf("more than one JSON value in the file:\n%s", data)
	}
	return m
}

// ezKeysKept fails unless the file still holds the eight keys as the app wrote them.
func ezKeysKept(t *testing.T, h *harness, step string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.gridHome, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := ezKeysRead(t, string(data))
	want := ezKeysRead(t, ezKeysAppFile)
	for _, k := range ezKeysAppKeys {
		if !reflect.DeepEqual(got[k], want[k]) {
			t.Errorf("after %s, %s changed:\n got  %#v\n want %#v", step, k, got[k], want[k])
		}
	}
	return got
}

func TestEzAppKeysSettingsVerbsOnTheAppFile(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "settings.json"), ezKeysAppFile)

	if code := h.run(h.home, "hook", "levels", "--json"); code != 0 {
		t.Fatalf("hook levels --json on the app's settings.json: exit %d: %s %s", code, h.out, h.errb)
	}
	var levels map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &levels); err != nil || levels["home-allow"] == "" {
		t.Fatalf("hook levels --json is not the levels: %v: %s", err, h.out)
	}
	for _, args := range [][]string{{"hook", "level", "home-allow", "off"}, {"agent", "set", "grid-builder", "--model", "opus"}} {
		if code := h.run(h.home, args...); code != 0 || !strings.Contains(h.out.String(), "updated in personal") {
			t.Fatalf("%v on the app's settings.json: exit %d: %s %s", args, code, h.out, h.errb)
		}
		got := ezKeysKept(t, h, strings.Join(args, " "))
		for _, k := range []string{"agents", "skills", "hooks"} {
			if _, ok := got[k]; !ok {
				t.Errorf("after %v the plugin's %q is missing beside the app keys", args, k)
			}
		}
	}
	// The plugin's own changes are in the file, and read back.
	if code := h.run(h.home, "hook", "levels"); code != 0 || !strings.Contains(h.out.String(), "home-allow off") {
		t.Fatalf("hook levels after the write: exit %d: %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.home, "agent", "list", "--json"); code != 0 || !strings.Contains(h.out.String(), "opus") {
		t.Fatalf("agent list after the write: exit %d: %s %s", code, h.out, h.errb)
	}
}

// The app's keys are the app's: no list shows them.
func TestEzAppKeysListsDoNotShowThem(t *testing.T) {
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.write(filepath.Join(h.gridHome, "settings.json"), ezKeysAppFile)
	for _, args := range [][]string{{"agent", "list"}, {"agent", "list", "--json"}, {"skill", "list"}, {"skill", "list", "--json"}, {"hook", "levels"}, {"hook", "levels", "--json"}} {
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("%v on the app's settings.json: exit %d: %s %s", args, code, h.out, h.errb)
		}
		for _, leak := range append([]string{"claude-ann", "codex-lab", "9007199254740993", "~/tasks"}, ezKeysAppKeys...) {
			if strings.Contains(h.out.String(), leak) {
				t.Errorf("%v shows %q:\n%s", args, leak, h.out)
			}
		}
	}
}
