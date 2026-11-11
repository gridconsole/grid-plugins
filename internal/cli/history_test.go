package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// historyTask makes a personal task whose id resolves for any caller role in
// h.env, with no role set in the environment.
func historyTask(t *testing.T) (*harness, string, string) {
	t.Helper()
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	tk := taskFixture(t, h)
	return h, tk.ID, tk.Path
}

func readHistory(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestHistoryAddAppendsLine(t *testing.T) {
	h, id, dir := historyTask(t)
	if code := h.run(h.home, "history", "add", "Checked the label", "--task", id); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); got != "History added\n" {
		t.Errorf("stdout %q, want %q", got, "History added\n")
	}
	if !strings.Contains(readHistory(t, dir), "Checked the label") {
		t.Errorf("history.md lacks the line:\n%s", readHistory(t, dir))
	}
	// --json prints the line and still appends it.
	if code := h.run(h.home, "history", "add", "Second line", "--task", id, "--json"); code != 0 {
		t.Fatalf("json exit %d: %s", code, h.errb)
	}
	var got map[string]string
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got["line"] != "Second line" {
		t.Errorf("json %q (%v), want line=Second line", h.out.String(), err)
	}
	if !strings.Contains(readHistory(t, dir), "Second line") {
		t.Errorf("history.md lacks the json line:\n%s", readHistory(t, dir))
	}
}

func TestHistoryAddReadBack(t *testing.T) {
	h, id, dir := historyTask(t)
	before := strings.Count(readHistory(t, dir), "\n")
	for _, line := range []string{"first line", "second line"} {
		if code := h.run(h.home, "history", "add", line, "--task", id); code != 0 {
			t.Fatalf("add %q exit %d: %s", line, code, h.errb)
		}
	}
	text := readHistory(t, dir)
	if got := strings.Count(text, "\n"); got != before+2 {
		t.Errorf("history.md has %d lines, want %d:\n%s", got, before+2, text)
	}
	if i, j := strings.Index(text, "first line"), strings.Index(text, "second line"); i < 0 || j < i {
		t.Errorf("lines out of order or missing:\n%s", text)
	}
	// The read verb of the task page sees the same lines.
	if code := h.run(h.home, "task", "history", id, "--json"); code != 0 {
		t.Fatalf("task history exit %d: %s", code, h.errb)
	}
	if out := h.out.String(); !strings.Contains(out, "first line") || !strings.Contains(out, "second line") {
		t.Errorf("task history lacks added lines: %s", out)
	}
}

func TestHistoryAddWithoutHistoryFile(t *testing.T) {
	h, id, dir := historyTask(t)
	if err := os.Remove(filepath.Join(dir, "history.md")); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if code := h.run(h.home, "history", "add", "First ever", "--task", id); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	text := readHistory(t, dir)
	if !strings.Contains(text, "First ever") || strings.Count(text, "\n") != 1 {
		t.Errorf("history.md after first add:\n%s", text)
	}
}

func TestHistoryAddUsageAndGuest(t *testing.T) {
	h, id, dir := historyTask(t)
	before := readHistory(t, dir)
	for name, args := range map[string][]string{
		"no subcommand": {"history", "bogus"},
		"no line":       {"history", "add", "--task", id},
		"two lines":     {"history", "add", "one", "two", "--task", id},
	} {
		if code := h.run(h.home, args...); code != 2 {
			t.Errorf("%s: exit %d, want 2", name, code)
		}
	}
	if readHistory(t, dir) != before {
		t.Error("a usage error changed history.md")
	}
	// A guest does not see a personal task, so the call is refused and writes nothing.
	h.env.Getenv = func(key string) string {
		if key == "GRID_ROLE" {
			return "guest"
		}
		return ""
	}
	if code := h.run(h.home, "history", "add", "guest line", "--task", id); code != 1 {
		t.Fatalf("guest exit %d, want 1: %s", code, h.errb)
	}
	if !strings.Contains(h.errb.String(), "task not found") || h.out.Len() != 0 {
		t.Errorf("guest answer stdout %q stderr %q", h.out.String(), h.errb.String())
	}
	if readHistory(t, dir) != before {
		t.Error("a guest call changed history.md")
	}
}
