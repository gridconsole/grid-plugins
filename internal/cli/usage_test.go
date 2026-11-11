package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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
		{"usage-post-lead", []string{"post", "--job", "lead", "--dollars", "20", "--minutes", "10", "--session", "session-1", "--model", "example-model"}},
		{"usage-show-totals", []string{"show"}},
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

func TestUsageTaskResolution(t *testing.T) {
	h, task := usageTask(t)
	if code := h.run(h.home, "usage", "post", "--job", "lead", "--dollars", "100", "--json", "--task", task); code != 0 {
		t.Fatal(code, h.errb)
	}
	var out usage.PostResult
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if out.Totals.Dollars != 100 || len(out.Notices) != 0 || len(out.Warnings) != 0 {
		t.Fatal(out)
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
	for _, op := range []string{"post"} {
		for _, value := range []string{"-1", "1.5", "lots", "1e3", "+1", "18446744073709551616"} {
			t.Run(op+"/"+value, func(t *testing.T) {
				h, task := usageTask(t)
				args := []string{"usage", "post", "--job", "lead"}
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

func TestTokenHelp(t *testing.T) {
	h, task := usageTask(t)
	if code := h.run(task, "usage", "--help"); code != 0 {
		t.Fatal(code, h.errb)
	}
	for _, text := range []string{"--tokens N", "nothing stops, holds or warns"} {
		if !strings.Contains(h.out.String(), text) {
			t.Fatalf("help omits %q", text)
		}
	}
	for _, text := range []string{"threshold", "budget", "PERCENT", "notice", "tokens_per_task", "workspace.md"} {
		if strings.Contains(h.out.String(), text) {
			t.Fatalf("help still names %q:\n%s", text, h.out)
		}
	}
	golden.Assert(t, "usage-help", h.out.String())
}

// The threshold and budget verbs are removed (owner's direction: usage is a number the
// window shows). They answer the unknown-command error, exit 2, and write nothing.
func TestUsageThresholdAndBudgetVerbsRemoved(t *testing.T) {
	for _, args := range [][]string{
		{"threshold"}, {"threshold", "show"}, {"threshold", "set", "--dollars", "80"}, {"threshold", "clear"},
		{"threshold", "clear", "--tokens", "--json"},
		{"budget"}, {"budget", "show"}, {"budget", "show", "--workspace"}, {"budget", "show", "--json"},
		{"budget", "set", "--dollars", "200"}, {"budget", "clear"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h, task := usageTask(t)
			if code := h.run(task, append([]string{"usage"}, args...)...); code != 2 || !strings.Contains(h.errb.String(), `unknown command "`+args[0]+`"`) || h.out.Len() != 0 {
				t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out, h.errb)
			}
			if _, err := os.Stat(filepath.Join(task, "usage")); !os.IsNotExist(err) {
				t.Fatal("removed verb wrote data")
			}
		})
	}
}

// Old homes carry budgets, so the old 50/80/100 crossing points are exercised against a
// budget in the old thresholds.json and in workspace.md: a post still records the delta
// and writes no notice event and prints no usage-notice line.
func TestUsagePostAtOldThresholdsWritesNoNotice(t *testing.T) {
	for _, unit := range []string{"dollars", "minutes", "tokens"} {
		for _, source := range []string{"task", "workspace"} {
			if source == "workspace" && unit != "tokens" {
				continue
			}
			t.Run(unit+"/"+source, func(t *testing.T) {
				h, task := usageTask(t)
				if source == "task" {
					h.write(filepath.Join(task, "usage", "thresholds.json"), `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"tokens":[50,80,100],"budget":{"`+unit+`":100},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{}}`)
				} else {
					h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 100\n---\n")
				}
				total := 0
				for _, step := range []int{49, 1, 29, 1, 19, 1, 50} { // 49, 50, 79, 80, 99, 100, 150 percent
					total += step
					if code := h.run(task, "usage", "post", "--job", "lead", "--"+unit, strconv.Itoa(step)); code != 0 {
						t.Fatalf("post at %d: exit %d: %s", total, code, h.errb)
					}
					if strings.Contains(h.out.String(), "usage-notice") || strings.Contains(h.out.String()+h.errb.String(), "budget") || h.errb.Len() != 0 {
						t.Fatalf("post at %d printed a notice: stdout %s stderr %s", total, h.out, h.errb)
					}
				}
				if code := h.run(task, "usage", "post", "--job", "lead", "--"+unit, "0", "--json"); code != 0 {
					t.Fatal(code, h.errb)
				}
				var out usage.PostResult
				if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
					t.Fatal(err)
				}
				if len(out.Notices) != 0 || len(out.Warnings) != 0 {
					t.Fatalf("post result carries notices: %+v", out)
				}
				var got float64
				switch unit {
				case "dollars":
					got = out.Totals.Dollars
				case "minutes":
					got = out.Totals.Minutes
				default:
					got = float64(out.Totals.Tokens)
				}
				if got != float64(total) || out.Totals.Posts != 8 {
					t.Fatalf("usage was not recorded as a number: %+v", out.Summary)
				}
				read, err := (feed.Store{TaskDir: task, TaskID: "01ARZ3NDEKTSV4RRFFQ69G5FAV"}).Read(feed.Filter{Type: "usage-notice"})
				if err == nil && len(read.Records) != 0 {
					t.Fatalf("notice events written: %+v", read.Records)
				}
			})
		}
	}
}

// A task from an old home keeps a budget in usage/thresholds.json (even one the old reader
// refused), a usage_notice_thresholds key in workflow.md (even an invalid one) and a
// tokens_per_task default. Reads and posts ignore all of it and succeed without a warning.
func TestUsageIgnoresOldBudgetsAndThresholds(t *testing.T) {
	for _, tc := range []struct{ name, thresholds, workflow string }{
		{"valid-budget", `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"tokens":[80,100],"budget":{"dollars":10,"tokens":5},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{}}`, "---\nusage_notice_thresholds: {tokens: [90, 100]}\n---\n"},
		{"sparse-v2", `{"v":2,"tokens":[],"budget":{"minutes":3},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{"tokens:80":"01ARZ3NDEKTSV4RRFFQ69G5FAV"}}`, "---\nusage_notice_thresholds: {dollars: [10, 100], minutes: [100]}\n---\n"},
		{"unreadable-config", `not json`, "---\nusage_notice_thresholds: {dollars: [0, 7], rubbish: [1]}\n---\n"},
		{"invalid-key", `{"v":9,"budget":{"dollars":"x"}}`, "---\nusage_notice_thresholds: nonsense\n---\n"},
		{"no-front-matter", `{}`, "usage_notice_thresholds: {tokens: [90, 100]}\n"},
		{"unclosed-front-matter", ``, "---\nusage_notice_thresholds: {tokens: [90, 100]}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, task := usageTask(t)
			h.write(filepath.Join(task, "usage", "thresholds.json"), tc.thresholds)
			h.write(filepath.Join(task, "workflow.md"), tc.workflow)
			h.write(filepath.Join(h.gridHome, "workspace.md"), "---\ntokens_per_task: 1\n---\n")
			h.write(filepath.Join(task, "usage", "usage.jsonl"), `{"v":1,"at":"2026-01-02T03:04:05.678Z","job":"lead","session":"","model":"","dollars":1,"minutes":2,"tokens":3}`+"\n")
			if code := h.run(task, "usage", "show", "--json"); code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), `"totals":{"dollars":1,"minutes":2,"tokens":3,"posts":1}`) {
				t.Fatalf("show: exit %d, stdout %s, stderr %s", code, h.out, h.errb)
			}
			if code := h.run(task, "usage", "post", "--job", "lead", "--dollars", "20", "--minutes", "20", "--tokens", "20"); code != 0 || h.errb.Len() != 0 || strings.Contains(h.out.String(), "usage-notice") || !strings.Contains(h.out.String(), "total  dollars 21  minutes 22  tokens 23  posts 2") {
				t.Fatalf("post: exit %d, stdout %s, stderr %s", code, h.out, h.errb)
			}
			after, err := os.ReadFile(filepath.Join(task, "usage", "thresholds.json"))
			if err != nil || string(after) != tc.thresholds {
				t.Fatalf("old config was rewritten: %q %v", after, err)
			}
		})
	}
}
