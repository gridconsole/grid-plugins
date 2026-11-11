package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// Row 6: the seeded Feed holds J-1, ready and never picked up, and J-2 behind
// it. job tree prints the orphan line under J-1 only, and --json carries
// `orphan` on J-1 only.
func TestJobTreePrintsOrphanLine(t *testing.T) {
	h, path, _ := orchestratorHarness(t)
	body := workerNextBody(t, "J-2")
	body["after"] = []any{"J-1"}
	raw, _ := json.Marshal(body)
	if code := h.run(path, "job", "post", "--type", "job", "--body", string(raw)); code != 0 {
		t.Fatalf("post J-2: %d %s", code, h.errb)
	}
	time.Sleep(5 * time.Millisecond)
	h.out.Reset()
	if code := h.run(path, "job", "tree"); code != 0 {
		t.Fatalf("%d %s", code, h.errb)
	}
	lines := strings.Split(strings.TrimRight(h.out.String(), "\n"), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "J-1  ") || !strings.HasPrefix(lines[1], "  orphan never_picked_up since 20") || !strings.HasSuffix(lines[1], ": ready and not assigned to any worker") || !strings.HasPrefix(lines[2], "J-2  ") {
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
	orphan, _ := rows[0]["orphan"].(map[string]any)
	if rows[0]["id"] != "J-1" || orphan["kind"] != "never_picked_up" || orphan["since"] == "" || orphan["detail"] == "" {
		t.Fatalf("J-1 row: %v", rows[0])
	}
	if _, has := rows[1]["orphan"]; has || rows[1]["id"] != "J-2" {
		t.Fatalf("J-2 row carries orphan: %v", rows[1])
	}
}
