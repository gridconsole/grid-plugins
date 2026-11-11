package hook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/hookio"
)

// sbnLimit is the read limit readBounded names today (stop.go:33-34).
const sbnLimit = 1 << 20

func sbnWrite(t *testing.T, path string, b []byte) string {
	t.Helper()
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// sbnRow is one transcript JSONL line whose message content is the given value.
func sbnRow(t *testing.T, content any) string {
	t.Helper()
	b, err := json.Marshal(map[string]any{"message": map[string]any{"content": content}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func sbnTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	return sbnWrite(t, filepath.Join(t.TempDir(), "agent.jsonl"), []byte(strings.Join(lines, "\n")+"\n"))
}

func TestReadBoundedLimits(t *testing.T) {
	dir := t.TempDir()
	if _, err := readBounded(dir); err == nil || err.Error() != "hook input must be a regular file" {
		t.Fatalf("directory: %v", err)
	}
	if _, err := readBounded(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("a missing file read without error")
	}
	target := sbnWrite(t, filepath.Join(dir, "target"), []byte("x"))
	if err := os.Symlink(target, filepath.Join(dir, "link")); err == nil {
		if _, err := readBounded(filepath.Join(dir, "link")); err == nil || err.Error() != "hook input must be a regular file" {
			t.Fatalf("symlink: %v", err)
		}
	}
	at := sbnWrite(t, filepath.Join(dir, "at"), []byte(strings.Repeat("a", sbnLimit)))
	got, err := readBounded(at)
	if err != nil || len(got) != sbnLimit {
		t.Fatalf("file at the limit: len %d, %v", len(got), err)
	}
	over := sbnWrite(t, filepath.Join(dir, "over"), []byte(strings.Repeat("a", sbnLimit+1)))
	if _, err := readBounded(over); err == nil || err.Error() != "hook file exceeds read limit" {
		t.Fatalf("file one byte over: %v", err)
	}
}

func TestBriefNameFromTranscript(t *testing.T) {
	task := t.TempDir()
	other := t.TempDir()
	brief := filepath.Join(task, "subagents", "03-build.md")
	second := filepath.Join(task, "subagents", "04-test.md")
	elsewhere := filepath.Join(other, "subagents", "03-build.md")
	in := func(path string) hookio.Input { return hookio.Input{AgentTranscriptPath: path, Cwd: task} }

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"string content", []string{sbnRow(t, "Read "+brief+" and build it.")}, "03-build.md"},
		{"block text", []string{sbnRow(t, []any{map[string]any{"type": "text", "text": "Your brief is " + brief}})}, "03-build.md"},
		{"tool input file_path", []string{sbnRow(t, []any{map[string]any{"type": "tool_use", "input": map[string]any{"file_path": brief}}})}, "03-build.md"},
		{"relative to cwd", []string{sbnRow(t, "Read ./subagents/03-build.md first.")}, "03-build.md"},
		{"no mention", []string{sbnRow(t, "Nothing named here."), sbnRow(t, []any{map[string]any{"type": "text", "text": "still nothing"}})}, ""},
		{"two different briefs", []string{sbnRow(t, "Read "+brief), sbnRow(t, "then "+second)}, ""},
		{"the same brief twice", []string{sbnRow(t, "Read "+brief), sbnRow(t, []any{map[string]any{"type": "tool_use", "input": map[string]any{"file_path": brief}}})}, "03-build.md"},
		{"another task's subagents folder", []string{sbnRow(t, "Read "+elsewhere)}, ""},
		{"other task's brief does not add a second name", []string{sbnRow(t, "Read "+brief+" not "+filepath.Join(other, "subagents", "09-other.md"))}, "03-build.md"},
		{"a line that is not JSON is skipped", []string{"this is not json " + brief, "{broken", sbnRow(t, "Read "+brief)}, "03-build.md"},
		{"only a non-JSON line", []string{"not json " + brief}, ""},
		{"content that is neither string nor list", []string{`{"message":{"content":7}}`, `{"message":{"content":[7,"x"]}}`}, ""},
		{"not a .md brief", []string{sbnRow(t, "Read "+filepath.Join(task, "subagents", "03-build.txt"))}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := briefName(in(sbnTranscript(t, c.lines...)), task); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}

	t.Run("unreadable transcript gives no name", func(t *testing.T) {
		if got := briefName(in(filepath.Join(t.TempDir(), "missing.jsonl")), task); got != "" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("transcript over the read limit gives no name", func(t *testing.T) {
		big := sbnWrite(t, filepath.Join(t.TempDir(), "big.jsonl"), []byte(sbnRow(t, "Read "+brief)+"\n"+strings.Repeat("a", sbnLimit)))
		if got := briefName(in(big), task); got != "" {
			t.Fatalf("got %q", got)
		}
	})
	if runtime.GOOS != "windows" {
		// stop.go:76 compares filepath.Dir with filepath.Join; on a POSIX host a
		// backslash is not a separator, so a backslash mention names no brief here.
		t.Run("backslash mention on a POSIX host", func(t *testing.T) {
			path := sbnTranscript(t, sbnRow(t, `Read task\subagents\03-build.md`))
			if got := briefName(in(path), task); got != "" {
				t.Fatalf("got %q", got)
			}
		})
	}
}

// Compiled, needs Windows run:
//
//	GOOS=windows go test -c -o /dev/null ./internal/hook/   (compile only, any host)
//	go test -count=1 -run TestBriefNameWindowsDriveLetterCase ./internal/hook/   (on a Windows host)
//
// The transcript names the brief with an upper-case drive letter and the task
// folder has a lower-case one. stop.go:76 is a plain string compare, so the
// code today gives "" for the different case; the same case gives the name.
func TestBriefNameWindowsDriveLetterCase(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("compiled, needs Windows run")
	}
	upper := `C:\Grid\tasks\T-01`
	lower := `c:\Grid\tasks\T-01`
	mention := upper + `\subagents\03-build.md`
	path := sbnTranscript(t, sbnRow(t, "Read "+mention))
	in := hookio.Input{AgentTranscriptPath: path, Cwd: lower}
	if got := briefName(in, upper); got != "03-build.md" {
		t.Fatalf("same case: got %q", got)
	}
	if got := briefName(in, lower); got != "" {
		t.Fatalf("drive letter in another case: got %q, want \"\" (the compare at stop.go:76 is case-sensitive)", got)
	}
}

func sbnHistory(t *testing.T, f *eventFixture) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(f.dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSubagentStopHistoryNamesTheBrief(t *testing.T) {
	t.Run("brief in the transcript", func(t *testing.T) {
		f := eventSetup(t)
		brief := filepath.Join(f.dir, "subagents", "03-build.md")
		path := sbnTranscript(t, sbnRow(t, "Read "+brief+" and do it."))
		_, log, code := f.run(t, "SubagentStop", hookio.Input{AgentTranscriptPath: path})
		if code != 0 || log != "" {
			t.Fatalf("code %d, log %q", code, log)
		}
		h := sbnHistory(t, f)
		if n := strings.Count(h, "subagent finished"); n != 1 {
			t.Fatalf("want one line, got %d: %s", n, h)
		}
		if !strings.Contains(h, "subagent finished · brief 03-build · by claude") || strings.Contains(h, "brief 03-build.md") || strings.Contains(h, "brief 03 ") {
			t.Fatalf("history: %s", h)
		}
	})
	t.Run("no brief in the transcript", func(t *testing.T) {
		f := eventSetup(t)
		path := sbnTranscript(t, sbnRow(t, "Nothing named here."))
		_, log, code := f.run(t, "SubagentStop", hookio.Input{AgentTranscriptPath: path})
		if code != 0 || log != "" {
			t.Fatalf("code %d, log %q", code, log)
		}
		h := sbnHistory(t, f)
		if n := strings.Count(h, "subagent finished"); n != 1 || strings.Contains(h, "brief") {
			t.Fatalf("history: %s", h)
		}
		if !strings.Contains(h, "subagent finished · by claude") {
			t.Fatalf("history: %s", h)
		}
	})
	t.Run("two briefs name none", func(t *testing.T) {
		f := eventSetup(t)
		path := sbnTranscript(t, sbnRow(t, "Read "+filepath.Join(f.dir, "subagents", "03-build.md")), sbnRow(t, "and "+filepath.Join(f.dir, "subagents", "04-test.md")))
		if _, log, code := f.run(t, "SubagentStop", hookio.Input{AgentTranscriptPath: path}); code != 0 || log != "" {
			t.Fatalf("code %d, log %q", code, log)
		}
		if h := sbnHistory(t, f); strings.Contains(h, "brief") {
			t.Fatalf("history: %s", h)
		}
	})
}

func TestCheckStopLogsTaskDirAndFeedErrors(t *testing.T) {
	stopInput := func(f *eventFixture) hookio.Input {
		return hookio.Input{HookEventName: "Stop", Cwd: f.dir, SessionID: "session-demo"}
	}
	assertLogged := func(t *testing.T, f *eventFixture) {
		t.Helper()
		var log strings.Builder
		o := f.o
		o.Stderr = &log
		if code := checkStop(o, stopInput(f)); code != 0 {
			t.Fatalf("code %d, log %q", code, log.String())
		}
		got := log.String()
		if !strings.HasPrefix(got, "grid hook: stop check: ") || strings.Contains(got, "Post the result") || strings.Count(got, "\n") != 1 {
			t.Fatalf("log: %q", got)
		}
	}

	t.Run("taskDir error", func(t *testing.T) {
		f := eventSetup(t)
		f.env["GRID_JOB"] = "J-01"
		f.env["GRID_TASK"] = "no-such-task-anywhere"
		assertLogged(t, f)
	})
	t.Run("jobHasResult error", func(t *testing.T) {
		f := eventSetup(t)
		f.env["GRID_JOB"] = "J-01"
		// An unreadable reader cache makes the Feed read fail without chmod
		// (feed/cache.go: "invalid reader cache").
		if err := os.MkdirAll(filepath.Join(f.dir, "feed", ".cache"), 0o700); err != nil {
			t.Fatal(err)
		}
		sbnWrite(t, filepath.Join(f.dir, "feed", ".cache", "reader.json"), []byte("not json"))
		assertLogged(t, f)
	})
	t.Run("same errors through the hook entry exit 0", func(t *testing.T) {
		f := eventSetup(t)
		f.env["GRID_JOB"] = "J-01"
		f.env["GRID_TASK"] = "no-such-task-anywhere"
		out, log, code := f.run(t, "Stop", hookio.Input{})
		if code != 0 || out != "" || !strings.Contains(log, "grid hook: stop check: ") {
			t.Fatalf("code %d, out %q, log %q", code, out, log)
		}
	})
}
