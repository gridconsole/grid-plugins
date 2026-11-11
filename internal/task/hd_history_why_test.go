package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

// hdWStamp is the time stamp the first line of a history file holds: the writer
// reads the clock, so a test builds its expected line from what the file has.
func hdWStamp(t *testing.T, line string) string {
	t.Helper()
	stamp := strings.TrimPrefix(line, "- ")
	if len(stamp) < 16 {
		t.Fatalf("line has no timestamp: %q", line)
	}
	return stamp[:16]
}

func hdWLines(t *testing.T, dir string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestHdWLine(t *testing.T) {
	const at, event, detail = "2026-11-11T09:00", "brain read", "brain:project/deploy"
	for _, c := range []struct{ name, got, want string }{
		{"via and why",
			taskfile.HistoryLine(at, event, detail, "claude", "J-3", "search: deploy"),
			"- 2026-11-11T09:00 · brain read · brain:project/deploy · by claude · via J-3 · why search: deploy"},
		{"why without via",
			taskfile.HistoryLine(at, event, detail, "claude", "", "search: deploy"),
			"- 2026-11-11T09:00 · brain read · brain:project/deploy · by claude · why search: deploy"},
		{"via and an empty why is the line main writes",
			taskfile.HistoryLine(at, event, detail, "claude", "J-3", ""),
			taskfile.HistoryLine(at, event, detail, "claude", "J-3")},
		{"no why is the line main writes",
			taskfile.HistoryLine(at, event, detail, "claude"),
			"- 2026-11-11T09:00 · brain read · brain:project/deploy · by claude"},
		{"an empty via and an empty why are the line main writes",
			taskfile.HistoryLine(at, event, detail, "claude", "", ""),
			taskfile.HistoryLine(at, event, detail, "claude")},
	} {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
	// The line without a why has no trace of the field.
	if got := taskfile.HistoryLine(at, event, detail, "claude", "J-3"); strings.Contains(got, " · why") {
		t.Errorf("a line without a why carries the field: %q", got)
	}
}

func TestHdWAppend(t *testing.T) {
	_, tk := writeFixture(t)
	dir := tk.Path

	// A separator and a newline in the why never reach the line.
	if err := AppendHistoryWhy(dir, "brain read", "brain:project/deploy", "claude", "J-3", "a · b\n"); err != nil {
		t.Fatal(err)
	}
	lines := hdWLines(t, dir)
	if len(lines) != 1 {
		t.Fatalf("want one line, got %q", lines)
	}
	stamp := hdWStamp(t, lines[0])
	if want := taskfile.HistoryLine(stamp, "brain read", "brain:project/deploy", "claude", "J-3", "a - b"); lines[0] != want {
		t.Fatalf("line %q, want %q", lines[0], want)
	}
	if !strings.HasSuffix(lines[0], " · why a - b") {
		t.Fatalf("line %q does not end with the cleaned why", lines[0])
	}
	if n := strings.Count(lines[0], " · "); n != 5 {
		t.Fatalf("the why added a separator: %d in %q", n, lines[0])
	}

	// A newline inside the why is one space.
	if err := AppendHistoryWhy(dir, "brain read", "brain:project/other", "claude", "", "x ·\ny"); err != nil {
		t.Fatal(err)
	}
	lines = hdWLines(t, dir)
	if len(lines) != 2 {
		t.Fatalf("want two lines, got %q", lines)
	}
	// The minute may turn between two writes, so compare after the stamp.
	_, got, _ := strings.Cut(lines[1], " · ")
	_, want, _ := strings.Cut(taskfile.HistoryLine(stamp, "brain read", "brain:project/other", "claude", "", "x - y"), " · ")
	if got != want {
		t.Fatalf("line %q, want after the stamp %q", lines[1], want)
	}

	// 300 runes are cut to 160 by CleanHistory, whole runes.
	long := strings.Repeat("é", 300)
	if err := AppendHistoryWhy(dir, "brain read", "brain:project/long", "claude", "J-3", long); err != nil {
		t.Fatal(err)
	}
	lines = hdWLines(t, dir)
	last := lines[len(lines)-1]
	if !strings.HasSuffix(last, " · why "+strings.Repeat("é", 160)) || strings.HasSuffix(last, strings.Repeat("é", 161)) {
		t.Fatalf("the why was not cut to 160 runes: %q", last)
	}

	// The same call twice in a row adds nothing.
	before, _ := os.ReadFile(filepath.Join(dir, "history.md"))
	if err := AppendHistoryWhy(dir, "brain read", "brain:project/long", "claude", "J-3", long); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(filepath.Join(dir, "history.md")); string(after) != string(before) {
		t.Fatalf("a repeated line with the same why was appended:\n%s", after)
	}

	// The same line with another why is a new line.
	if err := AppendHistoryWhy(dir, "brain read", "brain:project/long", "claude", "J-3", "another reason"); err != nil {
		t.Fatal(err)
	}
	lines = hdWLines(t, dir)
	if len(lines) != 4 || !strings.HasSuffix(lines[3], " · brain read · brain:project/long · by claude · via J-3 · why another reason") {
		t.Fatalf("another why did not append: %q", lines)
	}

	// The same line again without a why is also a new line, and one more without a
	// why is the consecutive duplicate it always was.
	for i := 0; i < 2; i++ {
		if err := AppendHistoryWhy(dir, "brain read", "brain:project/long", "claude", "J-3", " \n "); err != nil {
			t.Fatal(err)
		}
		lines = hdWLines(t, dir)
		if len(lines) != 5 {
			t.Fatalf("write %d without a why: want 5 lines, got %q", i, lines)
		}
	}
	if strings.Contains(lines[4], " · why") {
		t.Fatalf("a blank why wrote the field: %q", lines[4])
	}
}

// AppendHistory is AppendHistoryWhy without a why: it writes what main wrote.
func TestHdWAppendHistoryWritesNoWhy(t *testing.T) {
	_, tk := writeFixture(t)
	if err := AppendHistory(tk.Path, "brain read", "brain:project/deploy", "claude", "J-3"); err != nil {
		t.Fatal(err)
	}
	lines := hdWLines(t, tk.Path)
	if len(lines) != 1 {
		t.Fatalf("want one line, got %q", lines)
	}
	if want := taskfile.HistoryLine(hdWStamp(t, lines[0]), "brain read", "brain:project/deploy", "claude", "J-3"); lines[0] != want {
		t.Fatalf("line %q, want %q", lines[0], want)
	}
	if strings.Contains(lines[0], " · why") {
		t.Fatalf("AppendHistory wrote a why: %q", lines[0])
	}
}
