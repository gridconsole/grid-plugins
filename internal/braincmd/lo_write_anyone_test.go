package braincmd

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// Owner answer 3 of 11 Nov (owner-answers/2026-11-11T2107Z.md): anyone the verb
// lets in may store a note in the brain, a `type: workflow` note too, and the
// author is written. Guests and workers stay refused by the verb's role rule.

const loWaHome = "--level=home"

// loWaAuthor is the audit line of a write of decisions/flow by member bo, in the
// form of brain's changes.go: `- <at> | <status> | <name> | <kind>:<id> | ...`.
var loWaAuthor = regexp.MustCompile(`\| (created|edited) \| flow \| person:bo \|`)

// loWaWrite runs one write of decisions/<name> that must succeed: exit 0, stdout
// want, no stderr, the bytes on disk and exactly one new log.md line, which it
// returns.
func loWaWrite(t *testing.T, f *keWrFix, want, role, member, name string, data []byte, expect string) string {
	t.Helper()
	before := len(keWrLog(t, f.env.GridHome))
	code := f.run(role, member, "", "decisions/"+name, "--expect", expect, "--file", f.path("lowa-"+brain.NoteRev(data), data), loWaHome)
	if code != 0 || f.out.String() != want || f.err.Len() != 0 {
		t.Fatalf("%s as %q (member %q): exit %d, stdout %q (want %q), stderr %q", name, role, member, code, f.out.String(), want, f.err.String())
	}
	disk, err := os.ReadFile(filepath.Join(f.env.GridHome, "knowledge", "decisions", name+".md"))
	if err != nil || !bytes.Equal(disk, data) {
		t.Fatalf("%s on disk: %q, %v", name, disk, err)
	}
	lines := keWrLog(t, f.env.GridHome)
	if len(lines) != before+1 {
		t.Fatalf("log.md has %d lines, want %d", len(lines), before+1)
	}
	return lines[len(lines)-1]
}

// A member writes a new workflow note, replaces it with plain text and replaces
// the plain note with workflow text; each audit line names person:bo.
func TestLoWaMemberWorkflow(t *testing.T) {
	f := keWrNew(t)
	flow := keWrBytes("decisions", "flow", "# Flow\n", keWrWorkflow)
	plain := keWrBytes("decisions", "flow", "# Plain\n")
	flow2 := keWrBytes("decisions", "flow", "# Flow two\n", keWrWorkflow)
	steps := []struct {
		name, want, expect string
		data               []byte
	}{
		{"new workflow note", "created decisions/flow v1 rev " + brain.NoteRev(flow) + "\n", "none", flow},
		{"workflow note to plain text", "edited decisions/flow v2 rev " + brain.NoteRev(plain) + "\n", brain.NoteRev(flow), plain},
		{"plain note to workflow text", "edited decisions/flow v3 rev " + brain.NoteRev(flow2) + "\n", brain.NoteRev(plain), flow2},
	}
	for _, s := range steps {
		if line := loWaWrite(t, f, s.want, "member", "bo", "flow", s.data, s.expect); !loWaAuthor.MatchString(line) {
			t.Fatalf("%s: log line %q", s.name, line)
		}
	}
}

// Guests and workers stay refused with the verb's role line, for a new workflow
// note and for a replace of the workflow note a member wrote; nothing changes.
func TestLoWaGuestWorker(t *testing.T) {
	f := keWrNew(t)
	flow := keWrBytes("decisions", "flow", "# Flow\n", keWrWorkflow)
	loWaWrite(t, f, "created decisions/flow v1 rev "+brain.NoteRev(flow)+"\n", "member", "bo", "flow", flow, "none")
	newFlow := f.path("lowa-new", keWrBytes("decisions", "n", "# N\n", keWrWorkflow))
	toFlow := f.path("lowa-to-flow", keWrBytes("decisions", "flow", "# Flow two\n", keWrWorkflow))
	toPlain := f.path("lowa-to-plain", keWrBytes("decisions", "flow", "# Plain\n"))
	cases := []struct {
		name string
		args []string
	}{
		{"new workflow note", []string{"decisions/n", "--expect", "none", "--file", newFlow, loWaHome}},
		{"replace the workflow note with workflow text", []string{"decisions/flow", "--expect", brain.NoteRev(flow), "--file", toFlow, loWaHome}},
		{"replace the workflow note with plain text", []string{"decisions/flow", "--expect", brain.NoteRev(flow), "--file", toPlain, loWaHome}},
	}
	for _, role := range []string{"guest", "worker"} {
		for _, c := range cases {
			t.Run(role+" "+c.name, func(t *testing.T) {
				before, lines := keWrSnapshot(t, f.env.GridHome), len(keWrLog(t, f.env.GridHome))
				if code := f.run(role, "bo", "", c.args...); code != 3 || f.err.String() != keWrRoleLine || f.out.Len() != 0 {
					t.Errorf("exit %d (want 3), stderr %q (want %q), stdout %q", code, f.err.String(), keWrRoleLine, f.out.String())
				}
				keWrSame(t, before, keWrSnapshot(t, f.env.GridHome))
				if n := len(keWrLog(t, f.env.GridHome)); n != lines {
					t.Errorf("log.md has %d lines, was %d", n, lines)
				}
			})
		}
	}
}

// With no member set, a member's workflow note is written and the audit line
// names the session, not a person.
func TestLoWaSessionAuthor(t *testing.T) {
	f := keWrNew(t)
	flow := keWrBytes("decisions", "flow", "# Flow\n", keWrWorkflow)
	line := loWaWrite(t, f, "created decisions/flow v1 rev "+brain.NoteRev(flow)+"\n", "member", "", "flow", flow, "none")
	if !regexp.MustCompile(`\| created \| flow \| session:[0-9a-f]{24} \|`).MatchString(line) || strings.Contains(line, "person:") {
		t.Fatalf("log line: %q", line)
	}
}
