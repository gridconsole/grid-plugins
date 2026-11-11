package orchestrator

import (
	"encoding/json"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// exitedRow posts one assign, ack and worker-exit with the given body (nil is an
// empty body) and returns J-1's tree row as `grid job tree --json` prints it.
func exitedRow(t *testing.T, exit map[string]any) map[string]any {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	if exit == nil {
		exit = map[string]any{}
	}
	j := f.state(t)
	if _, err := Write(f.s.Feed, f.worker, feed.Input{Type: "worker-exit", Job: j.ID, Attempt: j.Attempt, Body: exit}); err != nil {
		t.Fatal(err)
	}
	return jsonRow(t, f)
}

func jsonRow(t *testing.T, f *fixture) map[string]any {
	t.Helper()
	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range Tree(snap) {
		if r.ID != "J-1" {
			continue
		}
		data, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var row map[string]any
		if err := json.Unmarshal(data, &row); err != nil {
			t.Fatal(err)
		}
		return row
	}
	t.Fatal("no row J-1")
	return nil
}

func firstAttempt(t *testing.T, row map[string]any) map[string]any {
	t.Helper()
	attempts, _ := row["attempts"].([]any)
	if len(attempts) != 1 {
		t.Fatalf("attempts: %v", row["attempts"])
	}
	return attempts[0].(map[string]any)
}

// Row 5: the attempt row carries the code, and the Job carries it as exit_code.
func TestTreeAttemptCarriesTheExitCode(t *testing.T) {
	row := exitedRow(t, map[string]any{"reason": "cancelled", "code": "prepare_failed"})
	a := firstAttempt(t, row)
	if a["state"] != "exited" || a["exit"] != "cancelled" || a["code"] != "prepare_failed" {
		t.Fatalf("attempt %v: want exited, cancelled, code prepare_failed", a)
	}
	if _, has := a["reason_missing"]; has {
		t.Fatalf("a body with a reason must not set reason_missing: %v", a)
	}
	if row["exit_code"] != "prepare_failed" || row["exit_reason"] != "cancelled" {
		t.Fatalf("Job row %v: want exit_code prepare_failed", row)
	}
	if a := firstAttempt(t, exitedRow(t, map[string]any{"reason": "crash", "code": 1})); a["code"] != "1" {
		t.Fatalf("integer code 1 reads as the text 1: %v", a)
	}
}

// An empty body reads as crash and says the reason was missing.
func TestTreeAttemptSaysWhenTheReasonWasMissing(t *testing.T) {
	row := exitedRow(t, nil)
	a := firstAttempt(t, row)
	if a["exit"] != "crash" || a["reason_missing"] != true {
		t.Fatalf("attempt %v: want exit crash and reason_missing true", a)
	}
	if _, has := a["code"]; has {
		t.Fatalf("no code in the body, no code on the attempt: %v", a)
	}
	if row["exit_reason"] != "crash" || row["exit_reason_missing"] != true {
		t.Fatalf("Job row %v: want exit_reason crash and exit_reason_missing true", row)
	}
}

// A Job with no worker-exit prints none of the new keys, so its row is the row
// main printed.
func TestTreeRowWithoutExitHasNoExitCodeKeys(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	if out := f.tick(t); out.Assigned != 1 {
		t.Fatal(out)
	}
	f.ack(t)
	row := jsonRow(t, f)
	a := firstAttempt(t, row)
	for _, key := range []string{"code", "reason_missing"} {
		if _, has := a[key]; has {
			t.Fatalf("attempt carries %s with no worker-exit: %v", key, a)
		}
	}
	for _, key := range []string{"exit_code", "exit_reason_missing"} {
		if _, has := row[key]; has {
			t.Fatalf("row carries %s with no worker-exit: %v", key, row)
		}
	}
}
