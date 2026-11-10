package contract

import (
	"encoding/json"
	"testing"
)

// State written before Jobs carried a kind still loads, with an empty kind;
// state with a kind keeps it.
func TestStateJobKindLoads(t *testing.T) {
	var old State
	if err := json.Unmarshal(fixture(t, "cmd-feed-state", "valid/populated"), &old); err != nil {
		t.Fatal(err)
	}
	if j := old.Jobs["J-1"]; j == nil || j.Kind != "" {
		t.Fatalf("old state J-1 = %+v, want a Job with no kind", j)
	}
	var withKind State
	if err := json.Unmarshal(fixture(t, "cmd-feed-state", "valid/kind"), &withKind); err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]string{"J-1": "code", "J-2": "user-docs"} {
		if j := withKind.Jobs[id]; j == nil || j.Kind != want {
			t.Fatalf("%s = %+v, want kind %q", id, j, want)
		}
	}
}
