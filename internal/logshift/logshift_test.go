package logshift

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

const (
	// testLimit is a small limit so a test crosses it in a few lines; testLine is
	// the size of every line the tests write, so testLimit/testLine lines fill a file.
	testLimit = 2 << 10
	testLine  = 64
)

// line is a line of exactly testLine bytes that starts with tag, then dots.
func line(tag string) string {
	return tag + strings.Repeat(".", testLine-len(tag)-1) + "\n"
}

// appendLine writes one line the way hookevents.Append does: a fresh open with
// O_APPEND, one write, a close.
func appendLine(t *testing.T, path, tag string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Errorf("open %s: %v", path, err)
		return
	}
	if _, err := f.WriteString(line(tag)); err != nil {
		t.Errorf("write %s: %v", path, err)
	}
	_ = f.Close()
}

// fill writes testLimit bytes of lines tagged <tag>-000, <tag>-001, ... to name and
// returns the tags.
func fill(t *testing.T, name, tag string) []string {
	t.Helper()
	var tags []string
	var b strings.Builder
	for i := 0; b.Len() < testLimit; i++ {
		tags = append(tags, fmt.Sprintf("%s-%03d", tag, i))
		b.WriteString(line(tags[i]))
	}
	if err := os.WriteFile(name, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return tags
}

// wipe removes path and every shifted copy of it; the lock file stays.
func wipe(t *testing.T, path string, files int) {
	t.Helper()
	for _, name := range Names(path, files) {
		if err := os.Remove(name); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
}

// checkCopies reads path and its shifted copies and reports a shifted copy smaller
// than testLimit, a line in want that is gone or twice there, a line nobody wrote,
// and (when needCopy) a log with no shifted copy at all.
func checkCopies(t *testing.T, path string, files int, want []string, needCopy bool) {
	t.Helper()
	got := map[string]int{}
	copies := 0
	for i, name := range Names(path, files) {
		b, err := os.ReadFile(name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if i > 0 {
			copies++
			if len(b) < testLimit {
				t.Errorf("%s holds %d bytes: a shifted copy holds at least the limit, %d", filepath.Base(name), len(b), testLimit)
			}
		}
		for _, l := range strings.Split(string(b), "\n") {
			if l != "" {
				got[strings.SplitN(l, ".", 2)[0]]++
			}
		}
	}
	if needCopy && copies == 0 {
		t.Errorf("no shifted copy exists")
	}
	var lost, twice []string
	for _, tag := range want {
		switch got[tag] {
		case 1:
		case 0:
			lost = append(lost, tag)
		default:
			twice = append(twice, tag)
		}
		delete(got, tag)
	}
	if len(lost) > 0 {
		t.Errorf("%d lines are gone, first %v", len(lost), lost[:min(len(lost), 4)])
	}
	if len(twice) > 0 {
		t.Errorf("%d lines are there twice, first %v", len(twice), twice[:min(len(twice), 4)])
	}
	if len(got) > 0 {
		t.Errorf("%d lines nobody wrote", len(got))
	}
}

// Eight writers find the log full at the same moment, as eight grid hook processes
// do. Each one shifts and appends. The log is shifted once: the copy that was full
// is path.1 and the copy before it is path.2, as one writer alone leaves them.
func TestShiftEightWritersAtTheLimitLoseNoCopy(t *testing.T) {
	const writers, files, rounds = 8, 3, 60
	path := filepath.Join(t.TempDir(), "events.jsonl")
	for round := 0; round < rounds; round++ {
		wipe(t, path, files)
		want := append(fill(t, path+".1", "old"), fill(t, path, "cur")...)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for w := 0; w < writers; w++ {
			tag := fmt.Sprintf("w%d", w)
			want = append(want, tag)
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				Shift(path, testLimit, files)
				appendLine(t, path, tag)
			}()
		}
		close(start)
		wg.Wait()
		checkCopies(t, path, files, want, true)
		if t.Failed() {
			t.Fatalf("round %d of %d", round+1, rounds)
		}
	}
}

// Eight writers keep appending past the limit, ten limits in all. Files is so large
// that no copy is dropped on purpose, so every line must be in one copy and every
// shifted copy must be as large as the limit.
func TestShiftEightWritersAppendingPastTheLimitLoseNoLine(t *testing.T) {
	const writers, perWriter, files = 8, 40, 16
	path := filepath.Join(t.TempDir(), "events.jsonl")
	var want []string
	start := make(chan struct{})
	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		var tags []string
		for n := 0; n < perWriter; n++ {
			tags = append(tags, fmt.Sprintf("w%d-%03d", w, n))
		}
		want = append(want, tags...)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for _, tag := range tags {
				Shift(path, testLimit, files)
				appendLine(t, path, tag)
			}
		}()
	}
	close(start)
	wg.Wait()
	checkCopies(t, path, files, want, true)
}

// A process that saw the log full, then took the lock after another process had
// shifted it, finds a small log and moves nothing: path.1 stays the copy that was
// full and path.2 the one before it. shiftLocked is what Shift runs once it holds
// the lock; the other tests cannot hit this window on purpose.
func TestShiftLockedReadsTheSizeAgain(t *testing.T) {
	const files = 3
	path := filepath.Join(t.TempDir(), "events.jsonl")
	want := fill(t, path+".2", "old")
	want = append(want, fill(t, path+".1", "cur")...)
	if err := os.WriteFile(path, []byte(line("new")), 0o600); err != nil {
		t.Fatal(err)
	}
	want = append(want, "new")
	shiftLocked(path, testLimit, files)
	checkCopies(t, path, files, want, true)
	for name, first := range map[string]string{path + ".1": "cur-000", path + ".2": "old-000"} {
		b, err := os.ReadFile(name)
		if err != nil || !strings.HasPrefix(string(b), first) {
			t.Errorf("%s no longer starts with %s: %v", filepath.Base(name), first, err)
		}
	}
}

// A writer that cannot take the lock because another process holds it does not
// wait and does not shift; the caller appends as before. Once the lock is free the
// next call shifts.
func TestShiftSkipsWhileAnotherProcessHoldsTheLock(t *testing.T) {
	const files = 3
	path := filepath.Join(t.TempDir(), "events.jsonl")
	want := fill(t, path, "cur")
	holder, err := fslock.OpenNoFollow(path+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := fslock.LockExclusive(holder); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		Shift(path, testLimit, files)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Shift waited for a lock that another process holds")
	}
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Fatal("Shift moved the log while another process held the lock")
	}
	checkCopies(t, path, files, want, false)
	fslock.Unlock(holder)
	Shift(path, testLimit, files)
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("Shift did not shift once the lock was free: %v", err)
	}
	checkCopies(t, path, files, want, true)
}

// A lock file that cannot be opened is a lock that cannot be taken: Shift returns
// without a panic and without moving anything.
func TestShiftSkipsWhenTheLockFileCannotBeOpened(t *testing.T) {
	const files = 3
	path := filepath.Join(t.TempDir(), "events.jsonl")
	want := fill(t, path, "cur")
	if err := os.Mkdir(path+".lock", 0o700); err != nil {
		t.Fatal(err)
	}
	Shift(path, testLimit, files)
	if _, err := os.Stat(path + ".1"); err == nil {
		t.Fatal("Shift moved the log without holding the lock")
	}
	checkCopies(t, path, files, want, false)
}
