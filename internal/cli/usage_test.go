package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
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
		want := `{"v":1,"dollars":[50,80,100],"minutes":[50,80,100],"tokens":[80,100],"budget":{"tokens":` + budget + `},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{},"token_budget_source":"` + source + `"}` + "\n"
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
	for _, text := range []string{"--tokens N", "--tokens PERCENT", "tokens_per_task", "workspace.md", "hold at the limit is not enforced yet", "worker and session hooks", "tokens 80%, 100%; dollars and minutes 50%, 80%, 100%"} {
		if !strings.Contains(h.out.String(), text) {
			t.Fatalf("help omits %q", text)
		}
	}
	golden.Assert(t, "usage-help", h.out.String())
}

func TestWorkspaceBudgetShowCLI(t *testing.T) {
	for _, tc := range []struct{ name, data, want string }{
		{"missing", "", "null"},
		{"no-key", "---\nname: workspace\n---\n", "null"},
		{"valid", "---\ntokens_per_task: 100\n---\n", "100"},
		{"zero", "---\ntokens_per_task: 0\n---\n", "null"},
		{"negative", "---\ntokens_per_task: -1\n---\n", "null"},
		{"exponent", "---\ntokens_per_task: 1e3\n---\n", "null"},
		{"linked", "---\ntokens_per_task: 100\n---\n", "null"},
		{"oversized", "---\ntokens_per_task: 100\n---\n" + strings.Repeat("x", 65537), "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("GRID_ROLE", "")
			path := filepath.Join(h.gridHome, "workspace.md")
			if tc.name == "linked" {
				h.write(filepath.Join(h.gridHome, "other.md"), tc.data)
				if err := os.Symlink("other.md", path); err != nil {
					t.Fatal(err)
				}
			} else if tc.name != "missing" {
				h.write(path, tc.data)
			}
			if code := h.run(h.home, "usage", "budget", "show", "--workspace", "--json"); code != 0 || h.out.String() != "{\"tokens_per_task\":"+tc.want+"}\n" {
				t.Fatalf("workspace show: exit %d, stdout %q, stderr %q", code, h.out, h.errb)
			}
			want := tc.want
			if want == "null" {
				want = "none"
			}
			if code := h.run(h.home, "usage", "budget", "show", "--workspace"); code != 0 || h.out.String() != "tokens_per_task "+want+"\n" {
				t.Fatalf("plain workspace show: exit %d, %s", code, h.out)
			}
		})
	}
}

func TestWorkspaceBudgetMutationCLI(t *testing.T) {
	h := newHarness(t)
	t.Setenv("GRID_ROLE", "")
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"set", "--tokens", "100"}, "100"},
		{[]string{"clear", "--tokens"}, "null"},
		{[]string{"set", "--tokens", "18446744073709551615"}, "18446744073709551615"},
		{[]string{"clear"}, "null"},
	} {
		args := append([]string{"usage", "budget"}, tc.args...)
		args = append(args, "--workspace", "--json")
		if code := h.run(h.home, args...); code != 0 || h.out.String() != "{\"tokens_per_task\":"+tc.want+"}\n" {
			t.Fatalf("workspace mutation: exit %d, stdout %q, stderr %q", code, h.out, h.errb)
		}
	}
}

func TestWorkspaceBudgetFlagsCLI(t *testing.T) {
	for _, args := range [][]string{
		{"show", "--task", "T-01"}, {"show", "--task="},
		{"set", "--tokens", "100", "--task", "T-01"},
		{"set", "--dollars", "1"}, {"set", "--minutes", "1"}, {"set"},
		{"set", "--tokens", "0"}, {"set", "--tokens", "-1"}, {"set", "--tokens", "1e3"},
		{"set", "--tokens", "1", "--tokens", "2"},
		{"clear", "--dollars"}, {"clear", "--minutes=false"}, {"clear", "--tokens=false"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("GRID_ROLE", "guest") // Syntax errors precede authorization and I/O.
			argv := append([]string{"usage", "budget"}, args...)
			argv = append(argv, "--workspace", "--json")
			if code := h.run(h.home, argv...); code != 2 {
				t.Fatalf("invalid workspace flags: exit %d: %s", code, h.errb)
			}
			entries, err := os.ReadDir(h.gridHome)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid flags touched home: %v %v", entries, err)
			}
		})
	}
}

func TestWorkspaceBudgetRolesCLI(t *testing.T) {
	for _, role := range []string{"guest", "unknown", "member", "owner", "", " owner "} {
		for _, op := range []string{"show", "set", "clear"} {
			t.Run(role+"/"+op, func(t *testing.T) {
				h := newHarness(t)
				t.Setenv("GRID_ROLE", role)
				args := []string{"usage", "budget", op, "--workspace", "--json"}
				if op == "set" {
					args = append(args, "--tokens", "100")
				}
				want := 0
				if role == "guest" || role == "unknown" || role == "member" && op != "show" {
					want = 3
				}
				if code := h.run(h.home, args...); code != want {
					t.Fatalf("role %q %s: exit %d, want %d: %s", role, op, code, want, h.errb)
				}
				if want == 3 {
					entries, err := os.ReadDir(h.gridHome)
					if err != nil || len(entries) != 0 || h.out.Len() != 0 {
						t.Fatalf("denied role touched home: %v %v", entries, err)
					}
				}
			})
		}
	}
}

func TestWorkspaceBudgetRefusalsCLI(t *testing.T) {
	for _, tc := range []struct{ kind, message string }{
		{"linked", "workspace must be a regular file within the size limit"},
		{"directory", "workspace must be a regular file within the size limit"},
		{"invalid-utf8", "workspace must be UTF-8 within the size limit"},
		{"unparsable", "workspace front matter does not parse"},
		{"oversized", "workspace must be a regular file within the size limit"},
		{"lock-directory", "workspace lock is not a regular file"},
		{"read-only-home", "cannot save workspace budget"},
		{"read-only-home-no-lock", "cannot open workspace lock"},
	} {
		kind := tc.kind
		for _, op := range []string{"set", "clear"} {
			t.Run(kind+"/"+op, func(t *testing.T) {
				h := newHarness(t)
				t.Setenv("GRID_ROLE", "owner")
				path := filepath.Join(h.gridHome, "workspace.md")
				original := "---\ntokens_per_task: 100\n---\nBody\n"
				switch kind {
				case "linked":
					h.write(filepath.Join(h.gridHome, "other.md"), original)
					if err := os.Symlink("other.md", path); err != nil {
						t.Fatal(err)
					}
				case "directory":
					h.mkdir(path)
				case "invalid-utf8":
					original += "\xff"
					h.write(path, original)
				case "unparsable":
					original = "---\ntokens_per_task: 100\n"
					h.write(path, original)
				case "oversized":
					original += strings.Repeat("x", 65537)
					h.write(path, original)
				case "lock-directory":
					h.write(path, original)
					h.mkdir(filepath.Join(h.gridHome, ".workspace.lock"))
				case "read-only-home", "read-only-home-no-lock":
					if runtime.GOOS == "windows" || os.Geteuid() == 0 {
						t.Skip("requires unprivileged Unix permissions")
					}
					h.write(path, original)
					if kind == "read-only-home" {
						h.write(filepath.Join(h.gridHome, ".workspace.lock"), "")
					}
					if err := os.Chmod(h.gridHome, 0o555); err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := os.Chmod(h.gridHome, 0o755); err != nil {
							t.Error(err)
						}
					})
				}
				before, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				args := []string{"usage", "budget", op, "--workspace", "--json"}
				if op == "set" {
					args = append(args, "--tokens", "200")
				}
				if code := h.run(h.home, args...); code != 1 || h.out.Len() != 0 {
					t.Errorf("unsafe workspace: exit %d, want 1: %s", code, h.errb)
				}
				// Exact diagnostics also ensure one stderr line with no private path.
				if want := "usage budget: " + tc.message + "\n"; h.errb.String() != want {
					t.Errorf("stderr = %q, want %q", h.errb.String(), want)
				}
				after, err := os.Lstat(path)
				if err != nil || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
					t.Fatal("refusal changed workspace")
				}
				if kind != "directory" {
					data, err := os.ReadFile(path)
					if err != nil || string(data) != original {
						t.Fatal("refusal changed workspace bytes")
					}
				}
			})
		}
	}
}

func TestWorkspaceBudgetReplaceablePoliciesCLI(t *testing.T) {
	h := newHarness(t)
	// A role the person gate accepts, so a write reaches the replaced policy and the store.
	t.Setenv("GRID_ROLE", "member")
	read, write := usage.WorkspaceBudgetReadAllowed, usage.WorkspaceBudgetWriteAllowed
	t.Cleanup(func() { usage.WorkspaceBudgetReadAllowed, usage.WorkspaceBudgetWriteAllowed = read, write })
	readCalls, writeCalls := 0, 0
	usage.WorkspaceBudgetReadAllowed = func(role string) bool { readCalls++; return role == "member" }
	usage.WorkspaceBudgetWriteAllowed = func(role string) bool { writeCalls++; return role == "member" }
	for _, op := range []string{"set", "show", "clear"} {
		args := []string{"usage", "budget", op, "--workspace", "--json"}
		if op == "set" {
			args = append(args, "--tokens", "100")
		}
		if code := h.run(h.home, args...); code != 0 {
			t.Fatalf("replacement policy: exit %d: %s", code, h.errb)
		}
	}
	if readCalls != 1 || writeCalls != 2 {
		t.Fatalf("policy calls: read %d, write %d", readCalls, writeCalls)
	}
}

// Bypass flag validation to exercise the store's input-error classification.
func TestWorkspaceBudgetStoreErrorsCLI(t *testing.T) {
	for _, tc := range []struct {
		name, role, message string
		value               uint64
		code                int
	}{
		{"zero", "owner", "workspace token budget must be at least 1", 0, 2},
		{"missing-home", "owner", "cannot find the Grid home; set GRID_HOME", 100, 1},
		{"guest", "guest", "caller role is not allowed to access the workspace budget", 100, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			t.Setenv("GRID_ROLE", tc.role)
			h.env.GridHome = ""
			if code := runWorkspaceBudgetGated("set", &tc.value, true, h.env, func() int { return 0 }); code != tc.code {
				t.Errorf("store error: exit %d, want %d", code, tc.code)
			}
			if want := "usage budget: " + tc.message + "\n"; h.errb.String() != want || h.out.Len() != 0 {
				t.Fatalf("stdout %q, stderr %q, want %q", h.out, h.errb, want)
			}
		})
	}
}
