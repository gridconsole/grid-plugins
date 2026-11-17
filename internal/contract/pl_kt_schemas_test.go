package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// plKtRecord is the valid job-record fixture with the given fields set.
func plKtRecord(t *testing.T, set map[string]any) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "job-record", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		rec[k] = v
	}
	return rec
}

func plKtValidate(t *testing.T, typ string, v any) error {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return contract.Validate(typ, data)
}

// Rows 4 and 5 (V27, V26): the Job record schema accepts a land Job whose branch
// is a full ref, a command Job with an argv and a wait Job with every and
// timeout, and refuses an argv that is not a list of words and a duration that
// is not a number and a unit.
func TestPlKtJobRecordKinds(t *testing.T) {
	sha := "0123456789abcdef0123456789abcdef01234567"
	for _, tc := range []struct {
		name string
		set  map[string]any
		ok   bool
	}{
		{"land full ref", map[string]any{"kind": "land", "role": "runner", "target": "main", "branch": "refs/grid/T-1/head", "sha": sha}, true},
		{"command", map[string]any{"kind": "command", "role": "runner", "command": []any{"land", "T-1"}}, true},
		{"runner command", map[string]any{"kind": "command", "role": "runner", "command": []any{"land", "test", "T-1", "J-3"}}, true},
		{"wait", map[string]any{"kind": "wait", "role": "runner", "every": "1m", "timeout": "30m"}, true},
		{"wait hours and seconds", map[string]any{"kind": "wait", "role": "runner", "every": "45s", "timeout": "2h"}, true},
		{"empty argv", map[string]any{"kind": "command", "command": []any{}}, false},
		{"empty word", map[string]any{"kind": "command", "command": []any{"land", ""}}, false},
		{"argv as text", map[string]any{"kind": "command", "command": "land T-1"}, false},
		{"argv of numbers", map[string]any{"kind": "command", "command": []any{"land", 1}}, false},
		{"every without unit", map[string]any{"kind": "wait", "every": "60", "timeout": "30m"}, false},
		{"zero timeout", map[string]any{"kind": "wait", "every": "1m", "timeout": "0m"}, false},
		{"timeout as number", map[string]any{"kind": "wait", "every": "1m", "timeout": 1800}, false},
		{"empty every", map[string]any{"kind": "wait", "every": "", "timeout": "30m"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := plKtValidate(t, "job-record", plKtRecord(t, tc.set))
			if (err == nil) != tc.ok {
				t.Fatalf("Validate = %v, want ok %v", err, tc.ok)
			}
		})
	}
}

// plKtTested is a tested post by the CLI on runner Job J-7 attempt 1 with the
// given body.
func plKtTested(body map[string]any) map[string]any {
	return map[string]any{
		"id":      "01K9J0000000000000000000A1",
		"task":    "01K9J000000000000000000001",
		"type":    "tested",
		"type_v":  1,
		"author":  map[string]any{"id": "cli", "role": "cli"},
		"tier":    "agent",
		"worker":  "cli@01K9J000000000000000000002",
		"lease":   nil,
		"gen":     0,
		"fence":   0,
		"seq":     1,
		"at":      "2026-11-17T10:00:00.000Z",
		"re":      nil,
		"after":   []any{},
		"job":     "J-7",
		"attempt": 1,
		"wake":    nil,
		"body":    body,
	}
}

// plKtGreen is a green tested body with one flake and one preexisting red.
func plKtGreen() map[string]any {
	return map[string]any{
		"sha":         "9ab01cd",
		"ref":         "refs/grid/T-1/jobs/J-7/a1",
		"base_sha":    "3f2c9e1",
		"head_sha":    "5d4e3f2",
		"result":      "green",
		"flakes":      []any{map[string]any{"test": "TestSlowFold", "package": "./internal/feed", "green": "2/3"}},
		"preexisting": []any{map[string]any{"test": "TestOldRed", "package": "./internal/cli"}},
	}
}

// Row 6 (V12.c): tested is a registered state post type; its fixtures validate
// as they should; its schema keeps sha, ref, base_sha, head_sha, result,
// flakes[] and preexisting[], and refuses a result other than green or nothing,
// a green result with no ref and a flake or preexisting entry without its test.
func TestPlKtTestedSchema(t *testing.T) {
	info, ok := contract.LookupType("tested")
	if !ok || info.Version != 1 || info.Class != "state" || len(info.Roles) == 0 {
		t.Fatalf("tested is not a registered post type: %+v (ok %v)", info, ok)
	}
	for _, category := range []string{"valid", "invalid"} {
		files, err := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", "tested", category, "*.json"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) < 2 {
			t.Fatalf("tested has %d %s fixtures, want at least 2", len(files), category)
		}
		for _, f := range files {
			data, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			if err := contract.Validate("tested", data); (err == nil) != (category == "valid") {
				t.Errorf("%s/%s: Validate = %v", category, filepath.Base(f), err)
			}
		}
	}
	nothing := plKtGreen()
	delete(nothing, "ref")
	nothing["result"], nothing["flakes"], nothing["preexisting"] = "nothing", []any{}, []any{}
	cases := []struct {
		name string
		body map[string]any
		ok   bool
	}{
		{"green", plKtGreen(), true},
		{"nothing without ref", nothing, true},
		{"external body", map[string]any{"ref": "bodies/01K9J0000000000000000000B1.json"}, true},
	}
	for _, key := range []string{"sha", "ref", "base_sha", "head_sha", "result", "flakes", "preexisting"} {
		b := plKtGreen()
		delete(b, key)
		cases = append(cases, struct {
			name string
			body map[string]any
			ok   bool
		}{"green without " + key, b, false})
	}
	for _, bad := range []struct {
		name  string
		key   string
		value any
	}{
		{"red result", "result", "red"},
		{"sha not hex", "sha", "main"},
		{"head_sha not hex", "head_sha", "HEAD"},
		{"flake without test", "flakes", []any{map[string]any{"package": "./internal/feed", "green": "2/3"}}},
		{"flake green not a count", "flakes", []any{map[string]any{"test": "TestSlowFold", "package": "./internal/feed", "green": "most"}}},
		{"preexisting without test", "preexisting", []any{map[string]any{"package": "./internal/cli"}}},
		{"unknown body key", "colour", "blue"},
	} {
		b := plKtGreen()
		b[bad.key] = bad.value
		cases = append(cases, struct {
			name string
			body map[string]any
			ok   bool
		}{bad.name, b, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := plKtValidate(t, "tested", plKtTested(tc.body)); (err == nil) != tc.ok {
				t.Fatalf("Validate = %v, want ok %v", err, tc.ok)
			}
		})
	}
	member := plKtTested(plKtGreen())
	member["author"] = map[string]any{"id": "ana", "role": "member"}
	member["tier"] = "person-team"
	if plKtValidate(t, "tested", member) == nil {
		t.Fatal("tested by a member validated")
	}
}
