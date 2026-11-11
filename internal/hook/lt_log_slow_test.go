package hook

import (
	"bytes"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

var ltLogSlowLine = regexp.MustCompile(`^grid hook: hook (PreToolUse|PostToolUse) slow: (\d+) ms\n$`)

var ltLogSlowEvents = []string{"PreToolUse", "PostToolUse"}

// TestLtLogSlowFastLogsNothing proves the under-slowHook branch with a start
// back-dated by half of slowHook: no wait, no clock injection.
func TestLtLogSlowFastLogsNothing(t *testing.T) {
	for _, ev := range ltLogSlowEvents {
		var buf bytes.Buffer
		Options{Event: ev, Stderr: &buf}.logSlow(time.Now().Add(-slowHook / 2))
		if buf.Len() != 0 {
			t.Errorf("%s: fast call wrote %q, want nothing", ev, buf.String())
		}
	}
}

// TestLtLogSlowSlowWritesOneLine proves the slow branch with a 3 s old start.
func TestLtLogSlowSlowWritesOneLine(t *testing.T) {
	for _, ev := range ltLogSlowEvents {
		var buf bytes.Buffer
		Options{Event: ev, Stderr: &buf}.logSlow(time.Now().Add(-3 * time.Second))
		m := ltLogSlowLine.FindStringSubmatch(buf.String())
		if m == nil {
			t.Errorf("%s: output %q does not match the slow line", ev, buf.String())
			continue
		}
		if m[1] != ev {
			t.Errorf("event in line = %q, want %q", m[1], ev)
		}
		ms, err := strconv.Atoi(m[2])
		if err != nil || ms < 3000 {
			t.Errorf("%s: ms = %q, want >= 3000", ev, m[2])
		}
	}
}

// TestLtLogSlowNormalisesName proves the event is printed with the table
// spelling, and that a nil Stderr neither panics nor writes.
func TestLtLogSlowNormalisesName(t *testing.T) {
	var buf bytes.Buffer
	Options{Event: "pretooluse", Stderr: &buf}.logSlow(time.Now().Add(-3 * time.Second))
	if !strings.HasPrefix(buf.String(), "grid hook: hook PreToolUse slow: ") {
		t.Errorf("got %q, want the table spelling PreToolUse", buf.String())
	}
	Options{Event: "pretooluse", Stderr: nil}.logSlow(time.Now().Add(-3 * time.Second))
}
