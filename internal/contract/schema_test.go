package contract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, typ, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", typ, name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFixtures(t *testing.T) {
	for _, name := range SchemaNames() {
		t.Run(name, func(t *testing.T) {
			for _, category := range []string{"valid", "invalid"} {
				files, err := filepath.Glob(filepath.Join("..", "..", "contract", "fixtures", name, category, "*.json"))
				if err != nil || len(files) < 2 {
					t.Fatalf("need at least two %s fixtures: %v", category, err)
				}
				for _, file := range files {
					t.Run(category+"/"+filepath.Base(file), func(t *testing.T) {
						data, err := os.ReadFile(file)
						if err != nil {
							t.Fatal(err)
						}
						if err := Validate(name, data); (err == nil) != (category == "valid") {
							t.Fatalf("Validate: %v; fixture is %s", err, category)
						}
					})
				}
			}
		})
	}
	if schemas.err != nil {
		t.Fatal(schemas.err)
	}
	// Catch orphan fixture folders as well as missing fixtures for schemas.
	dirs, err := os.ReadDir(filepath.Join("..", "..", "contract", "fixtures"))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		if _, ok := schemas.byName[dir.Name()+".json"]; !ok {
			t.Errorf("fixture directory has no schema: %s", dir.Name())
		}
	}
}

func TestTypesRegistered(t *testing.T) {
	known := strings.Fields("job control ack submit result checkpoint question answer handover worker-exit loop-detected post knowledge found usage-notice head host transfer deploy-failed")
	for _, typ := range known {
		t.Run(typ, func(t *testing.T) {
			data, err := Schema(typ)
			if err != nil {
				t.Fatal(err)
			}
			var schema struct {
				Roles []string `json:"x-roles"`
				Class string   `json:"x-class"`
			}
			if err := json.Unmarshal(data, &schema); err != nil || len(schema.Roles) == 0 || schema.Class == "" {
				t.Fatalf("missing registration: %v", err)
			}
			var p map[string]any
			_ = json.Unmarshal(fixture(t, typ, "valid/basic"), &p)
			p["author"].(map[string]any)["role"] = "unregistered"
			data, _ = json.Marshal(p)
			if Validate(typ, data) == nil {
				t.Fatal("unregistered author accepted")
			}
		})
	}
}

func TestSemanticRules(t *testing.T) {
	for _, tc := range []struct {
		name, typ string
		mutate    func(map[string]any)
	}{
		{"merge-duration", "merge-lease", func(v map[string]any) { v["until"] = "2026-11-08T10:16:00.000Z" }},
		{"heartbeat-duration", "heartbeat", func(v map[string]any) { v["until"] = "2026-11-08T10:11:00.000Z" }},
		{"tenure-file-identity", "takeover", func(v map[string]any) { v["fence"] = 3 }},
		{"job-lease-identity", "result", func(v map[string]any) { v["lease"] = "J-2" }},
		{"job-body-identity", "job", func(v map[string]any) { v["job"] = "J-2" }},
		{"task-body-identity", "usage-notice", func(v map[string]any) { v["body"].(map[string]any)["task"] = "01K9J000000000000000000099" }},
		{"trust-tier-matches-role", "post", func(v map[string]any) { v["tier"] = "external" }},
		{"guest-cannot-submit", "submit", func(v map[string]any) { v["author"].(map[string]any)["role"] = "guest"; v["tier"] = "person-guest" }},
		{"guest-cannot-pickup", "post", func(v map[string]any) {
			v["author"].(map[string]any)["role"] = "guest"
			v["tier"] = "person-guest"
			v["body"].(map[string]any)["kind"] = "pickup"
		}},
		{"guest-knowledge-external", "knowledge", func(v map[string]any) { v["author"].(map[string]any)["role"] = "guest"; v["tier"] = "person-guest" }},
		{"must-fix-needs-row", "result", func(v map[string]any) {
			v["body"].(map[string]any)["must_fix"] = []any{map[string]any{"at": "main.go:1"}}
		}},
		{"l1-needs-validator-job", "result", func(v map[string]any) {
			v["body"].(map[string]any)["verified"] = map[string]any{"level": "L1", "pass": true, "by": "sonnet", "evidence": []string{}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var v map[string]any
			_ = json.Unmarshal(fixture(t, tc.typ, "valid/basic"), &v)
			tc.mutate(v)
			data, _ := json.Marshal(v)
			if Validate(tc.typ, data) == nil {
				t.Fatal("invalid semantic rule accepted")
			}
		})
	}
}

func TestMergeLeaseExpiry(t *testing.T) {
	var lease MergeLease
	if err := json.Unmarshal(fixture(t, "merge-lease", "valid/basic"), &lease); err != nil {
		t.Fatal(err)
	}
	end, _ := time.Parse(time.RFC3339Nano, lease.Until)
	for _, tc := range []struct {
		name string
		at   time.Time
		want bool
	}{{"live", end.Add(-time.Nanosecond), false}, {"expiry", end, true}, {"stale", end.Add(time.Hour), true}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := lease.Breakable(tc.at); got != tc.want {
				t.Fatalf("Breakable = %v, want %v", got, tc.want)
			}
		})
	}
	lease.Holder = nil
	if !lease.Breakable(end.Add(-time.Minute)) {
		t.Fatal("released lease should be available")
	}
	lease.Until = "invalid"
	if lease.Breakable(end) {
		t.Fatal("malformed lease should not authorize a break")
	}
	if err := json.Unmarshal(fixture(t, "merge-lease", "valid/renewed"), &lease); err != nil {
		t.Fatal(err)
	}
	if lease.Breakable(end) || !lease.Breakable(end.Add(5*time.Minute)) {
		t.Fatal("a renewal must move the fifteen-minute expiry window")
	}
}

func TestValidatorBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, schema, value string }{
		{"unknown-schema", "missing", `{}`},
		{"multiple-values", "trust-tier", `"agent" "external"`},
		{"malformed-json", "trust-tier", `{`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if Validate(tc.schema, []byte(tc.value)) == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
	if checkSubset(map[string]any{"unsupported": true}) == nil {
		t.Fatal("unsupported keyword accepted")
	}
	for _, tc := range []struct {
		name, schema, value string
		valid               bool
	}{
		{"integer-exponent", `{"type":"integer"}`, `1e2`, true},
		{"integer-fraction", `{"type":"integer"}`, `1.5`, false},
		{"large-number", `{"type":"integer","maximum":9007199254740992}`, `9007199254740993`, false},
		{"unicode-length", `{"type":"string","maxLength":1}`, `"é"`, true},
		{"invalid-calendar", `{"format":"date-time"}`, `"2026-02-30T10:00:00.000Z"`, false},
		{"oneof-overlap", `{"oneOf":[{"type":"string"},{"minLength":1}]}`, `"x"`, false},
		{"numeric-equality", `{"enum":[1]}`, `1.0`, true},
		{"unique", `{"uniqueItems":true}`, `[1,1.0]`, false},
		{"unknown-ref", `{"$ref":"missing.json"}`, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var s map[string]any
			var v any
			_ = decode([]byte(tc.schema), &s)
			_ = decode([]byte(tc.value), &v)
			if err := validate(s, v, "$", 0); (err == nil) != tc.valid {
				t.Fatalf("validate = %v, want valid %v", err, tc.valid)
			}
		})
	}
}
