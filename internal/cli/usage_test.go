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
	return regexp.MustCompile(`("(?:dollars|minutes|tokens):[^"]+":)"[0-7][0-9A-HJKMNP-TV-Z]{25}"`).ReplaceAllString(s, `${1}"<ID>"`)
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
		{"lead-minutes", []string{"post", "--job", "lead", "--minutes=2"}, 0, "lead  dollars 0  minutes 2  tokens 0  posts 1"},
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

func TestTokenUsageCLI(t *testing.T) {
	h, task := usageTask(t)
	for _, args := range [][]string{
		{"post", "--job", "lead", "--tokens", "7", "--session", "session-a"},
		{"post", "--job", "J-1", "--tokens", "3", "--session", "session-b"},
		{"post", "--job", "lead", "--tokens", "0"},
	} {
		if code := h.run(task, append([]string{"usage"}, args...)...); code != 0 {
			t.Fatalf("post tokens: exit %d: %s", code, h.errb)
		}
	}
	if code := h.run(task, "usage", "show", "--json"); code != 0 {
		t.Fatal(code, h.errb)
	}
	want := `{"jobs":[{"job":"J-1","dollars":0,"minutes":0,"tokens":3,"posts":1},{"job":"lead","dollars":0,"minutes":0,"tokens":7,"posts":2}],"totals":{"dollars":0,"minutes":0,"tokens":10,"posts":3}}` + "\n"
	if h.out.String() != want {
		t.Fatalf("token totals JSON: %s", h.out)
	}
	if code := h.run(task, "usage", "show"); code != 0 || !strings.Contains(h.out.String(), "tokens 10") {
		t.Fatalf("show tokens: exit %d: %s %s", code, h.out, h.errb)
	}
}

func TestTokenInvalidCLI(t *testing.T) {
	for _, op := range []string{"post", "budget"} {
		for _, value := range []string{"-1", "1.5", "lots", "1e3", "+1", "18446744073709551616"} {
			t.Run(op+"/"+value, func(t *testing.T) {
				h, task := usageTask(t)
				args := []string{"usage", "post", "--job", "lead"}
				if op == "budget" {
					args = []string{"usage", "budget", "set"}
				}
				if code := h.run(task, append(args, "--tokens", value)...); code != 2 || !strings.Contains(h.errb.String(), "non-negative integer") {
					t.Fatalf("invalid tokens: exit %d: %s", code, h.errb)
				}
				if _, err := os.Stat(filepath.Join(task, "usage")); !os.IsNotExist(err) {
					t.Fatal("invalid tokens wrote data")
				}
			})
		}
	}
}

func TestTokenBudgetCLI(t *testing.T) {
	h, task := usageTask(t)
	h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
	check := func(args []string, budget, source string) {
		t.Helper()
		if code := h.run(task, append(append([]string{"usage", "budget"}, args...), "--json")...); code != 0 {
			t.Fatalf("token budget: exit %d: %s", code, h.errb)
		}
		want := `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"tokens":[50,80,100],"budget":{"tokens":` + budget + `},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{},"token_budget_source":"` + source + `"}` + "\n"
		if h.out.String() != want {
			t.Fatalf("token budget JSON: %s", h.out)
		}
	}
	check([]string{"show"}, "100", "workspace")
	check([]string{"set", "--tokens", "200"}, "200", "task")
	check([]string{"show"}, "200", "task")
	check([]string{"clear", "--tokens"}, "100", "workspace")
	if code := h.run(task, "usage", "budget", "show"); code != 0 || !strings.Contains(h.out.String(), "tokens  budget 100") || !strings.Contains(h.out.String(), "source workspace") {
		t.Fatalf("token budget text: exit %d: %s %s", code, h.out, h.errb)
	}
	data, err := os.ReadFile(filepath.Join(task, "usage", "thresholds.json"))
	if err != nil || strings.Contains(string(data), `"budget":{"tokens"`) || strings.Contains(string(data), "workspace") {
		t.Fatalf("persisted inherited budget: %s %v", data, err)
	}
}

func TestTokenWorkspaceInvalidCLI(t *testing.T) {
	for _, value := range []string{"0", "-5", "1.5", "lots", "+5", "1e3", "[100]", "18446744073709551616", "missing", "linked", "oversized", "directory", "unreadable", "invalid-utf8"} {
		t.Run(value, func(t *testing.T) {
			h, task := usageTask(t)
			path := filepath.Join(h.gridHome, "workspace.md")
			switch value {
			case "missing":
			case "linked":
				h.write(filepath.Join(h.gridHome, "other.md"), "---\ntokens_per_task: 100\n---\n")
				if err := os.Symlink("other.md", path); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				h.write(path, "---\ntokens_per_task: 100\n---\n"+strings.Repeat("x", 64*1024+1))
			case "directory":
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				h.write(path, "---\ntokens_per_task: 100\n---\n")
				if err := os.Chmod(path, 0000); err != nil {
					t.Fatal(err)
				}
				defer func() { _ = os.Chmod(path, 0600) }()
				if _, err := os.ReadFile(path); err == nil {
					t.Skip("filesystem does not enforce read permissions")
				}
			case "invalid-utf8":
				h.write(path, "---\ntokens_per_task: 100\n---\n\xff")
			default:
				h.write(path, "---\ntokens_per_task: "+value+"\n---\n")
			}
			if code := h.run(task, "usage", "budget", "show", "--json"); code != 0 || !strings.Contains(h.out.String(), `"budget":{}`) || !strings.Contains(h.out.String(), `"token_budget_source":"none"`) {
				t.Fatalf("invalid workspace limit: exit %d: %s %s", code, h.out, h.errb)
			}
			if _, err := os.Stat(filepath.Join(task, "usage")); !os.IsNotExist(err) {
				t.Fatal("budget show wrote data")
			}
		})
	}
}

func TestTokenNoticeCLI(t *testing.T) {
	for _, source := range []string{"workspace", "task"} {
		t.Run(source, func(t *testing.T) {
			h, task := usageTask(t)
			if source == "workspace" {
				h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
			} else if code := h.run(task, "usage", "budget", "set", "--tokens", "100"); code != 0 {
				t.Fatalf("set token budget: exit %d: %s", code, h.errb)
			}
			if code := h.run(task, "usage", "threshold", "set", "--tokens", "80"); code != 0 {
				t.Fatalf("set token threshold: exit %d: %s", code, h.errb)
			}
			for i, amount := range []string{"79", "1", "1", "19", "1"} {
				if code := h.run(task, "usage", "post", "--job", "lead", "--tokens", amount, "--json"); code != 0 || h.errb.Len() != 0 {
					t.Fatalf("token notice post: exit %d: %s", code, h.errb)
				}
				var result struct {
					Notices []feed.Record `json:"notices"`
				}
				if err := json.Unmarshal(h.out.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if i == 1 {
					if len(result.Notices) != 1 || result.Notices[0].Body["threshold"] != float64(80) || result.Notices[0].Body["tokens"] != json.Number("80") {
						t.Fatalf("80%% notice: %s", h.out)
					}
				} else if len(result.Notices) != 0 {
					t.Fatalf("duplicate or early notice: %s", h.out)
				}
			}
			rows, err := (feed.Store{TaskDir: task, TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}).Read(feed.Filter{Type: "usage-notice"})
			if err != nil || len(rows.Records) != 1 || rows.Records[0].Verdict != "counted" {
				t.Fatalf("token notice admission: %+v %v", rows, err)
			}
		})
	}
}

func TestTokenConfigurationSelectors(t *testing.T) {
	h, task := usageTask(t)
	for _, args := range [][]string{
		{"budget", "set", "--tokens", "0"},
		{"budget", "set", "--tokens", "10", "--tokens", "20"},
		{"budget", "clear", "--tokens=false"},
	} {
		if code := h.run(task, append([]string{"usage"}, args...)...); code == 0 {
			t.Fatal("accepted invalid selector or budget")
		}
		if _, err := os.Stat(filepath.Join(task, "usage")); !os.IsNotExist(err) {
			t.Fatal("invalid selector wrote data")
		}
	}
	for _, args := range [][]string{
		{"budget", "set", "--tokens", "100", "--dollars", "200", "--minutes", "120"},
		{"budget", "clear", "--tokens"},
	} {
		if code := h.run(task, append(append([]string{"usage"}, args...), "--json")...); code != 0 {
			t.Fatal(code, h.errb)
		}
	}
	if !strings.Contains(h.out.String(), `"budget":{"dollars":200,"minutes":120}`) || !strings.Contains(h.out.String(), `"token_budget_source":"none"`) {
		t.Fatal(h.out)
	}
	if code := h.run(task, "usage", "threshold", "clear", "--tokens", "--json"); code != 0 || !strings.Contains(h.out.String(), `"tokens":[]`) {
		t.Fatal(code, h.out, h.errb)
	}
	if code := h.run(task, "usage", "budget", "clear", "--json"); code != 0 || !strings.Contains(h.out.String(), `"budget":{}`) {
		t.Fatal(code, h.out, h.errb)
	}
}

func TestTokenHelp(t *testing.T) {
	h, task := usageTask(t)
	if code := h.run(task, "usage", "--help"); code != 0 {
		t.Fatal(code, h.errb)
	}
	for _, text := range []string{"--tokens N", "--tokens PERCENT", "tokens_per_task", "workspace.md", "hold at the limit is not enforced yet", "worker and session hooks"} {
		if !strings.Contains(h.out.String(), text) {
			t.Fatalf("help omits %q", text)
		}
	}
}
