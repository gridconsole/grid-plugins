package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withSettleAndPause sets settle to zero and replaces the pause for one test.
func withSettleAndPause(t *testing.T, p func(time.Duration)) {
	t.Helper()
	oldSettle, oldPause := settle, pause
	t.Cleanup(func() { settle, pause = oldSettle, oldPause })
	settle = 0
	pause = p
}

func TestSettleRescansBeforeReportingEntryThatIsGoneAfterPause(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grid-eval-other")
	calls := 0
	withSettleAndPause(t, func(time.Duration) {
		calls++
		if calls == 1 {
			_ = os.Remove(path)
		}
	})
	var stderr bytes.Buffer
	status := check(dir, func() int {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		return 0
	}, &stderr)
	if status != 0 || strings.Contains(stderr.String(), "new entry") {
		t.Fatalf("transient entry reported: status %d: %s", status, &stderr)
	}
	if calls < 1 {
		t.Fatal("entry was reported or ignored without a pause and rescan")
	}
}

func TestSettleReportsEntryThatStaysAndDoesNotRemoveIt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "grid-eval-stays")
	calls := 0
	withSettleAndPause(t, func(d time.Duration) {
		calls++
		if d != 50*time.Millisecond {
			t.Errorf("pause step %s, want 50ms", d)
		}
	})
	var stderr bytes.Buffer
	status := check(dir, func() int {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		return 0
	}, &stderr)
	if status != 1 || !strings.Contains(stderr.String(), "temp leak check: new entry grid-eval-stays") {
		t.Fatalf("leak not reported: status %d: %s", status, &stderr)
	}
	if calls < 1 {
		t.Fatal("pause was never called before the entry was reported")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("check must report rather than delete leaks: %v", err)
	}
}
