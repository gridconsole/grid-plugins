package contract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// plScNewTypes are the post types the launcher lane needs (run-jobs-launcher.md
// lane 1, brain-board.md P-loop-board rows).
var plScNewTypes = strings.Fields("approval blocked cancel exit integrated landed started task-move waited waiting wake")

// plScPost is a post envelope with the given type, author role and body. The
// lease is null at fence 0 and gen 0; job and attempt name J-7 attempt 3.
func plScPost(typ, role string, body map[string]any) map[string]any {
	tier := "agent"
	switch role {
	case "owner", "member", "lead":
		tier = "person-team"
	}
	return map[string]any{
		"id":      "01K9J0000000000000000000A1",
		"task":    "01K9J000000000000000000001",
		"type":    typ,
		"type_v":  1,
		"author":  map[string]any{"id": "ana", "role": role},
		"tier":    tier,
		"worker":  "worker@01K9J000000000000000000002",
		"lease":   nil,
		"gen":     0,
		"fence":   0,
		"seq":     1,
		"at":      "2026-11-17T10:00:00.000Z",
		"re":      nil,
		"after":   []any{},
		"job":     "J-7",
		"attempt": 3,
		"wake":    nil,
		"body":    body,
	}
}

// plScValidate marshals v and validates it as typ.
func plScValidate(t *testing.T, typ string, v any) error {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return contract.Validate(typ, data)
}

// plScFixtures reads the fixture files of typ in category (valid or invalid).
func plScFixtures(t *testing.T, typ, category string) map[string][]byte {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", typ, category, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Base(f)] = data
	}
	return out
}

// TestPlScNewTypes walks every new type: it is a registered post type with a
// class and roles, its valid fixtures pass and its invalid ones fail, and an
// author role outside x-roles is refused.
func TestPlScNewTypes(t *testing.T) {
	registered := map[string]bool{}
	for _, typ := range contract.PostTypes() {
		registered[typ.Name] = true
	}
	for _, typ := range plScNewTypes {
		t.Run(typ, func(t *testing.T) {
			info, ok := contract.LookupType(typ)
			if !ok || !registered[typ] || info.Version != 1 || info.Class != "state" || len(info.Roles) == 0 {
				t.Fatalf("%s is not a registered post type: %+v (ok %v)", typ, info, ok)
			}
			for _, category := range []string{"valid", "invalid"} {
				files := plScFixtures(t, typ, category)
				if len(files) < 2 {
					t.Fatalf("%s has %d %s fixtures, want at least 2", typ, len(files), category)
				}
				for name, data := range files {
					if err := contract.Validate(typ, data); (err == nil) != (category == "valid") {
						t.Errorf("%s/%s/%s: Validate = %v", typ, category, name, err)
					}
				}
			}
			var p map[string]any
			if err := json.Unmarshal(plScFixtures(t, typ, "valid")["basic.json"], &p); err != nil {
				t.Fatal(err)
			}
			p["author"].(map[string]any)["role"] = "unregistered"
			if plScValidate(t, typ, p) == nil {
				t.Fatal("unregistered author accepted")
			}
		})
	}
}

// TestPlScTaskMoveNeedsTo is V9: a task-move without to fails validation.
func TestPlScTaskMoveNeedsTo(t *testing.T) {
	ok := plScPost("task-move", "owner", map[string]any{"from": "01K9J000000000000000000002", "to": "01K9J000000000000000000003"})
	ok["job"], ok["attempt"] = nil, nil
	if err := plScValidate(t, "task-move", ok); err != nil {
		t.Fatalf("task-move with from and to: %v", err)
	}
	migrated := plScPost("task-move", "owner", map[string]any{"from": nil, "to": "01K9J000000000000000000003"})
	migrated["job"], migrated["attempt"] = nil, nil
	if err := plScValidate(t, "task-move", migrated); err != nil {
		t.Fatalf("task-move with from null: %v", err)
	}
	noTo := plScPost("task-move", "owner", map[string]any{"from": "01K9J000000000000000000002"})
	noTo["job"], noTo["attempt"] = nil, nil
	if plScValidate(t, "task-move", noTo) == nil {
		t.Fatal("task-move without to validated")
	}
}

// plScAssign is a control post with an assign carrying the launcher fields of
// run-jobs-launcher.md Q2 and Addendum 7 (A2, B).
func plScAssign() map[string]any {
	p := plScPost("control", "lead", map[string]any{
		"assign": map[string]any{
			"job":      "J-7",
			"attempt":  3,
			"to":       "builder@01K9J000000000000000000002",
			"fence":    1,
			"until":    "2026-11-17T10:30:00.000Z",
			"on":       "vm:grid2",
			"account":  "claude-team-2",
			"model":    "opus",
			"effort":   "xhigh",
			"base":     map[string]any{"ref": "refs/grid/T-12/jobs/J-7/a2", "sha": "3f2c9e1"},
			"allow":    []any{"Bash(make deploy-prod)"},
			"grant":    "01K9J0000000000000000000B1",
			"expect":   map[string]any{"cores": 4, "mem_gb": 6},
			"test_run": false,
			"start":    map[string]any{"all": []any{map[string]any{"cores_free": 4}, map[string]any{"no_test_run": true}}, "wait_max": "30m"},
			"carry":    map[string]any{"job": "J-7", "attempt": 2},
		},
	})
	p["lease"], p["fence"] = "task", 1
	return p
}

// TestPlScAssignFields: an assign with the new fields validates as a control,
// start may be "now", and an assign with an unknown key is still refused.
func TestPlScAssignFields(t *testing.T) {
	if err := plScValidate(t, "control", plScAssign()); err != nil {
		t.Fatalf("assign with the launcher fields: %v", err)
	}
	now := plScAssign()
	now["body"].(map[string]any)["assign"].(map[string]any)["start"] = "now"
	if err := plScValidate(t, "control", now); err != nil {
		t.Fatalf("assign with start now: %v", err)
	}
	unknown := plScAssign()
	unknown["body"].(map[string]any)["assign"].(map[string]any)["colour"] = "blue"
	if plScValidate(t, "control", unknown) == nil {
		t.Fatal("assign with an unknown key validated")
	}
}

// TestPlScSubmitFields: a submit carries repo, base_sha and commits, and a
// submit body with an unknown key is still refused.
func TestPlScSubmitFields(t *testing.T) {
	p := plScPost("submit", "worker", map[string]any{"repo": "plugin", "ref": "refs/grid/T-12/jobs/J-7/a3", "sha": "9ab01cd", "base_sha": "3f2c9e1", "commits": 2})
	p["lease"], p["fence"] = "J-7", 2
	if err := plScValidate(t, "submit", p); err != nil {
		t.Fatalf("submit with repo, base_sha and commits: %v", err)
	}
	p["body"].(map[string]any)["colour"] = "blue"
	if plScValidate(t, "submit", p) == nil {
		t.Fatal("submit with an unknown body key validated")
	}
}

// TestPlScJobRecordFields: a Job record carries home, an after edge of kind
// landed, and an after edge on a Job of another task.
func TestPlScJobRecordFields(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "job-record", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data, &rec); err != nil {
		t.Fatal(err)
	}
	rec["home"] = "01K9J000000000000000000002"
	rec["after"] = []any{
		map[string]any{"job": "J-4", "kind": "landed", "why": "Start after the build Job lands"},
		map[string]any{"job": "J-4", "kind": "landed", "task": "01K9J000000000000000000009", "why": "Wait for the build task to land"},
	}
	if err := plScValidate(t, "job-record", rec); err != nil {
		t.Fatalf("job-record with home and landed edges: %v", err)
	}
	rec["home"] = ""
	if plScValidate(t, "job-record", rec) == nil {
		t.Fatal("job-record with an empty home validated")
	}
}

// TestPlScEnvelopeByLegacy: every post accepts by and legacy, and still needs
// its lease and fence.
func TestPlScEnvelopeByLegacy(t *testing.T) {
	p := plScPost("landed", "cli", map[string]any{"main_sha": "9ab01cd"})
	p["by"], p["legacy"] = "grid2:grid3", "counted"
	if err := plScValidate(t, "landed", p); err != nil {
		t.Fatalf("post with by and legacy: %v", err)
	}
	delete(p, "fence")
	if plScValidate(t, "landed", p) == nil {
		t.Fatal("post without fence validated")
	}
}
