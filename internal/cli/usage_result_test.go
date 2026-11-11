package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/usage"
)

const (
	resultSession1 = "5f1c2a9e-7b3d-4c1e-9a52-3d0e8f6b7a11"
	resultSession2 = "9b7e4d20-1a6c-4f83-8e55-0c2d7a9f3b42"
)

func resultFixture(t *testing.T, name string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "usage", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// resultRow is the usage.jsonl line as these tests read it.
type resultRow struct {
	Job     string  `json:"job"`
	Session string  `json:"session"`
	Model   string  `json:"model"`
	Key     string  `json:"key"`
	Dollars float64 `json:"dollars"`
	Minutes float64 `json:"minutes"`
	Tokens  *uint64 `json:"tokens"`
}

func usageLines(t *testing.T, task string) (string, []resultRow) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(task, "usage", "usage.jsonl"))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var rows []resultRow
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		var r resultRow
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, r)
	}
	return string(raw), rows
}

func TestUsageResultPostBooksOnce(t *testing.T) {
	h, task := usageTask(t)
	log1 := resultFixture(t, "attempt-1.log")
	if code := h.run(task, "usage", "post", "--task", "T-01", "--job", "J-2", "--result", log1, "--json"); code != 0 {
		t.Fatalf("first post: exit %d: %s", code, h.errb)
	}
	raw1, rows := usageLines(t, task)
	if len(rows) != 1 {
		t.Fatalf("want one row, got %d: %s", len(rows), raw1)
	}
	r := rows[0]
	if r.Job != "J-2" || r.Session != resultSession1 || r.Model != "claude-opus-4-1" || r.Key != "result:"+resultSession1 {
		t.Fatalf("row identity wrong: %+v", r)
	}
	if r.Dollars != 0.476067 || r.Minutes != 2.09 || r.Tokens == nil || *r.Tokens != 174412 {
		t.Fatalf("row amounts wrong: %+v", r)
	}
	if code := h.run(task, "usage", "show", "--task", "T-01", "--json"); code != 0 {
		t.Fatalf("show: exit %d: %s", code, h.errb)
	}
	var sum usage.Summary
	if err := json.Unmarshal(h.out.Bytes(), &sum); err != nil {
		t.Fatal(err)
	}
	if sum.Totals.Dollars != 0.476067 || sum.Totals.Minutes != 2.09 || sum.Totals.Tokens != 174412 || sum.Totals.Posts != 1 {
		t.Fatalf("show totals wrong: %s", h.out)
	}

	// the same log again: exit 0, no byte appended, the answer says it was booked
	if code := h.run(task, "usage", "post", "--task", "T-01", "--job", "J-2", "--result", log1, "--json"); code != 0 {
		t.Fatalf("repeat post: exit %d: %s", code, h.errb)
	}
	var again struct {
		AlreadyBooked bool `json:"alreadyBooked"`
		Totals        struct{ Posts int }
	}
	if err := json.Unmarshal(h.out.Bytes(), &again); err != nil || !again.AlreadyBooked || again.Totals.Posts != 1 {
		t.Fatalf("repeat answer: %v %s", err, h.out)
	}
	if raw2, _ := usageLines(t, task); raw2 != raw1 {
		t.Fatalf("repeat post changed usage.jsonl:\n%s\n%s", raw1, raw2)
	}

	// another attempt, another session id: a second row
	if code := h.run(task, "usage", "post", "--task", "T-01", "--job", "J-2", "--result", resultFixture(t, "attempt-2.log"), "--json"); code != 0 {
		t.Fatalf("second attempt: exit %d: %s", code, h.errb)
	}
	_, rows = usageLines(t, task)
	if len(rows) != 2 || rows[1].Session != resultSession2 || rows[1].Dollars != 0.1234 || rows[1].Minutes != 1 {
		t.Fatalf("want a second row for the second session: %+v", rows)
	}
}

func TestUsageResultPostRefusals(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	good := `{"type":"result","session_id":"s-1","total_cost_usd":0.5,"duration_ms":60000,"usage":{"input_tokens":1}}`
	for _, tc := range []struct {
		name string
		path string
	}{
		{"killed-run", write("killed.log", `{"type":"system","session_id":"s-1"}`+"\n"+`{"type":"assistant"}`+"\n")},
		{"unreadable", filepath.Join(dir, "absent.log")},
		{"no-cost", write("nocost.log", `{"type":"result","session_id":"s-1"}`+"\n")},
		{"negative-cost", write("negative.log", `{"type":"result","session_id":"s-1","total_cost_usd":-0.1}`+"\n")},
		{"no-session", write("nosession.log", `{"type":"result","total_cost_usd":0.1}`+"\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, task := usageTask(t)
			code := h.run(task, "usage", "post", "--job", "J-1", "--result", tc.path)
			if code != 1 || !strings.Contains(h.errb.String(), tc.path) {
				t.Fatalf("want exit 1 naming %s, got %d: %s", tc.path, code, h.errb)
			}
			if raw, _ := usageLines(t, task); raw != "" {
				t.Fatalf("a refused log appended: %s", raw)
			}
		})
	}
	t.Run("trailing-text", func(t *testing.T) {
		h, task := usageTask(t)
		p := write("trailing.log", good+"\n"+"Claude exited with code 0\n\n")
		if code := h.run(task, "usage", "post", "--job", "J-1", "--result", p); code != 0 {
			t.Fatalf("exit %d: %s", code, h.errb)
		}
		if _, rows := usageLines(t, task); len(rows) != 1 || rows[0].Dollars != 0.5 {
			t.Fatalf("rows: %+v", rows)
		}
	})
	t.Run("job-required", func(t *testing.T) {
		h, task := usageTask(t)
		if code := h.run(task, "usage", "post", "--result", resultFixture(t, "attempt-1.log")); code != 1 || !strings.Contains(h.errb.String(), "job must") {
			t.Fatalf("exit %d: %s", code, h.errb)
		}
		if raw, _ := usageLines(t, task); raw != "" {
			t.Fatalf("appended: %s", raw)
		}
	})
}

func TestUsageResultConflictsAndOverrides(t *testing.T) {
	log1 := resultFixture(t, "attempt-1.log")
	for _, flag := range [][]string{{"--dollars", "1"}, {"--minutes", "1"}, {"--tokens", "1"}} {
		t.Run(flag[0], func(t *testing.T) {
			h, task := usageTask(t)
			args := append([]string{"usage", "post", "--job", "J-1", "--result", log1}, flag...)
			if code := h.run(task, args...); code != 2 {
				t.Fatalf("want exit 2, got %d: %s", code, h.errb)
			}
			if raw, _ := usageLines(t, task); raw != "" {
				t.Fatalf("appended: %s", raw)
			}
		})
	}
	h, task := usageTask(t)
	if code := h.run(task, "usage", "post", "--job", "J-1", "--result", log1, "--session", "typed-session", "--model", "typed-model"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	_, rows := usageLines(t, task)
	if len(rows) != 1 || rows[0].Session != "typed-session" || rows[0].Model != "typed-model" || rows[0].Dollars != 0.476067 {
		t.Fatalf("typed values must override: %+v", rows)
	}
}
