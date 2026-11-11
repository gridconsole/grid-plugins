package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

// J-1 is seeded; J-2 waits on it. After `job cancel J-1`, `job tree` names J-2
// as a dead wait in text and in --json, and J-1 has no orphan. No clock is
// involved: a dead wait is not a matter of time.
func TestGuDwTreeLine(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	body := workerNextBody(t, "J-2")
	body["after"] = []any{"J-1"}
	raw, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--type", "job", "--body", string(raw)); code != 0 {
		t.Fatalf("post J-2: %d %s", code, h.errb)
	}
	if code := h.run(path, "job", "cancel", "J-1", "--reason", "dropped"); code != 0 {
		t.Fatalf("cancel J-1: %d %s", code, h.errb)
	}
	h.out.Reset()
	if code := h.run(path, "job", "tree"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	lines := strings.Split(strings.TrimRight(h.out.String(), "\n"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "J-1  ") || !strings.HasPrefix(lines[1], "J-2  ") || !strings.HasPrefix(lines[2], "  orphan dead_wait since 20") || !strings.HasSuffix(lines[2], ": waiting for J-1, which was cancelled") {
		t.Fatalf("text tree:\n%s", h.out)
	}
	h.out.Reset()
	if code := h.run(path, "job", "tree", "--json"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil || len(rows) != 2 {
		t.Fatalf("%v %s", err, h.out)
	}
	if _, has := rows[0]["orphan"]; has || rows[0]["id"] != "J-1" {
		t.Fatalf("J-1 row carries orphan: %v", rows[0])
	}
	orphan, _ := rows[1]["orphan"].(map[string]any)
	if rows[1]["id"] != "J-2" || orphan["kind"] != "dead_wait" || orphan["since"] == "" || orphan["detail"] != "waiting for J-1, which was cancelled" {
		t.Fatalf("J-2 row: %v", rows[1])
	}
}
