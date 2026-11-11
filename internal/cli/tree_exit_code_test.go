package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// Row 5 and 6: J-1 is assigned, acked and exits cancelled with the code
// prepare_failed. grid job tree prints the code, --json carries it on the
// attempt and the Job, and grid feed state --json carries exit_code and still
// validates against cmd-feed-state. Before the exit the same commands print
// none of the new keys.
func TestJobTreeShowsTheWorkerExitCode(t *testing.T) {
	h, path, _, _, _ := assignedExitHarness(t, true)
	run := func(args ...string) string {
		t.Helper()
		h.out.Reset()
		h.errb.Reset()
		if code := h.run(path, args...); code != 0 {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
		return h.out.String()
	}
	for _, args := range [][]string{{"job", "tree", "--json"}, {"feed", "state", "--json"}} {
		out := run(args...)
		for _, key := range []string{`"exit_code"`, `"exit_reason_missing"`, `"code"`, `"reason_missing"`} {
			if strings.Contains(out, key) {
				t.Fatalf("%v before any worker-exit carries %s:\n%s", args, key, out)
			}
		}
	}
	run("job", "post", "J-1", "--attempt", "1", "--type", "worker-exit", "--body", `{"reason":"cancelled","code":"prepare_failed"}`)

	if text := run("job", "tree"); !strings.Contains(text, "  attempt 1  fence ") || !strings.Contains(text, "exited  cancelled code prepare_failed\n") || strings.Contains(text, "no reason given") {
		t.Fatalf("text tree:\n%s", text)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(run("job", "tree", "--json")), &rows); err != nil || len(rows) != 1 {
		t.Fatalf("%v %s", err, h.out)
	}
	attempt := rows[0]["attempts"].([]any)[0].(map[string]any)
	if attempt["exit"] != "cancelled" || attempt["code"] != "prepare_failed" || rows[0]["exit_code"] != "prepare_failed" {
		t.Fatalf("tree --json row %v", rows[0])
	}
	state := run("feed", "state", "--json")
	if err := contract.Validate("cmd-feed-state", []byte(state)); err != nil {
		t.Fatalf("cmd-feed-state: %v\n%s", err, state)
	}
	if !strings.Contains(state, `"exit_code":"prepare_failed"`) || !strings.Contains(state, `"exit_reason":"cancelled"`) {
		t.Fatalf("feed state --json lacks exit_code:\n%s", state)
	}
}
