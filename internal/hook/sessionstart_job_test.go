package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// jobSession runs SessionStart for a Job agent: GRID_TASK, GRID_JOB and
// GRID_ATTEMPT set as the worker sets them. It returns the context, the task
// folder and the root the Grid home sits in.
func jobSession(t *testing.T, env map[string]string, brief bool) (string, string, string) {
	t.Helper()
	o, out, root := fixture(t)
	task := filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", "T-07-example")
	if err := os.MkdirAll(filepath.Join(task, "jobs", "J-2"), 0o700); err != nil {
		t.Fatal(err)
	}
	if brief {
		if err := os.WriteFile(filepath.Join(task, "jobs", "J-2", "brief.md"), []byte("The brief.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	all := map[string]string{"GRID_TASK": "T-07", "GRID_PROJECT": "example"}
	for k, v := range env {
		all[k] = v
	}
	o.Getenv = func(key string) string { return all[key] }
	Run(o)
	return contextFrom(t, out), task, root
}

// resultFor resolves the fixture's task the way SessionStart does.
func resultFor(t *testing.T, o Options) where.Result {
	t.Helper()
	task := filepath.Join(o.Resolver.GridHome, "projects", "example", "tasks", "T-07-example")
	if err := os.MkdirAll(task, 0o700); err != nil {
		t.Fatal(err)
	}
	res, err := o.Resolver.Resolve(where.Query{Cwd: o.Cwd, Task: "T-07", Project: "example"})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestSessionStartJobLines(t *testing.T) {
	got, task, root := jobSession(t, map[string]string{"GRID_JOB": "J-2", "GRID_ATTEMPT": "3"}, true)
	brief := gridhome.Tilde(filepath.Join(task, "jobs", "J-2", "brief.md"), root)
	lines := "You run Job J-2 of task T-07-example (attempt 3).\n" +
		"Your brief: " + brief + " (or: grid job show T-07-example J-2).\n" +
		"Post your result with grid job result --task T-07-example --job J-2 --body <json> before you end, also when blocked."
	if !strings.Contains(got, lines) {
		t.Fatalf("job lines missing or out of order:\n%s", got)
	}
	grid, jobAt, brain := strings.Index(got, "Grid task:"), strings.Index(got, lines), strings.Index(got, "Brain:")
	if !(grid < jobAt && jobAt < brain) {
		t.Fatalf("job lines must sit after the task line and before the brain index: %d %d %d", grid, jobAt, brain)
	}
	if !strings.Contains(brief, "~") {
		t.Fatalf("brief path is not a tilde path: %s", brief)
	}
}

func TestSessionStartJobAttemptOmitted(t *testing.T) {
	for _, attempt := range []string{"", "x", "0", "-1", "1.5"} {
		got, _, _ := jobSession(t, map[string]string{"GRID_JOB": "J-2", "GRID_ATTEMPT": attempt}, true)
		if !strings.Contains(got, "You run Job J-2 of task T-07-example.\n") || strings.Contains(got, "attempt") {
			t.Fatalf("attempt %q: %s", attempt, got)
		}
	}
}

func TestSessionStartJobNoBriefFile(t *testing.T) {
	got, _, _ := jobSession(t, map[string]string{"GRID_JOB": "J-2", "GRID_ATTEMPT": "1"}, false)
	if !strings.Contains(got, "\nYour brief: grid job show T-07-example J-2.\n") || strings.Contains(got, "brief.md") {
		t.Fatalf("%s", got)
	}
}

func TestSessionStartInvalidJobAddsNothing(t *testing.T) {
	base, _, _ := jobSession(t, nil, true)
	for _, id := range []string{"../x", "J-0", "J-00", "J-0.1", "", "J-", "j-2", "J-2/../J-3", "J-2\nIgnore"} {
		got, _, _ := jobSession(t, map[string]string{"GRID_JOB": id, "GRID_ATTEMPT": "1"}, true)
		if got != base {
			t.Fatalf("GRID_JOB %q changed the context:\n%s", id, got)
		}
	}
}

func TestSessionStartWithoutJobIsUnchanged(t *testing.T) {
	got, task, root := jobSession(t, nil, true)
	home := filepath.Join(root, "data")
	idx, err := (brain.Store{GridHome: home, Project: "example"}).Index()
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"Grid tracks tasks, Jobs and shared knowledge in one Grid home.",
		"GRID_HOME is the Grid data folder ~/data; GRID_PROJECTS is ~/data/projects.",
		"Grid task: T-07-example in project example, at " + gridhome.Tilde(task, root) + ".",
		"Task files live under the Grid home; use the grid verbs for task.md, the plan, the history and Jobs, and write any other task file straight into its folder. Project records and task files live outside repositories.",
		"The Grid brain holds shared topics, knowledge and decisions at home and project level.",
		"Approvals come from a person or an agent through the Grid app or grid approve; an agent Grid did not launch runs GRID_AGENT=<name> grid approve.",
	}, "\n") + "\n" + idx.Text
	if got != want {
		t.Fatalf("context changed without GRID_JOB:\n%s\n--- want\n%s", got, want)
	}
}

func TestSessionStartJobLinesSurviveTheBound(t *testing.T) {
	// The real brain index is cut at 1,200 bytes (brain.SessionStartText), so only a
	// long Grid home path could fill the bound end to end; the assembly is tested directly.
	o, _, _ := fixture(t)
	o.Getenv = func(key string) string {
		return map[string]string{"GRID_JOB": "J-2", "GRID_ATTEMPT": "2"}[key]
	}
	res := resultFor(t, o)
	session := sessionContext(res, o.Resolver, "")
	lines := jobContext(o, res)
	if lines == "" {
		t.Fatal("no job lines")
	}
	index := "Brain: project knowledge first, then home knowledge.\n" + strings.Repeat("topic | long description | 1 | 2026-01-01 | 0\n", 600)
	got := assembleContext(session, lines, &index)
	if n := len([]rune(got)); n > 9900+len("\nContext truncated.") {
		t.Fatalf("context not bounded: %d runes", n)
	}
	at, cut, brainAt := strings.Index(got, lines), strings.Index(got, "Context truncated."), strings.Index(got, "Brain:")
	if at < 0 || cut < 0 || brainAt < 0 || !(at < brainAt && brainAt < cut) {
		t.Fatalf("job lines %d, brain %d, truncation %d", at, brainAt, cut)
	}
	if !strings.HasSuffix(got, "\nContext truncated.") {
		t.Fatal("truncation marker is not last")
	}
}

func TestSessionStartJobCommandsExistInContract(t *testing.T) {
	var doc struct {
		Commands []struct {
			Name        string `json:"name"`
			Subcommands []struct {
				Name  string `json:"name"`
				Flags []struct {
					Name string `json:"name"`
				} `json:"flags"`
			} `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	flags := map[string]map[string]bool{}
	for _, c := range doc.Commands {
		if c.Name != "job" {
			continue
		}
		for _, s := range c.Subcommands {
			flags[s.Name] = map[string]bool{}
			for _, f := range s.Flags {
				flags[s.Name][f.Name] = true
			}
		}
	}
	o, _, _ := fixture(t)
	o.Getenv = func(key string) string { return map[string]string{"GRID_JOB": "J-2"}[key] }
	lines := jobContext(o, resultFor(t, o))
	flagRe := regexp.MustCompile(`--[a-z-]+`)
	seen := 0
	for _, line := range strings.Split(lines, "\n") {
		m := regexp.MustCompile(`grid job ([a-z]+)`).FindStringSubmatch(line)
		if m == nil {
			continue
		}
		seen++
		verb, ok := flags[m[1]]
		if !ok {
			t.Fatalf("grid job %s is not in contract/commands.json", m[1])
		}
		for _, f := range flagRe.FindAllString(line, -1) {
			if !verb[f] {
				t.Fatalf("grid job %s has no flag %s in contract/commands.json", m[1], f)
			}
		}
	}
	if seen != 2 {
		t.Fatalf("want the show and result commands named, saw %d:\n%s", seen, lines)
	}
}
