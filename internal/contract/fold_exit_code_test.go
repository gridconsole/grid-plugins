package contract

import (
	"bytes"
	"encoding/json"
	"testing"
)

// foldExit folds an assign, an ack and one worker-exit with the given body (nil
// keeps the fixture's empty body) and returns the folded Job as state.json
// carries it, plus the marshalled bytes.
func foldExit(t *testing.T, exit any) (map[string]any, []byte) {
	t.Helper()
	x := side(t, "worker-exit", 4)
	if exit != nil {
		body(t, &x, exit)
	}
	job := Fold(append(base(t), x)).Jobs["J-1"]
	if job == nil || job.Exited == "" {
		t.Fatalf("worker-exit was not folded: %+v", job)
	}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	var row map[string]any
	if err := json.Unmarshal(data, &row); err != nil {
		t.Fatal(err)
	}
	return row, data
}

// Row 1 and 2: the code a worker-exit carries reaches the folded Job, string or
// integer, byte for byte.
func TestFoldKeepsWorkerExitCode(t *testing.T) {
	row, data := foldExit(t, map[string]any{"reason": "cancelled", "code": "prepare_failed"})
	if row["exit_reason"] != "cancelled" || row["exit_code"] != "prepare_failed" {
		t.Fatalf("worker-exit {reason cancelled, code prepare_failed}: the folded Job holds exit_reason %v exit_code %v (no field holds prepare_failed): %s", row["exit_reason"], row["exit_code"], data)
	}
	if _, has := row["exit_reason_missing"]; has {
		t.Fatalf("a body with a reason must not set exit_reason_missing: %s", data)
	}
	row, data = foldExit(t, map[string]any{"reason": "crash", "code": 1})
	if row["exit_reason"] != "crash" || exitCodeBytes(t, data) != "1" {
		t.Fatalf("integer code 1 must round-trip as 1 in state.json: %s", data)
	}
}

// exitCodeBytes returns the exact bytes state.json holds under exit_code.
func exitCodeBytes(t *testing.T, state []byte) string {
	t.Helper()
	var row map[string]json.RawMessage
	if err := json.Unmarshal(state, &row); err != nil {
		t.Fatal(err)
	}
	return string(row["exit_code"])
}

// Row 3: an empty body still reads as crash, and now says the reason was missing.
func TestFoldSaysWhenTheExitReasonWasMissing(t *testing.T) {
	for _, tc := range []struct {
		name string
		body any
	}{{"fixture body", nil}, {"empty object", map[string]any{}}, {"code only", map[string]any{"code": 7}}} {
		row, data := foldExit(t, tc.body)
		if row["exit_reason"] != "crash" || row["exit_reason_missing"] != true {
			t.Fatalf("%s: want exit_reason crash and exit_reason_missing true: %s", tc.name, data)
		}
	}
	row, data := foldExit(t, map[string]any{"reason": "done"})
	if _, has := row["exit_reason_missing"]; has || row["exit_reason"] != "done" {
		t.Fatalf("a body with a reason never sets exit_reason_missing: %s", data)
	}
	if _, has := row["exit_code"]; has {
		t.Fatalf("no code in the body, no exit_code in state: %s", data)
	}
}

// A code the post schema would refuse never reaches state.json, where the
// cmd-feed-state schema allows only an integer or a non-empty string.
func TestFoldDropsAnExitCodeThatIsNotIntegerOrText(t *testing.T) {
	for _, code := range []string{`null`, `""`, `true`, `1.5`, `{"a":1}`, `[1]`} {
		x := side(t, "worker-exit", 4)
		x.Body = json.RawMessage(`{"reason":"cancelled","code":` + code + `}`)
		data, err := json.Marshal(Fold(append(base(t), x)).Jobs["J-1"])
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(`"exit_code"`)) {
			t.Fatalf("code %s must not be kept: %s", code, data)
		}
	}
	x := side(t, "worker-exit", 4)
	x.Body = json.RawMessage(`{"reason":"cancelled","code":2.0}`)
	if data, _ := json.Marshal(Fold(append(base(t), x)).Jobs["J-1"]); exitCodeBytes(t, data) != "2" {
		t.Fatalf("a whole-number code 2.0 reads as the integer 2: %s", data)
	}
}

// Row 4: a new assign clears the code and the missing flag with exited and
// exit_reason, for a new attempt and for the usage_limit same-attempt retry.
func TestAssignClearsTheExitCode(t *testing.T) {
	for _, tc := range []struct {
		name    string
		attempt int
		exit    map[string]any
	}{
		{"new attempt after cancelled", 2, map[string]any{"reason": "cancelled", "code": "prepare_failed"}},
		{"new attempt after a missing reason", 2, map[string]any{}},
		{"usage_limit same attempt", 1, map[string]any{"reason": "usage_limit", "code": 3}},
	} {
		x := side(t, "worker-exit", 4)
		body(t, &x, tc.exit)
		a, k := retry(t, 5, tc.attempt, 3)
		posts := append(base(t), x, a, k)
		if before, _ := json.Marshal(Fold(posts[:4]).Jobs["J-1"]); !bytes.Contains(before, []byte(`"exited":"`+x.ID+`"`)) {
			t.Fatalf("%s: precondition, the exit was not folded: %s", tc.name, before)
		}
		job := Fold(posts).Jobs["J-1"]
		data, _ := json.Marshal(job)
		var row map[string]any
		_ = json.Unmarshal(data, &row)
		if row["exited"] != "" || row["exit_reason"] != "" || row["status"] != "running" || row["attempt"] != float64(tc.attempt) {
			t.Fatalf("%s: assign did not reset the Job: %s", tc.name, data)
		}
		for _, key := range []string{"exit_code", "exit_reason_missing"} {
			if _, has := row[key]; has {
				t.Fatalf("%s: assign left %s behind: %s", tc.name, key, data)
			}
		}
	}
}

// A Job with no worker-exit marshals exactly as it did before the two fields.
func TestJobWithoutExitHasNoExitCodeKeys(t *testing.T) {
	data, err := json.Marshal(Fold(base(t)).Jobs["J-1"])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`exit_code`)) || bytes.Contains(data, []byte(`exit_reason_missing`)) {
		t.Fatalf("a Job that never exited carries the new keys: %s", data)
	}
}
