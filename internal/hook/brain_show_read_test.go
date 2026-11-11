package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/settings"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// showFixture is a task of project "demo" with an accepted project note
// `deploy` and an accepted home note `x`.
func showFixture(t *testing.T) (*eventFixture, brain.Entry) {
	t.Helper()
	f := eventSetup(t)
	if err := project.Write(f.home, project.Record{Name: "demo"}); err != nil {
		t.Fatal(err)
	}
	made, err := (task.Store{Resolver: f.o.Resolver}).New(task.NewOptions{Cwd: filepath.Dir(f.home), Words: "Deploy notes", Project: "demo"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(made.Path, "history.md"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	f.dir, f.o.Cwd = made.Path, made.Path
	f.env["GRID_TASK"] = made.Path
	s := brain.Store{GridHome: f.home, Project: "demo"}
	var deploy brain.Entry
	for _, level := range []string{"project", "home"} {
		name := map[string]string{"project": "deploy", "home": "x"}[level]
		n := brain.Note{Name: name, Topic: "decisions", Type: "convention", Description: "Use when deploying.", Summary: "How to deploy.", Source: "task:T-01", Level: level, Body: "body", Origin: brain.Origin{Kind: "session", ID: "session-demo"}}
		if _, err := s.Propose(n, n.Origin); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Accept(level, name); err != nil {
			t.Fatal(err)
		}
		e, err := s.Show("brain:" + level + "/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if level == "project" {
			deploy = e
		}
	}
	return f, deploy
}

func bash(cmd string) hookio.Input {
	return hookio.Input{ToolName: "Bash", ToolInput: map[string]any{"command": cmd}}
}

func historyOf(t *testing.T, f *eventFixture) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBrainShowLogsOneReadLine(t *testing.T) {
	f, deploy := showFixture(t)
	if deploy.ID != "brain:project/deploy" {
		t.Fatalf("fixture id: %s", deploy.ID)
	}
	out, log, code := f.run(t, "PostToolUse", bash("grid brain show deploy"))
	if out != "" || log != "" || code != 0 {
		t.Fatalf("show: %q %q %d", out, log, code)
	}
	h := historyOf(t, f)
	if strings.Count(h, "· brain read · brain:project/deploy · by claude") != 1 || strings.Contains(h, "via") {
		t.Fatalf("history: %s", h)
	}
	// A line for another note sits between, so history's own suppression of a
	// repeated last line cannot hide a missing session-activity key.
	f.run(t, "PostToolUse", bash("grid brain show brain:home/x"))
	for _, cmd := range []string{"grid brain show deploy", "grid brain show brain:project/deploy --json"} {
		f.run(t, "PostToolUse", bash(cmd))
	}
	f.run(t, "PostToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, deploy.Path)}})
	h = historyOf(t, f)
	if n := strings.Count(h, "brain read · brain:project/deploy"); n != 1 || strings.Count(h, "brain read") != 2 {
		t.Fatalf("repeat show or Read added lines: %s", h)
	}
}

func TestBrainReadThenShowAddsNothing(t *testing.T) {
	f, deploy := showFixture(t)
	f.run(t, "PostToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, deploy.Path)}})
	if n := strings.Count(historyOf(t, f), "· brain read · brain:project/deploy"); n != 1 {
		t.Fatalf("Read did not log (%d): %s", n, historyOf(t, f))
	}
	f.run(t, "PostToolUse", bash("grid brain show brain:home/x"))
	f.run(t, "PostToolUse", bash("grid brain show deploy"))
	if h := historyOf(t, f); strings.Count(h, "brain read · brain:project/deploy") != 1 || strings.Count(h, "brain read") != 2 {
		t.Fatalf("show after Read added a line: %s", h)
	}
}

func TestBrainShowResolvesAsTheCLIDoes(t *testing.T) {
	f, _ := showFixture(t)
	f.run(t, "PostToolUse", bash("grid brain show brain:home/x"))
	if h := historyOf(t, f); !strings.Contains(h, "· brain read · brain:home/x · by claude") {
		t.Fatalf("home id: %s", h)
	}
	f.run(t, "PostToolUse", bash("grid brain show --level home deploy"))
	if h := historyOf(t, f); strings.Contains(h, "brain:home/deploy") {
		t.Fatalf("unknown home note logged: %s", h)
	}
}

func TestBrainShowRecognisedForms(t *testing.T) {
	for _, cmd := range []string{
		"grid brain show deploy",
		"/usr/local/bin/grid brain show deploy",
		"grid brain show --json deploy",
		"grid brain show deploy --json",
		"grid brain show deploy --task T-1",
		"grid brain show deploy && echo done",
		"grid brain show deploy | jq .",
		"grid brain show deploy 2>&1 | head",
		"grid brain show 'deploy'",
		"grid brain show deploy > /tmp/note.txt",
		"grid brain show deploy >/dev/null 2>&1",
	} {
		f, _ := showFixture(t)
		out, log, code := f.run(t, "PostToolUse", bash(cmd))
		if out != "" || log != "" || code != 0 {
			t.Fatalf("%s: %q %q %d", cmd, out, log, code)
		}
		if h := historyOf(t, f); strings.Count(h, "brain read · brain:project/deploy") != 1 {
			t.Errorf("%q not recorded: %s", cmd, h)
		}
	}
}

func TestBrainShowIgnoredForms(t *testing.T) {
	for _, cmd := range []string{
		"grid brain search deploy",
		"echo grid brain show deploy",
		"ls && grid brain show deploy",
		"grid brain show",
		"grid brain show --json",
		"grid brain show nothing-here",
		"grid brain show deploy --proposal",
		"grid brain show --proposal deploy",
		"grid brain show deploy other",
		"grid brain show --bogus deploy",
		"grid brain show 'unterminated",
		"mygrid brain show deploy",
		"",
	} {
		f, _ := showFixture(t)
		out, log, code := f.run(t, "PostToolUse", bash(cmd))
		if out != "" || log != "" || code != 0 {
			t.Fatalf("%s: %q %q %d", cmd, out, log, code)
		}
		if h := historyOf(t, f); h != "" {
			t.Errorf("%q wrote a line: %s", cmd, h)
		}
	}
}

func TestBrainReadNamesTheJob(t *testing.T) {
	f, deploy := showFixture(t)
	f.env["GRID_JOB"] = "J-3"
	// Different notes: a repeat of the last history line is suppressed.
	f.run(t, "PostToolUse", bash("grid brain show brain:home/x"))
	f.run(t, "PostToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, deploy.Path)}})
	h := historyOf(t, f)
	for _, id := range []string{"brain:home/x", "brain:project/deploy"} {
		if !strings.Contains(h, "· brain read · "+id+" · by claude · via J-3") {
			t.Fatalf("no via for %s: %s", id, h)
		}
	}
}

func TestBrainShowFromWorktreeAndHookOff(t *testing.T) {
	f, _ := showFixture(t)
	work := t.TempDir()
	f.run(t, "PostToolUse", func() hookio.Input { in := bash("grid brain show deploy"); in.Cwd = work; return in }())
	if strings.Count(historyOf(t, f), "brain read · brain:project/deploy") != 1 {
		t.Fatalf("worktree cwd did not log to GRID_TASK: %s", historyOf(t, f))
	}
	g, _ := showFixture(t)
	if err := (settings.Store{Home: g.home}).SetHook("history-log", "off"); err != nil {
		t.Fatal(err)
	}
	g.run(t, "PostToolUse", bash("grid brain show deploy"))
	if h := historyOf(t, g); h != "" {
		t.Fatalf("history-log off still wrote: %s", h)
	}
}
