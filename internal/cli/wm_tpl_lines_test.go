package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// The agent templates in agent-templates/ are synced into the user's agents. The
// tests here run the command lines they carry (the Job post, the result read and
// the proof add lines) as the CLI, so a line that stops working fails here.

var wmTplSpan = regexp.MustCompile("`([^`\n]+)`")
var wmTplKindChoice = regexp.MustCompile(`--kind ([a-z]+(?:\|[a-z]+)+)`)

// wmTplSpans returns the backticked `grid ...` spans of one agent-templates file
// that start with prefix and hold every word of has.
func wmTplSpans(t *testing.T, name, prefix string, has ...string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "agent-templates", name))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, m := range wmTplSpan.FindAllStringSubmatch(string(b), -1) {
		ok := strings.HasPrefix(m[1], "grid ") && strings.HasPrefix(m[1], prefix)
		for _, w := range has {
			ok = ok && strings.Contains(m[1], w)
		}
		if ok {
			out = append(out, m[1])
		}
	}
	return out
}

// wmTplArgv fills the placeholders of a span and splits it into the arguments
// after "grid", keeping a double-quoted run as one argument.
func wmTplArgv(t *testing.T, span string, vals map[string]string) []string {
	t.Helper()
	keys := make([]string, 0, len(vals))
	for k := range vals {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	line := span
	for _, k := range keys {
		if k != "<json>" { // a body has spaces: it replaces its argument after the split
			line = strings.ReplaceAll(line, k, vals[k])
		}
	}
	var args []string
	var cur strings.Builder
	quoted, has := false, false
	for _, r := range strings.TrimPrefix(line, "grid ") {
		switch {
		case r == '"':
			quoted, has = !quoted, true
		case r == ' ' && !quoted:
			if has {
				args = append(args, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if quoted {
		t.Fatalf("unbalanced quote in %q", span)
	}
	if has {
		args = append(args, cur.String())
	}
	for i, a := range args {
		if a == "<json>" {
			if body, ok := vals["<json>"]; ok {
				args[i], a = body, body
			}
		}
		if strings.Contains(a, "<") && strings.Contains(a, ">") && !strings.HasPrefix(a, "{") {
			t.Fatalf("%q: placeholder left unfilled in argument %q", span, a)
		}
	}
	return args
}

// Row 2(a): each planner line that posts a Job body posts a Job.
func TestWmTplPlannerPostsJobs(t *testing.T) {
	spans := wmTplSpans(t, "grid-planner.md", "grid job post", "--body <json>")
	if len(spans) == 0 {
		t.Fatal("grid-planner.md has no `grid job post ... --body <json>` line")
	}
	for _, span := range spans {
		span := span
		t.Run(span, func(t *testing.T) {
			h, path, _ := orchestratorHarness(t)
			raw, err := json.Marshal(workerNextBody(t, "J-2"))
			if err != nil {
				t.Fatal(err)
			}
			args := wmTplArgv(t, span, map[string]string{"<task>": feedTaskID, "<json>": string(raw)})
			if code := h.run(path, args...); code != 0 {
				t.Fatalf("%q exits %d, stderr: %s", span, code, strings.TrimSpace(h.errb.String()))
			}
			h.out.Reset()
			if code := h.run(path, "job", "tree", "--task", feedTaskID); code != 0 {
				t.Fatalf("job tree: %d %s", code, h.errb)
			}
			found := false
			for _, row := range strings.Split(h.out.String(), "\n") {
				found = found || strings.HasPrefix(row, "J-2")
			}
			if !found {
				t.Fatalf("job tree does not list J-2 after %q:\n%s", span, h.out)
			}
		})
	}
}

const wmTplResultText = "wmTpl posted result marker"

// Row 2(b): the line of the planner and of the reviewer that reads a Job's result
// prints the result that was posted.
func TestWmTplResultReadLines(t *testing.T) {
	for _, name := range []string{"grid-planner.md", "grid-reviewer.md"} {
		spans := wmTplSpans(t, name, "grid feed read", "--type result")
		if len(spans) == 0 {
			t.Errorf("%s has no `grid feed read ... --type result` line that reads a result", name)
			continue
		}
		for _, span := range spans {
			span := span
			t.Run(name+":"+span, func(t *testing.T) {
				h, path, _, _, _ := assignedExitHarness(t, true)
				t.Setenv("GRID_ATTEMPT", "1")
				body := `{"outcome":"` + wmTplResultText + `","changed":[],"evidence":[],"acceptance":[],"verified":null,"must_fix":[],"found":[],"proposed_jobs":[],"decisions_needed":[],"knowledge":[]}`
				if code := h.run(path, "job", "result", "--task", path, "--job", "J-1", "--body", body); code != 0 {
					t.Fatalf("job result: exit %d: %s", code, h.errb)
				}
				h.out.Reset()
				h.errb.Reset()
				args := wmTplArgv(t, span, map[string]string{"<task>": path, "<job>": "J-1"})
				if code := h.run(path, args...); code != 0 {
					t.Fatalf("%q: exit %d: %s", span, code, h.errb)
				}
				if !strings.Contains(h.out.String(), wmTplResultText) {
					t.Fatalf("%q does not print the posted result:\n%s", span, h.out)
				}
			})
		}
	}
}

// wmTplPlan has one plan item for each proof kind a template line can name.
const wmTplPlan = "# Plan 1\n- [x] 1 Check result\n  proof: check\n  jobs: []\n- [ ] 2 Show the page\n  proof: screenshot\n  jobs: []\n- [ ] 3 Record the flow\n  proof: video\n  jobs: []\n- [ ] 4 Keep the log\n  proof: log\n  jobs: []\n"

var wmTplKindItem = map[string]int{"check": 1, "screenshot": 2, "video": 3, "log": 4}
var wmTplKindExt = map[string]string{"check": ".txt", "screenshot": ".png", "video": ".mp4", "log": ".log"}

// Row 4: every `grid proof add` line of the templates saves proof for its item
// when run outside the task folder, once for each kind a choice offers.
func TestWmTplProofAddLines(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "agent-templates", "grid-*.md"))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]int{}
	for _, file := range files {
		name := filepath.Base(file)
		for _, span := range wmTplSpans(t, name, "grid proof add") {
			seen[name]++
			kinds := []string{"check"} // a --text line is a check
			if m := wmTplKindChoice.FindStringSubmatch(span); m != nil {
				kinds = strings.Split(m[1], "|")
			} else if m := regexp.MustCompile(`--kind ([a-z]+)`).FindStringSubmatch(span); m != nil {
				kinds = []string{m[1]}
			}
			for _, kind := range kinds {
				name, span, kind := name, span, kind
				t.Run(name+":"+kind+":"+span, func(t *testing.T) {
					h := newHarness(t)
					h.env.Getenv = func(string) string { return "" }
					tk := taskFixture(t, h)
					h.write(filepath.Join(tk.Path, "plan.md"), wmTplPlan)
					evidence := filepath.Join(t.TempDir(), "evidence"+wmTplKindExt[kind])
					h.write(evidence, "evidence bytes")
					line := span
					if wmTplKindChoice.MatchString(line) {
						line = wmTplKindChoice.ReplaceAllString(line, "--kind "+kind)
					}
					item := wmTplKindItem[kind]
					args := wmTplArgv(t, line, map[string]string{
						"<task>": tk.ID, "<item>": strconv.Itoa(item), "<file>": evidence, "<log>": evidence,
						"<what it shows>": "the page", "<suite>": "go test", "<N>": "3", "<reason>": "no screen",
					})
					if code := h.run(h.home, args...); code != 0 {
						t.Fatalf("%q: exit %d: %s", line, code, h.errb)
					}
					h.out.Reset()
					if code := h.run(h.home, "proof", "index", "--task", tk.ID); code != 0 {
						t.Fatalf("proof index: exit %d: %s", code, h.errb)
					}
					want := "i-" + strconv.Itoa(item) + ": " + kind + " proof saved"
					if !strings.Contains(h.out.String(), want) {
						t.Fatalf("proof index lacks %q after %q:\n%s", want, line, h.out)
					}
				})
			}
		}
	}
	for _, name := range []string{"grid-builder.md", "grid-checker.md", "grid-operator.md", "grid-runner.md"} {
		if seen[name] == 0 {
			t.Errorf("%s has no `grid proof add` line", name)
		}
	}
}
