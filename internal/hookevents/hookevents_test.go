package hookevents

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// writeLines appends n one-line records, one Append each, with rule "r<i>" and a
// time that rises one second a line. A line is about 85 bytes.
func writeLines(home string, n int, onEach func(i int)) {
	base := time.Date(2026, 11, 16, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		Append(home, []Record{{Rule: "r" + strconv.Itoa(i), Event: "PreToolUse", Outcome: OK,
			At: base.Add(time.Duration(i) * time.Second).Format(time.RFC3339)}})
		if onEach != nil {
			onEach(i)
		}
	}
}

// manyLines is enough lines to fill the live file and both older copies and to
// drop the oldest of them: about 5 x Limit bytes at 85 bytes a line.
const manyLines = 5 * Limit / 85

func TestLimitKeepsThreeHoursAtBusyRate(t *testing.T) {
	// About 100 bytes a line, 250 lines a minute, 3 hours.
	const line, perMinute, minutes = 100, 250, 180
	if got := int64(Files) * Limit / line; got < perMinute*minutes {
		t.Fatalf("Files x Limit holds about %d lines, need %d for 3 hours at %d a minute", got, perMinute*minutes, perMinute)
	}
}

func TestReadReturnsShiftedCopiesInTimeOrder(t *testing.T) {
	home := t.TempDir()
	n := manyLines
	writeLines(home, n, nil)
	for _, suffix := range []string{"", ".1", ".2"} {
		if _, err := os.Stat(Path(home) + suffix); err != nil {
			t.Fatalf("events.jsonl%s missing after %d lines: %v", suffix, n, err)
		}
	}
	recs, err := Read(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) < 2 {
		t.Fatalf("Read returned %d lines", len(recs))
	}
	// A busy account writes 250 lines a minute: the log must still hold 3 hours.
	if want := 250 * 180; len(recs) < want {
		t.Fatalf("Read returned %d lines, 3 hours at 250 a minute is %d", len(recs), want)
	}
	// Newest first, strictly: each line is one second older than the one before it.
	if recs[0].Rule != "r"+strconv.Itoa(n-1) {
		t.Fatalf("newest line is %q, want r%d", recs[0].Rule, n-1)
	}
	prev := -1
	fromFile := map[string]bool{}
	for _, suffix := range []string{"", ".1", ".2"} {
		b, err := os.ReadFile(Path(home) + suffix)
		if err != nil {
			t.Fatal(err)
		}
		for _, l := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
			fromFile[l] = true
		}
	}
	for i, r := range recs {
		idx, err := strconv.Atoi(strings.TrimPrefix(r.Rule, "r"))
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 && idx != prev-1 {
			t.Fatalf("line %d is r%d after r%d: not newest first and gap free", i, idx, prev)
		}
		prev = idx
	}
	if len(recs) != len(fromFile) {
		t.Fatalf("Read returned %d lines, the three files hold %d", len(recs), len(fromFile))
	}
	// The oldest line Read returns must come from events.jsonl.2, and the newest
	// from events.jsonl: it spans all three files.
	b2, err := os.ReadFile(Path(home) + ".2")
	if err != nil {
		t.Fatal(err)
	}
	oldest := recs[len(recs)-1].Rule
	if !strings.Contains(string(b2), fmt.Sprintf("%q", oldest)) {
		t.Fatalf("oldest line %s is not in events.jsonl.2", oldest)
	}
	b0, err := os.ReadFile(Path(home))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b0), fmt.Sprintf("%q", recs[0].Rule)) {
		t.Fatalf("newest line %s is not in events.jsonl", recs[0].Rule)
	}
	// Lines older than the third copy are gone.
	if prev == 0 {
		t.Fatalf("nothing was dropped: first line r0 still read after %d lines", n)
	}
}

func TestFilesStayWithinFilesTimesLimitPlusOneLine(t *testing.T) {
	home := t.TempDir()
	n := manyLines
	var maxFile int64
	var maxTotal int64
	dir := filepath.Dir(Path(home))
	writeLines(home, n, func(i int) {
		// Checked after every append, so the maximum is the true maximum.
		ents, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		var total int64
		names := 0
		for _, e := range ents {
			fi, err := e.Info()
			if err != nil {
				t.Fatal(err)
			}
			// events.jsonl.lock is the flock file logshift.Shift takes; it holds no
			// log lines and is not one of the Files copies. Its bytes still count.
			if e.Name() != filepath.Base(Path(home))+".lock" {
				names++
			}
			total += fi.Size()
			if fi.Size() > maxFile {
				maxFile = fi.Size()
			}
		}
		if names > Files {
			t.Fatalf("after line %d there are %d files, more than Files=%d", i, names, Files)
		}
		if total > maxTotal {
			maxTotal = total
		}
	})
	b, err := os.ReadFile(Path(home))
	if err != nil {
		t.Fatal(err)
	}
	// The line numbers rise, so the last line is the longest one written.
	ls := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	longest := int64(len(ls[len(ls)-1]) + 1)
	// A file is shifted at its next append once it holds Limit bytes or more, so it
	// stops less than one line past Limit: no file is above Limit plus one line, and
	// the log is at most Files files of that size.
	if maxFile >= Limit+longest {
		t.Fatalf("a file reached %d bytes, not under Limit %d plus one line of %d bytes", maxFile, int64(Limit), longest)
	}
	if bound := int64(Files) * (Limit + longest); maxTotal >= bound {
		t.Fatalf("the files reached %d bytes together, not under Files x (Limit + one line) = %d", maxTotal, bound)
	}
	if maxTotal < int64(Files-1)*Limit {
		t.Fatalf("the files only reached %d bytes: the test did not fill the log", maxTotal)
	}
}
