package contract

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCheckSubsetRefusals(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   string // "" means checkSubset returns nil; "any" means any error
	}{
		{"format other than date-time", `{"format":"email"}`, "unsupported format email"},
		{"format date-time is listed", `{"format":"date-time"}`, ""},
		{"pattern that does not compile", `{"pattern":"("}`, "any"},
		{"properties child with unsupported keyword", `{"properties":{"a":{"bogus":1}}}`, `unsupported keyword "bogus"`},
		{"tuple-form items", `{"items":[{}]}`, "unsupported tuple-form items"},
		{"not child that fails", `{"not":{"bogus":1}}`, `unsupported keyword "bogus"`},
		{"anyOf child that fails", `{"anyOf":[{"bogus":1}]}`, `unsupported keyword "bogus"`},
		{"unsupported keyword at top", `{"bogus":1}`, `unsupported keyword "bogus"`},
		{"only listed keywords", `{"type":"object","required":["a"],"properties":{"a":{"type":"string","pattern":"^a$","format":"date-time"}},"items":{"type":"string"},"allOf":[{"minimum":1}]}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s map[string]any
			if err := json.Unmarshal([]byte(tt.schema), &s); err != nil {
				t.Fatal(err)
			}
			err := checkSubset(s)
			switch tt.want {
			case "":
				if err != nil {
					t.Fatalf("checkSubset(%s) = %v, want nil", tt.schema, err)
				}
			case "any":
				if err == nil {
					t.Fatalf("checkSubset(%s) = nil, want an error", tt.schema)
				}
			default:
				if err == nil || err.Error() != tt.want {
					t.Fatalf("checkSubset(%s) = %v, want %q", tt.schema, err, tt.want)
				}
			}
		})
	}
}

func TestValidateNestingLimit(t *testing.T) {
	err := validate(map[string]any{}, nil, "$", 101)
	if err == nil || err.Error() != "$: schema nesting limit" {
		t.Fatalf("validate at depth 101 = %v, want %q", err, "$: schema nesting limit")
	}
	if err := validate(map[string]any{}, nil, "$", 100); err != nil {
		t.Fatalf("validate at depth 100 = %v, want nil", err)
	}
}

func TestMatchesTypeUnknownAndRationalUnreadable(t *testing.T) {
	if matchesType("date", "x") {
		t.Fatal(`matchesType("date", "x") = true, want false`)
	}
	if got := rational(json.Number("x")); got.Sign() != 0 {
		t.Fatalf(`rational("x") = %v, want 0`, got)
	}
}

func TestWorkerMemberSplit(t *testing.T) {
	const machine = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	tests := []struct{ id, want string }{
		{"plain", ""},
		{"@m", ""},
		{"a@b@" + machine, "a@b"},
		{"m@" + machine, "m"},
	}
	for _, tt := range tests {
		if got := WorkerMember(tt.id); got != tt.want {
			t.Errorf("WorkerMember(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestStateOfIntegrated(t *testing.T) {
	state, reason := StateOf("integrated")
	if state != StateNeedsReview || reason != "" {
		t.Fatalf(`StateOf("integrated") = %q, %q, want %q, ""`, state, reason, StateNeedsReview)
	}
}

func TestFormatTime(t *testing.T) {
	zone := time.FixedZone("x", 3600)
	if got, want := FormatTime(time.Date(2026, 11, 11, 13, 0, 5, 0, zone)), "2026-11-11T12:00:05.000Z"; got != want {
		t.Errorf("FormatTime(non-UTC zone) = %q, want %q", got, want)
	}
	got := FormatTime(time.Date(2026, 11, 11, 12, 0, 5, 123456789, time.UTC))
	if !strings.HasSuffix(got, ".123Z") {
		t.Errorf("FormatTime with 123456789 ns = %q, want it to end .123Z", got)
	}
	if want := "2026-11-11T12:00:05.123Z"; got != want {
		t.Errorf("FormatTime with 123456789 ns = %q, want %q", got, want)
	}
}

func TestExitCodeDropsWhatSchemasRefuse(t *testing.T) {
	for _, raw := range []string{`""`, `2.5`, `1e300`} {
		if got := exitCode(json.RawMessage(raw)); got != nil {
			t.Errorf("exitCode(%s) = %s, want nil", raw, string(got))
		}
	}
	// Contrast: a whole number written 2.0 reads as 2.
	if got := exitCode(json.RawMessage(`2.0`)); string(got) != `2` {
		t.Errorf("exitCode(2.0) = %s, want 2", string(got))
	}
}

func TestContainsNoMatch(t *testing.T) {
	if contains([]string{"a"}, "b") {
		t.Fatal(`contains([a], "b") = true, want false`)
	}
	if !contains([]string{"a"}, "a") {
		t.Fatal(`contains([a], "a") = false, want true`)
	}
}
