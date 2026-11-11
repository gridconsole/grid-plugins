package account_test

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// ntSlSame fails when the settings file's bytes differ from before.
func ntSlSame(t *testing.T, h *slHome, before []byte, when string) {
	t.Helper()
	if after := mustRead(t, h.settingsPath()); !bytes.Equal(before, after) {
		t.Fatalf("%s: the read changed the file\nbefore: %s\nafter:  %s", when, before, after)
	}
}

// ntSlRead runs the no-word read and answers its JSON, failing on a non-zero exit.
func ntSlRead(t *testing.T, h *slHome) slAnswer {
	t.Helper()
	if code := h.run("account", "statusline", "--json"); code != 0 {
		t.Fatalf("read: exit %d: %s", code, h.errb)
	}
	return h.answer()
}

func TestNtSlReadAfterOn(t *testing.T) {
	h := newSLHome(t)
	if code := h.run("account", "statusline", "on", "--json"); code != 0 {
		t.Fatalf("on: exit %d: %s", code, h.errb)
	}
	written := h.answer()
	if !written.On {
		t.Fatalf("on answered %+v", written)
	}
	got := ntSlRead(t, h)
	if !got.On || got.Path != written.Path {
		t.Fatalf("read answered %+v, want on at %s", got, written.Path)
	}
}

func TestNtSlReadChangesNothing(t *testing.T) {
	h := newSLHome(t)
	// No settings file: off, and the read creates nothing.
	if a := ntSlRead(t, h); a.On || a.Path != h.settingsPath() {
		t.Fatalf("no file: %+v", a)
	}
	if _, err := os.Stat(h.settingsPath()); !os.IsNotExist(err) {
		t.Fatalf("the read created the settings file: %v", err)
	}
	// A file with no statusLine: off.
	h.writeSettings(`{"theme": "dark"}`)
	before := mustRead(t, h.settingsPath())
	if a := ntSlRead(t, h); a.On || a.Kept != "" {
		t.Fatalf("no statusLine: %+v", a)
	}
	ntSlSame(t, h, before, "no statusLine")

	const their = "echo mine | cat"
	h.writeSettings(`{"statusLine": {"type": "command", "command": "` + their + `"}}`)
	if code := h.run("account", "statusline", "on"); code != 0 {
		t.Fatalf("on: exit %d: %s", code, h.errb)
	}
	before = mustRead(t, h.settingsPath())
	if a := ntSlRead(t, h); !a.On || a.Kept != their {
		t.Fatalf("after on: %+v", a)
	}
	ntSlSame(t, h, before, "after on")

	if code := h.run("account", "statusline", "off"); code != 0 {
		t.Fatalf("off: exit %d: %s", code, h.errb)
	}
	before = mustRead(t, h.settingsPath())
	if a := ntSlRead(t, h); a.On || a.Kept != their {
		t.Fatalf("after off: %+v", a)
	}
	ntSlSame(t, h, before, "after off")

	const own = "my-own-line --fancy"
	h.writeSettings(`{"statusLine": {"type": "command", "command": "` + own + `"}}`)
	before = mustRead(t, h.settingsPath())
	if a := ntSlRead(t, h); a.On || a.Kept != own {
		t.Fatalf("their own command: %+v", a)
	}
	ntSlSame(t, h, before, "their own command")

	// The words without --json are the ones the write prints.
	if code := h.run("account", "statusline"); code != 0 || !strings.HasPrefix(h.out.String(), "account statusline off: "+h.settingsPath()) {
		t.Fatalf("plain read: exit %d: %s", code, h.out)
	}
	ntSlSame(t, h, before, "plain read")
}

func TestNtSlReadRefusesBadFile(t *testing.T) {
	// The reasons are the ones the write gives (claude.go claudeInvalid, claude_statusline.go).
	for text, reason := range map[string]string{
		`{not json`: "is not valid JSON",
		`{"statusLine": {"type": "static", "text": "x"}}`: "statusLine is not a command status line",
	} {
		h := newSLHome(t)
		h.writeSettings(text)
		before := mustRead(t, h.settingsPath())
		if code := h.run("account", "statusline", "--json"); code != 1 {
			t.Fatalf("read of %q: exit %d, want 1: %s", text, code, h.out)
		}
		if !strings.Contains(h.errb.String(), reason) {
			t.Fatalf("read of %q: stderr %q lacks %q", text, h.errb, reason)
		}
		ntSlSame(t, h, before, text)
	}
}
