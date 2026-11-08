package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/session"
	"github.com/gridconsole/grid-plugins/internal/usage"
)

func usageTask(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	task := filepath.Join(h.gridHome, "projects", "example", "tasks", "T-01-usage")
	h.write(filepath.Join(task, "task.md"), "---\nid: 01ARZ3NDEKTSV4RRFFQ69G5FAV\n---\n")
	h.write(filepath.Join(h.gridHome, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAW\n")
	t.Setenv("GRID_MEMBER", "example")
	t.Setenv("GRID_ROLE", "worker") // usage notices are written by the CLI itself.
	return h, task
}

func usageJSON(s string) string {
	s = regexp.MustCompile(`"id":"[0-7][0-9A-HJKMNP-TV-Z]{25}"`).ReplaceAllString(s, `"id":"<ID>"`)
	s = regexp.MustCompile(`"at":"[^"]+"`).ReplaceAllString(s, `"at":"<AT>"`)
	return regexp.MustCompile(`("(?:dollars|minutes):[^"]+":)"[0-7][0-9A-HJKMNP-TV-Z]{25}"`).ReplaceAllString(s, `${1}"<ID>"`)
}

func TestUsageGolden(t *testing.T) {
	h, task := usageTask(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"usage-show-empty", []string{"show"}},
		{"usage-threshold-defaults", []string{"threshold", "show"}},
		{"usage-budget-set", []string{"budget", "set", "--dollars", "200", "--minutes", "120"}},
		{"usage-threshold-set", []string{"threshold", "set", "--dollars", "80", "--dollars", "50"}},
		{"usage-post-lead", []string{"post", "--job", "lead", "--dollars", "20", "--minutes", "10", "--session", "session-1", "--model", "example-model"}},
		{"usage-post-notices", []string{"post", "--job", "J-1.2", "--dollars", "80", "--minutes", "50"}},
		{"usage-show-totals", []string{"show"}},
		{"usage-threshold-fired", []string{"threshold", "show"}},
		{"usage-threshold-clear", []string{"threshold", "clear", "--dollars"}},
		{"usage-budget-clear", []string{"budget", "clear"}},
	} {
		if code := h.run(task, append(append([]string{"usage"}, tc.args...), "--json")...); code != 0 {
			t.Fatalf("%s: exit %d: %s", tc.name, code, h.errb)
		}
		golden.Assert(t, tc.name, usageJSON(h.out.String()))
	}
}

type usageUnreadableStdin struct{}

func (usageUnreadableStdin) Read([]byte) (int, error) { panic("usage must not read stdin") }

func TestUsageCLI(t *testing.T) {
	for _, tc := range []struct {
		name    string
		args    []string
		code    int
		message string
	}{
		{"help", nil, 0, "Usage:"},
		{"subhelp", []string{"post", "--help"}, 0, "Usage"},
		{"unknown", []string{"other"}, 2, "unknown"},
		{"missing-amount", []string{"post", "--job", "J-1"}, 1, "at least one"},
		{"negative", []string{"post", "--job", "J-1", "--dollars", "-1"}, 1, "non-negative"},
		{"nan", []string{"post", "--job", "J-1", "--dollars", "NaN"}, 1, "finite"},
		{"inf", []string{"post", "--job", "J-1", "--minutes", "Inf"}, 1, "finite"},
		{"no-job", []string{"post", "--dollars", "1"}, 1, "job must"},
		{"bad-job", []string{"post", "--job", "J-01", "--dollars", "1"}, 1, "job must"},
		{"missing-value", []string{"post", "--dollars"}, 2, "needs an argument"},
		{"stdin-value", []string{"post", "--job", "lead", "--dollars", "-"}, 2, "expected a number"},
		{"stdin-operand", []string{"post", "-"}, 2, "unexpected argument"},
		{"unknown-flag", []string{"post", "--prompt", "text"}, 2, "not defined"},
		{"transcript-flag", []string{"post", "--transcript", "file"}, 2, "not defined"},
		{"duplicate-amount", []string{"post", "--job", "lead", "--dollars", "1", "--dollars", "2"}, 2, "only once"},
		{"lead-minutes", []string{"post", "--job", "lead", "--minutes=2"}, 0, "lead  dollars 0  minutes 2  posts 1"},
		{"zero-dollars", []string{"post", "--job", "J-1", "--dollars=0"}, 0, "posts 1"},
		{"extra-operand", []string{"show", "extra"}, 2, "unexpected argument"},
		{"end-flags", []string{"show", "--", "--json"}, 2, "unexpected argument"},
		{"unknown-task", []string{"show", "--task", "T-999"}, 1, "T-999"},
		{"cap", []string{"threshold", "set", "--dollars", "101"}, 1, "schema limit is 100 percent"},
		{"budget-over-100", []string{"budget", "set", "--dollars", "200"}, 0, "budget 200"},
		{"zero-budget", []string{"budget", "set", "--minutes", "0"}, 1, "positive"},
		{"multiple-budgets", []string{"budget", "set", "--dollars", "200", "--dollars", "300"}, 2, "only once"},
		{"empty-set", []string{"threshold", "set"}, 1, "supply --dollars"},
		{"clear-false", []string{"threshold", "clear", "--dollars=false"}, 1, "cannot be false"},
		{"clear-value", []string{"threshold", "clear", "--dollars", "50"}, 2, "unexpected argument"},
		{"unknown-config-command", []string{"threshold", "other"}, 2, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, task := usageTask(t)
			h.env.Stdin = usageUnreadableStdin{}
			code := h.run(task, append([]string{"usage"}, tc.args...)...)
			if code != tc.code || !strings.Contains(h.out.String()+h.errb.String(), tc.message) {
				t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out, h.errb)
			}
		})
	}
}

func TestUsageTaskResolutionAndWarnings(t *testing.T) {
	h, task := usageTask(t)
	for _, args := range [][]string{
		{"budget", "set", "--dollars", "200"},
		{"post", "--job", "lead", "--dollars", "100", "--json"},
	} {
		if code := h.run(h.home, append(append([]string{"usage"}, args...), "--task", task)...); code != 0 {
			t.Fatal(code, h.errb)
		}
	}
	var out usage.PostResult
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Notices) != 1 || out.Notices[0].Author.Role != "cli" {
		t.Fatal(out)
	}
	// Make the Feed unavailable only after the log has been initialized.
	if err := os.RemoveAll(filepath.Join(task, "feed")); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(task, "feed"), "unavailable")
	if code := h.run(task, "usage", "post", "--job", "lead", "--dollars", "60", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Totals.Dollars != 160 || len(out.Warnings) != 1 || !strings.Contains(h.errb.String(), "warning") {
		t.Fatalf("%+v %s", out, h.errb)
	}
	if err := os.Remove(filepath.Join(task, "feed")); err != nil {
		t.Fatal(err)
	}
	if code := h.run(task, "usage", "post", "--job", "lead", "--dollars", "0"); code != 0 || !strings.Contains(h.out.String(), "80% of dollars budget") {
		t.Fatal(code, h.out, h.errb)
	}
	read, err := (feed.Store{TaskDir: task, TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}).Read(feed.Filter{Type: "usage-notice"})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("%+v %v", read, err)
	}
	if code := h.run(h.home, "usage", "show"); code != 1 {
		t.Fatal("task required", code)
	}
}

func TestUsageCommandFoldsLocalRecords(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	cwd := filepath.Join(home, "workspace")
	if err := os.Mkdir(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	s := session.Store{TaskDir: home}
	if err := s.RecordUsage("session-demo", "tool-demo", session.Usage{Input: 7, Output: 3}); err != nil {
		t.Fatal(err)
	}
	var out, log bytes.Buffer
	code := Run([]string{"usage", "--json"}, Env{Stdout: &out, Stderr: &log, Home: home, GridHome: home, Cwd: cwd, Ceiling: home, Getenv: func(string) string { return "" }})
	if code != 0 || log.Len() != 0 || !strings.Contains(out.String(), `"input_tokens":7`) {
		t.Fatalf("%d %s %s", code, out.String(), log.String())
	}
}
