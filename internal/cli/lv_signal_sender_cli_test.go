package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// lvSigProject registers a project named example and returns its folder.
func lvSigProject(t *testing.T, h *harness) string {
	t.Helper()
	folder := filepath.Join(h.root, "work", "example")
	h.mkdir(folder)
	if code := h.run(folder, "project", "add"); code != 0 {
		t.Fatal(h.errb.String())
	}
	return folder
}

// lvSigNotes lists the note files the project holds.
func lvSigNotes(h *harness) []string {
	entries, _ := os.ReadDir(filepath.Join(h.gridHome, "projects", "example", "notes"))
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func lvSigNote(t *testing.T, h *harness, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.gridHome, "projects", "example", "notes", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLvSigAddRecordsSenderAndRecipient(t *testing.T) {
	h := newHarness(t)
	folder := lvSigProject(t, h)
	h.env.Stdin = strings.NewReader("Hello from the mail.\n")
	code := h.run(folder, "signal", "add", "--source", "mail", "--source-id", "m-1", "--title", "T",
		"--sender", "Marieke", "--recipient", "the vertex address", "--body", "-", "--json")
	if code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var r struct{ Result, Note string }
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil || r.Result != "written" {
		t.Fatalf("answer %q: %v", h.out, err)
	}
	note := lvSigNote(t, h, r.Note)
	head, body, ok := strings.Cut(strings.TrimPrefix(note, "---\n"), "\n---\n")
	if !ok {
		t.Fatalf("no front matter in %q", note)
	}
	lines := strings.Split(head, "\n")
	for _, want := range []string{"sender: Marieke", "recipient: the vertex address", "tier: external", "body_kind: data"} {
		found := false
		for _, l := range lines {
			found = found || l == want
		}
		if !found {
			t.Errorf("front matter lacks %q:\n%s", want, head)
		}
	}
	if body != "Hello from the mail.\n" {
		t.Errorf("body %q", body)
	}
}

// Without the flags the note carries neither key, so notes written by earlier
// callers and the signal-note golden read the same as before.
func TestLvSigAddWithoutFlagsWritesNoKeys(t *testing.T) {
	h := newHarness(t)
	folder := lvSigProject(t, h)
	h.env.Stdin = strings.NewReader("Body\n")
	if code := h.run(folder, "signal", "add", "--source", "mail", "--source-id", "m-2", "--title", "T", "--body", "-", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var r struct{ Note string }
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if note := lvSigNote(t, h, r.Note); strings.Contains(note, "sender") || strings.Contains(note, "recipient") {
		t.Fatalf("note holds a key nobody gave:\n%s", note)
	}
}

func TestLvSigAddRefusals(t *testing.T) {
	long := strings.Repeat("x", 513)
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"sender-newline", []string{"--sender", "Mar\nieke"}, "sender must be one line"},
		{"recipient-newline", []string{"--recipient", "a\nb"}, "recipient must be one line"},
		{"sender-long", []string{"--sender", long}, "sender must be UTF-8 text of at most 512 bytes"},
		{"recipient-long", []string{"--recipient", long}, "recipient must be UTF-8 text of at most 512 bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			folder := lvSigProject(t, h)
			base := []string{"signal", "add", "--source", "mail", "--title", "T", "--body", "x"}
			// A bad --source-id is the reference for the add path's failure exit.
			badID := h.run(folder, append(base, "--source-id", "a\nb")...)
			if badID == 0 {
				t.Fatal("a source id with a newline was accepted")
			}
			code := h.run(folder, append(append(base, "--source-id", "m-3"), tc.args...)...)
			if code != badID {
				t.Errorf("exit %d, want %d like a bad --source-id", code, badID)
			}
			if !strings.Contains(h.errb.String(), tc.want) {
				t.Errorf("stderr %q lacks %q", h.errb, tc.want)
			}
			if names := lvSigNotes(h); len(names) != 0 {
				t.Errorf("a refused add wrote %v", names)
			}
		})
	}
}

// roleFlag must skip the value of --sender and --recipient, whatever it starts
// with, or text from a source can pose as the --act the role policy matches on.
func TestLvSigRoleFlagSkipsSenderAndRecipient(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"values-before-act", []string{"signal", "act", "S-1", "--sender", "-x", "--recipient", "-y", "--act", "dismiss"}, "dismiss"},
		{"sender-value-looks-like-act", []string{"signal", "act", "S-1", "--act", "dismiss", "--sender", "--act=turn"}, "dismiss"},
		{"recipient-value-looks-like-act", []string{"signal", "act", "S-1", "--act", "dismiss", "--recipient", "--act=turn"}, "dismiss"},
		{"both-values-look-like-flags", []string{"signal", "act", "S-1", "--act", "dismiss", "--sender", "--act", "--recipient", "--act=turn"}, "dismiss"},
		{"single-dash", []string{"signal", "act", "S-1", "-act", "dismiss", "-sender", "-act=turn", "-recipient", "-act=turn"}, "dismiss"},
		{"equals-form", []string{"signal", "act", "S-1", "--sender=-x", "--recipient=-y", "--act", "snooze"}, "snooze"},
		{"act-is-value-only", []string{"signal", "act", "S-1", "--sender", "--act=turn"}, "note"},
		{"fence-stops", []string{"signal", "act", "S-1", "--sender", "-x", "--", "--act", "turn"}, "note"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := roleFlag(tc.args, "act", "note"); got != tc.want {
				t.Fatalf("roleFlag(%q) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}
