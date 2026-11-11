package contract

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	if checkSubset(map[string]any{"items": []any{map[string]any{"type": "string"}}}) == nil {
		t.Fatal("tuple-form items accepted")
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

func TestTierForMatchesEnvelope(t *testing.T) {
	var schema struct {
		Properties struct {
			Author struct {
				Properties struct {
					Role struct {
						Enum []string `json:"enum"`
					} `json:"role"`
				} `json:"properties"`
			} `json:"author"`
		} `json:"properties"`
	}
	data, _ := Schema("envelope")
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	roles := schema.Properties.Author.Properties.Role.Enum
	if len(roles) == 0 {
		t.Fatal("envelope lists no roles")
	}
	for _, role := range roles {
		for _, tier := range []TrustTier{PersonTeam, PersonGuest, Agent, External} {
			var p map[string]any
			_ = json.Unmarshal(fixture(t, "post", "valid/basic"), &p)
			p["author"] = map[string]any{"id": "someone", "role": role}
			p["tier"] = tier
			data, _ := json.Marshal(p)
			if err := Validate("envelope", data); (err == nil) != (tier == TierFor(role)) {
				t.Errorf("role %s tier %s: %v", role, tier, err)
			}
		}
	}
}

func TestWorkerPatternIsTheContracts(t *testing.T) {
	var envelope struct {
		Properties struct {
			Worker struct {
				Pattern string `json:"pattern"`
			} `json:"worker"`
		} `json:"properties"`
	}
	data, _ := Schema("envelope")
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Properties.Worker.Pattern != workerPattern.String() {
		t.Fatalf("envelope worker pattern %q, shared %s: %v", envelope.Properties.Worker.Pattern, workerPattern, err)
	}
	quoted, _ := json.Marshal(workerPattern.String())
	for _, schema := range []string{"control", "handover"} {
		data, _ := Schema(schema)
		if !strings.Contains(string(data), string(quoted)) {
			t.Errorf("%s writer pattern differs from the shared one", schema)
		}
	}
	for id, want := range map[string]bool{
		"jürgen@01K9J000000000000000000002":    true,
		"a@b@01K9J000000000000000000002":       true,
		"worker@01K9J000000000000000000002":    true,
		"two words@01K9J000000000000000000002": false,
		"a/b@01K9J000000000000000000002":       false,
		"@01K9J000000000000000000002":          false,
		"worker@81K9J000000000000000000002":    false,
		"worker@01k9j000000000000000000002":    false,
	} {
		if ValidWorker(id) != want {
			t.Errorf("ValidWorker(%q) = %v", id, !want)
		}
		var p map[string]any
		_ = json.Unmarshal(fixture(t, "post", "valid/basic"), &p)
		p["worker"] = id
		data, _ := json.Marshal(p)
		if err := Validate("envelope", data); (err == nil) != want {
			t.Errorf("envelope worker %q: %v", id, err)
		}
	}
	if got := WorkerMember("a@b@01K9J000000000000000000002"); got != "a@b" {
		t.Fatalf("WorkerMember = %q", got)
	}
}

func TestPostTypes(t *testing.T) {
	types := PostTypes()
	if len(types) != 19 {
		t.Fatalf("post types = %d", len(types))
	}
	for _, typ := range types {
		if typ.Version < 1 || len(typ.Roles) == 0 || typ.Class != "state" && typ.Class != "chatter" {
			t.Errorf("bad registration %+v", typ)
		}
	}
	if r, ok := LookupType("result"); !ok || r.Version != 2 || r.Class != "state" {
		t.Fatalf("result = %+v", r)
	}
	for _, name := range []string{"envelope", "heartbeat", "claim", "../schemas/job"} {
		if _, ok := LookupType(name); ok {
			t.Errorf("%s registered as a post type", name)
		}
	}
}

func TestPersonWithdrawControlSchema(t *testing.T) {
	for _, tc := range []struct {
		role, event  string
		leased, want bool
	}{
		{"owner", "withdrawn", false, true}, {"member", "withdrawn", false, true},
		{"guest", "withdrawn", false, true}, {"lead", "withdrawn", false, true},
		{"agent", "withdrawn", false, false}, {"cli", "withdrawn", false, false},
		{"owner", "aged", false, false}, {"member", "defaulted", false, false},
		{"guest", "withdrawn", true, false}, {"cli", "withdrawn", true, true},
	} {
		t.Run(tc.role+"/"+tc.event+"/"+fmt.Sprint(tc.leased), func(t *testing.T) {
			var p map[string]any
			if err := json.Unmarshal(fixture(t, "control", "invalid/question-unleased"), &p); err != nil {
				t.Fatal(err)
			}
			p["author"] = Author{ID: "person", Role: tc.role}
			p["tier"] = TierFor(tc.role)
			p["body"] = map[string]any{"target": p["re"], "question_event": tc.event, "text": "No longer needed"}
			if tc.leased {
				p["lease"], p["fence"] = "task", 1
			}
			raw, _ := json.Marshal(p)
			if err := Validate("control", raw); (err == nil) != tc.want {
				t.Fatalf("control admitted=%v want=%v: %v", err == nil, tc.want, err)
			}
		})
	}
}
