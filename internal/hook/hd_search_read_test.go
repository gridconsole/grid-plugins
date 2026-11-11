package hook

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
	"github.com/gridconsole/grid-plugins/internal/settings"
)

const (
	hdSDeploy = "· brain read · brain:project/deploy · by claude · via session session-demo"
	hdSHomeX  = "· brain read · brain:home/x · by claude · via session session-demo"
)

// hdSReadLines returns the `brain read` lines of the fixture task's history.
func hdSReadLines(t *testing.T, f *eventFixture) []string {
	t.Helper()
	var lines []string
	for _, l := range strings.Split(historyOf(t, f), "\n") {
		if strings.Contains(l, "· brain read ·") {
			lines = append(lines, l)
		}
	}
	return lines
}

// hdSQuiet runs one Bash command and asserts the hook printed and logged nothing.
func hdSQuiet(t *testing.T, f *eventFixture, cmd string) {
	t.Helper()
	if out, log, code := f.run(t, "PostToolUse", bash(cmd)); out != "" || log != "" || code != 0 {
		t.Fatalf("%s: %q %q %d", cmd, out, log, code)
	}
}

// Both fixture notes match "deploy" with the same score, project level first.
func TestHdSSearchRecordsHits(t *testing.T) {
	f, _ := showFixture(t)
	hdSQuiet(t, f, "grid brain search deploy")
	lines := hdSReadLines(t, f)
	n := 0
	for _, l := range lines {
		if strings.Contains(l, hdSDeploy+" · why search: deploy") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("want exactly one deploy line with the query as the why, got %d: %s", n, historyOf(t, f))
	}
	if len(lines) != 2 || !strings.Contains(lines[1], hdSHomeX+" · why search: deploy") {
		t.Fatalf("the home hit is one more line: %s", historyOf(t, f))
	}
}

func TestHdSSearchNamesTheJob(t *testing.T) {
	f, _ := showFixture(t)
	f.env["GRID_JOB"] = "J-3"
	hdSQuiet(t, f, "grid brain search deploy")
	if h := historyOf(t, f); !strings.Contains(h, "· brain read · brain:project/deploy · by claude · via J-3 · why search: deploy") {
		t.Fatalf("no via and why: %s", h)
	}
}

func TestHdSSearchOneLinePerNoteAndSession(t *testing.T) {
	f, deploy := showFixture(t)
	hdSQuiet(t, f, "grid brain search deploy")
	if n := len(hdSReadLines(t, f)); n != 2 {
		t.Fatalf("first search: %d lines: %s", n, historyOf(t, f))
	}
	// The last line is the home note's, so history's own suppression of a repeated
	// last line cannot hide a missing session-activity key for the deploy note.
	hdSQuiet(t, f, "grid brain search deploy")
	hdSQuiet(t, f, "grid brain show deploy")
	f.run(t, "PostToolUse", hookio.Input{ToolName: "Read", ToolInput: map[string]any{"file_path": filepath.Join(f.home, deploy.Path)}})
	if n := len(hdSReadLines(t, f)); n != 2 {
		t.Fatalf("repeat search, show or Read added lines: %s", historyOf(t, f))
	}
}

func TestHdSShowThenSearchAddsOnlyTheOtherNote(t *testing.T) {
	f, _ := showFixture(t)
	hdSQuiet(t, f, "grid brain show deploy")
	hdSQuiet(t, f, "grid brain search deploy")
	lines := hdSReadLines(t, f)
	if len(lines) != 2 || strings.Contains(lines[0], "why") || !strings.Contains(lines[1], hdSHomeX+" · why search: deploy") {
		t.Fatalf("search after show: %s", historyOf(t, f))
	}
}

func TestHdSSearchWithoutHitsWritesNothing(t *testing.T) {
	for _, cmd := range []string{
		"grid brain search deploy --topic nothing-here",
		"grid brain search zzzqqq",
		"grid brain search deploy --topic conventions",
	} {
		f, _ := showFixture(t)
		hdSQuiet(t, f, cmd)
		if h := historyOf(t, f); h != "" {
			t.Errorf("%q wrote a line: %s", cmd, h)
		}
	}
}

func TestHdSSearchRecordsTheNotesTheCLIPrinted(t *testing.T) {
	for _, c := range []struct {
		cmd  string
		want []string
	}{
		{"grid brain search deploy --limit 1", []string{hdSDeploy}},
		{"grid brain search --limit=1 deploy", []string{hdSDeploy}},
		{"grid brain search deploy --topic decisions --json", []string{hdSDeploy, hdSHomeX}},
		{"grid brain search --level home deploy", []string{hdSHomeX}},
		{"grid brain search --level project deploy", []string{hdSDeploy}},
		{"grid brain search --project demo deploy", []string{hdSDeploy, hdSHomeX}},
		{"grid brain search deploy && echo done", []string{hdSDeploy, hdSHomeX}},
		{"/usr/local/bin/grid brain search 'deploy' | head", []string{hdSDeploy, hdSHomeX}},
	} {
		f, _ := showFixture(t)
		hdSQuiet(t, f, c.cmd)
		lines := hdSReadLines(t, f)
		if len(lines) != len(c.want) {
			t.Errorf("%q: want %d lines: %s", c.cmd, len(c.want), historyOf(t, f))
			continue
		}
		for i, w := range c.want {
			if !strings.Contains(lines[i], w+" · why search: deploy") {
				t.Errorf("%q: line %d lacks %q: %s", c.cmd, i, w, lines[i])
			}
		}
	}
}

// --task and --project are read as the CLI reads them: a search of another task's
// project is not a search of the notes the hook reads.
func TestHdSSearchTaskFlag(t *testing.T) {
	f, _ := showFixture(t)
	hdSQuiet(t, f, "grid brain search deploy --task "+f.dir)
	if n := len(hdSReadLines(t, f)); n != 2 {
		t.Fatalf("--task naming this task: %s", historyOf(t, f))
	}
	g, _ := showFixture(t)
	hdSQuiet(t, g, "grid brain search deploy --task no-such-task")
	if h := historyOf(t, g); h != "" {
		t.Fatalf("--task naming no task wrote: %s", h)
	}
}

func TestHdSSearchWhyIsTheWords(t *testing.T) {
	f, _ := showFixture(t)
	hdSQuiet(t, f, `grid brain search "how to deploy" --limit 1`)
	if h := historyOf(t, f); !strings.Contains(h, hdSDeploy+" · why search: how to deploy\n") {
		t.Fatalf("why: %s", h)
	}
}

func TestHdSSearchIgnoredForms(t *testing.T) {
	for _, cmd := range []string{
		"grid brain search",
		"grid brain search ''",
		"grid brain search --json",
		"grid brain search deploy --bogus",
		"grid brain search deploy --limit 0",
		"grid brain search deploy --limit 101",
		"grid brain search deploy --limit many",
		"grid brain search deploy --limit",
		"grid brain search deploy --level other",
		"grid brain search deploy --level home --project demo",
		"grid brain search deploy --level home --task T-1",
		"grid brain search deploy --project another",
		"grid brain search deploy --topic 'not a name'",
		"grid brain search 'unterminated",
		"echo grid brain search deploy",
		"ls && grid brain search deploy",
		"grid brain searchx deploy",
		"grid brain show --proposal deploy",
		"mygrid brain search deploy",
		"",
	} {
		f, _ := showFixture(t)
		hdSQuiet(t, f, cmd)
		if h := historyOf(t, f); h != "" {
			t.Errorf("%q wrote a line: %s", cmd, h)
		}
	}
}

func TestHdSSearchNoTaskOrHookOffWritesNothing(t *testing.T) {
	f, _ := showFixture(t)
	delete(f.env, "GRID_TASK")
	in := bash("grid brain search deploy")
	in.Cwd = t.TempDir()
	if out, log, code := f.run(t, "PostToolUse", in); out != "" || log != "" || code != 0 {
		t.Fatalf("no task: %q %q %d", out, log, code)
	}
	if h := historyOf(t, f); h != "" {
		t.Fatalf("no task resolved, yet a line was written: %s", h)
	}

	g, _ := showFixture(t)
	if err := (settings.Store{Home: g.home}).SetHook("history-log", "off"); err != nil {
		t.Fatal(err)
	}
	hdSQuiet(t, g, "grid brain search deploy")
	if h := historyOf(t, g); h != "" {
		t.Fatalf("history-log off still wrote: %s", h)
	}
}
