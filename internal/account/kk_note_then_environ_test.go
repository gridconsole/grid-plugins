package account

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
)

// These tests cover Env.Environ for `account note --then`: the person's command
// runs with the environment the Env holds, not the host process's. The host
// environment (t.Setenv) always carries KK_NOTE_X=host, so a child that still
// copied os.Environ would print "host".

// kkNotePayload is a status line payload with both windows, so NoteThen keeps it.
const kkNotePayload = `{"session_id":"s","rate_limits":{"five_hour":{"used_percentage":42,"resets_at":4102444800},"seven_day":{"used_percentage":18,"resets_at":4102444800}}}`

const kkNoteEcho = `echo "$KK_NOTE_X"`

// kkNoteShellOK skips on Windows: the command texts here are sh.
func kkNoteShellOK(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the --then command text is sh; Windows runs it with cmd /C")
	}
}

// kkNoteEnv is an Env that writes only under temp folders and, when withList is
// set, hands the child KK_NOTE_X=call plus the host's PATH entry.
func kkNoteEnv(t *testing.T, withList bool) Env {
	t.Helper()
	env := Env{Home: t.TempDir(), GridHome: t.TempDir()}
	if withList {
		list := []string{"KK_NOTE_X=call", "PATH=" + os.Getenv("PATH")}
		env.Environ = func() []string { return list }
	}
	return env
}

// kkNoteRun runs NoteThen on the payload and answers the exit code, stdout and stderr.
func kkNoteRun(t *testing.T, env Env, payload, then string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := NoteThen(env, strings.NewReader(payload), t.TempDir(), then, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestKkNoteThenEnvironReplacesHost(t *testing.T) {
	kkNoteShellOK(t)
	t.Setenv("KK_NOTE_X", "host")
	code, out, errOut := kkNoteRun(t, kkNoteEnv(t, true), "{}", kkNoteEcho)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != "call\n" {
		t.Fatalf("with Environ set the child reads that list: stdout %q, want %q", out, "call\n")
	}
}

func TestKkNoteThenEnvironNilReadsHost(t *testing.T) {
	kkNoteShellOK(t)
	t.Setenv("KK_NOTE_X", "host")
	code, out, errOut := kkNoteRun(t, kkNoteEnv(t, false), "{}", kkNoteEcho)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != "host\n" {
		t.Fatalf("with Environ nil the child reads the host environment: stdout %q, want %q", out, "host\n")
	}
}

func TestKkNoteThenEnvironEmptyListIsNotTheHost(t *testing.T) {
	kkNoteShellOK(t)
	t.Setenv("KK_NOTE_X", "host")
	env := kkNoteEnv(t, false)
	env.Environ = func() []string { return []string{} }
	code, out, errOut := kkNoteRun(t, env, "{}", kkNoteEcho)
	if code != 0 || errOut != "" {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != "\n" {
		t.Fatalf("an empty Environ is an empty environment, not the host's: stdout %q", out)
	}
}

// With Environ set, the rest of NoteThen is as before: the numbers are kept
// first, stdin is the payload's bytes, and the exit code is the command's.
func TestKkNoteThenEnvironKeepsNumbersStdinAndExit(t *testing.T) {
	kkNoteShellOK(t)
	t.Setenv("KK_NOTE_X", "host")
	env := kkNoteEnv(t, true)

	code, out, _ := kkNoteRun(t, env, kkNotePayload, "cat")
	if code != 0 || out != kkNotePayload {
		t.Fatalf("stdin is the payload: exit %d, stdout %q", code, out)
	}
	if code, _, _ := kkNoteRun(t, env, kkNotePayload, `test "$KK_NOTE_X" = call && exit 3`); code != 3 {
		t.Fatalf("the exit code is the command's: %d, want 3", code)
	}
	if code, out, _ := kkNoteRun(t, env, "not json", kkNoteEcho); code != 0 || out != "call\n" {
		t.Fatalf("an unreadable payload never stops the command: exit %d, stdout %q", code, out)
	}
	snaps, _ := os.ReadDir(env.GridHome)
	if len(snaps) == 0 {
		t.Fatal("the payload was not kept under the Grid home")
	}
}
