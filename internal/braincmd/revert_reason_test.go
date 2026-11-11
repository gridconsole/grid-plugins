package braincmd

import (
	"encoding/json"
	"strings"
	"testing"
)

// The reason given to revert is stored in the audit log and shown by changes.
func TestRevertReasonIsLoggedAndListed(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	h.out.Reset()
	h.err.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--reason", "v2 claim was wrong", "--level", "home"); code != 0 {
		t.Fatalf("revert --reason: exit %d, stderr %q", code, &h.err)
	}
	if got := h.out.String(); got != "v4 (reason: v2 claim was wrong)\n" {
		t.Fatalf("text output: %q", got)
	}
	h.out.Reset()
	h.err.Reset()
	first := h.changes(t).Changes[0]
	if first.Status != "reverted" || first.Version != 4 || first.Reason != "revert to v1: v2 claim was wrong" {
		t.Fatalf("newest change: %+v", first)
	}
	h.out.Reset()
	if code := h.run("changes", "--level", "home"); code != 0 {
		t.Fatalf("plain changes: %d", code)
	}
	if line := strings.SplitN(h.out.String(), "\n", 2)[0]; !strings.HasSuffix(line, " revert to v1: v2 claim was wrong") {
		t.Fatalf("plain line: %q", line)
	}
}

func TestRevertReasonJSONNamesIt(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	h.out.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--reason", "stale", "--json", "--level", "home"); code != 0 {
		t.Fatalf("revert: %d %s", code, &h.err)
	}
	var got map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, &h.out)
	}
	if got["reason"] != "stale" || got["version"] != float64(4) || got["noop"] != false {
		t.Fatalf("json: %v", got)
	}
	// Without --reason the key is absent.
	h.out.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "2", "--json", "--level", "home"); code != 0 {
		t.Fatalf("revert: %d %s", code, &h.err)
	}
	got = nil
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, has := got["reason"]; has {
		t.Fatalf("reason present without --reason: %v", got)
	}
}

func TestRevertWithoutReasonKeepsTheFixedText(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	h.out.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home"); code != 0 {
		t.Fatalf("revert: %d %s", code, &h.err)
	}
	if got := h.out.String(); got != "v4\n" {
		t.Fatalf("text output: %q", got)
	}
	h.out.Reset()
	if r := h.changes(t).Changes[0].Reason; r != "revert to v1" {
		t.Fatalf("reason %q", r)
	}
}

// A reason with a pipe, a newline or a leading dash is one cleaned audit line.
func TestRevertReasonWithAwkwardCharacters(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	h.out.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--reason", "-bad | claim\nline two", "--level", "home"); code != 0 {
		t.Fatalf("revert: %d %s", code, &h.err)
	}
	h.out.Reset()
	got := h.changes(t).Changes
	if len(got) != 4 || got[0].Reason != "revert to v1: -bad / claim line two" || got[0].Status != "reverted" || got[0].Version != 4 {
		t.Fatalf("changes: %+v", got)
	}
}

// The no-op path logs nothing, whatever the reason.
func TestRevertReasonOnNoopWritesNoLine(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	h.out.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "3", "--reason", "nothing to do", "--level", "home"); code != 0 {
		t.Fatalf("revert: %d %s", code, &h.err)
	}
	if got := h.out.String(); !strings.HasPrefix(got, "already at v3") {
		t.Fatalf("output %q", got)
	}
	h.out.Reset()
	if n := len(h.changes(t).Changes); n != 3 {
		t.Fatalf("%d change lines, want 3", n)
	}
}
