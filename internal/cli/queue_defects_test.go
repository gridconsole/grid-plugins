package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func jobBodyWith(t *testing.T, id string, set map[string]any) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal([]byte(personBody(t, id, nil)), &body); err != nil {
		t.Fatal(err)
	}
	for k, v := range set {
		body[k] = v
	}
	out, _ := json.Marshal(body)
	return string(out)
}

// D6: grid job post reads the Job id from the body, as --person does; a Job id on
// the command line that differs from the body id is refused.
func TestJobPostTakesTheBodyID(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--type", "job", "--body", personBody(t, "J-2", nil)); code != 0 {
		t.Fatalf("a body id alone: %d %s", code, h.errb)
	}
	h.errb.Reset()
	if code := h.run(path, "job", "post", "J-3", "--type", "job", "--body", personBody(t, "J-4", nil)); code != 3 || !strings.Contains(h.errb.String(), "J-3") || !strings.Contains(h.errb.String(), "J-4") {
		t.Fatalf("a positional id that differs from the body: %d %s", code, h.errb)
	}
	h.errb.Reset()
	if code := h.run(path, "job", "post", "J-5", "--type", "job", "--body", personBody(t, "J-5", nil)); code != 0 {
		t.Fatalf("a positional id equal to the body id: %d %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if st.Jobs["J-2"] == nil || st.Jobs["J-5"] == nil || st.Jobs["J-3"] != nil || st.Jobs["J-4"] != nil {
		t.Fatalf("jobs on the Feed: %v", st.Jobs)
	}
}

// D7: the refusal of a Job body names the field that is wrong, not the last
// branch the body was tried against.
func TestJobPostSchemaErrorNamesMinTests(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	body := jobBodyWith(t, "J-2", map[string]any{"verify": map[string]any{"command": "go test ./...", "min_tests": 0}})
	code := h.run(path, "job", "post", "J-2", "--type", "job", "--body", body)
	if code != 3 || !strings.Contains(h.errb.String(), "min_tests") {
		t.Fatalf("%d %s", code, h.errb)
	}
}

// D8: an unknown after target is refused at post time (exit 3), and a Job that is
// already on the Feed behind a missing target does not stop job next or job blocked.
func TestJobPostUnknownAfterTarget(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "J-2", "--type", "job", "--body", jobBodyWith(t, "J-2", map[string]any{"after": []any{"J-9"}})); code != 3 || !strings.Contains(h.errb.String(), "J-9") {
		t.Fatalf("%d %s", code, h.errb)
	}
	// A record written before the check, or by hand, reaches the Feed anyway.
	body := map[string]any{}
	if err := json.Unmarshal([]byte(jobBodyWith(t, "J-2", map[string]any{"after": []any{"J-9"}})), &body); err != nil {
		t.Fatal(err)
	}
	w, err := orchestratorWriter(h.env, "task", os.Getenv("GRID_LEASE_EPOCH"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Post(feed.Input{Type: "job", Job: "J-2", Body: body}, w); err != nil {
		t.Fatal(err)
	}
	h.errb.Reset()
	h.out.Reset()
	if code := h.run(path, "job", "blocked", "--json"); code != 0 || !strings.Contains(h.out.String(), "J-9") {
		t.Fatalf("job blocked: %d %s %s", code, h.out, h.errb)
	}
	h.out.Reset()
	if code := h.run(path, "job", "next", "--json"); code != 0 || !strings.Contains(h.out.String(), `"id":"J-1"`) {
		t.Fatalf("job next: %d %s %s", code, h.out, h.errb)
	}
}

// D11: job result on a person Job is refused while a Job it waits for is not accepted.
func TestPersonResultWaitsForItsAfter(t *testing.T) {
	h, path, store := orchestratorHarness(t)
	t.Setenv("GRID_ROLE", "member")
	if code := h.run(path, "job", "post", "--person", "dana", "--body", personBody(t, "J-2", []any{"J-1"})); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	h.errb.Reset()
	if code := h.run(path, "job", "result", "J-2", "--body", `{"answer":"approve"}`); code != 3 || !strings.Contains(h.errb.String(), "J-1") {
		t.Fatalf("%d %s", code, h.errb)
	}
	st, err := store.State()
	if err != nil {
		t.Fatal(err)
	}
	if got := string(st.Jobs["J-2"].Status); got != "posted" {
		t.Fatalf("J-2 is %s after a refused answer", got)
	}
}
