package account

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"github.com/gridconsole/grid-plugins/internal/settings"
)

const (
	noteShowCommand = "grid account note --show"
	noteThenPrefix  = "grid account note --then "
)

// StatusLineResult is the answer of `grid account statusline on|off`.
type StatusLineResult struct {
	On   bool   `json:"on"`
	Path string `json:"path"`
	// Kept is the person's own status line command that Grid runs after it
	// notes the numbers; empty when they had none.
	Kept string `json:"kept"`
}

// StatusLineSet turns the status line that feeds the 5-hour window on or off
// in the Claude settings file of CLAUDE_CONFIG_DIR (or ~/.claude). On wraps the
// person's own command as `grid account note --then <their command>`, or uses
// `grid account note --show` when they have none. Off puts back exactly what
// was there. Either is a no-op when the file already is in that state.
func StatusLineSet(env Env, on bool) (StatusLineResult, error) {
	path, err := settings.ClaudeSettingsPath(env.Home, env.getenv(dirVars[Claude]))
	if err != nil {
		return StatusLineResult{}, err
	}
	res := StatusLineResult{On: on, Path: path}
	_, err = settings.EditClaudeStatusLine(path, func(cmd string, present bool) (string, bool) {
		theirs, wrapped := unwrapNote(cmd)
		if !present {
			theirs = ""
		}
		res.Kept = theirs
		switch {
		case on && wrapped:
			return cmd, false // already on
		case on && theirs == "":
			return noteShowCommand, false
		case on:
			return noteThenPrefix + shellQuote(theirs), false
		case !present:
			return "", true // nothing to take out
		case !wrapped:
			return cmd, false // not ours: leave it
		case theirs == "":
			return "", true
		}
		return theirs, false
	})
	return res, err
}

// unwrapNote answers the command a Grid status line command wraps ("" for
// --show) and whether cmd is one.
func unwrapNote(cmd string) (string, bool) {
	if cmd == noteShowCommand {
		return "", true
	}
	if rest, ok := strings.CutPrefix(cmd, noteThenPrefix); ok {
		return shellUnquote(rest), true
	}
	return cmd, false
}

var shellPlain = regexp.MustCompile(`^[A-Za-z0-9_./:=@%+,-]+$`)

// shellQuote makes cmd one shell word: bare when it is plain, else in single
// quotes, so a pipe or a space in the person's command stays inside --then.
func shellQuote(cmd string) string {
	if shellPlain.MatchString(cmd) {
		return cmd
	}
	return "'" + strings.ReplaceAll(cmd, "'", `'\''`) + "'"
}

// shellUnquote undoes shellQuote.
func shellUnquote(s string) string {
	if !strings.HasPrefix(s, "'") {
		return s
	}
	var b strings.Builder
	for rest := s; rest != ""; {
		switch {
		case strings.HasPrefix(rest, `\'`):
			b.WriteByte('\'')
			rest = rest[2:]
		case rest[0] == '\'':
			end := strings.IndexByte(rest[1:], '\'')
			if end < 0 {
				return s
			}
			b.WriteString(rest[1 : 1+end])
			rest = rest[end+2:]
		default:
			return s
		}
	}
	return b.String()
}

// NoteThen keeps the numbers of a status line payload, then runs the person's
// own command with the same stdin and passes its stdout through. A payload that
// cannot be read, or a folder that cannot be written, never stops the command:
// the person's line is never broken by Grid. It answers the command's exit code.
func NoteThen(env Env, in io.Reader, dir, then string, stdout, stderr io.Writer) int {
	data, _ := io.ReadAll(in)
	_, _ = NoteKept(env, bytes.NewReader(data), dir)
	name, flag := "sh", "-c"
	if _, err := exec.LookPath(name); err != nil && runtime.GOOS == "windows" {
		name, flag = "cmd", "/C"
	}
	cmd := exec.Command(name, flag, then)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = bytes.NewReader(data), stdout, stderr
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &exit):
		return exit.ExitCode()
	}
	fmt.Fprintf(stderr, "grid account note: cannot run %q: %v\n", then, err)
	return 127
}

// NoteShow keeps the numbers, then prints one short line such as
// `5h 42% · 7d 18%`; nothing when the payload holds no window.
func NoteShow(env Env, in io.Reader, dir string, stdout io.Writer) {
	data, _ := io.ReadAll(in)
	snap, err := parsePayload(env, data, dir)
	if err != nil {
		return
	}
	_, _ = NoteKept(env, bytes.NewReader(data), dir)
	var parts []string
	if snap.FiveHour != nil {
		parts = append(parts, fmt.Sprintf("5h %.0f%%", snap.FiveHour.Used))
	}
	if snap.SevenDay != nil {
		parts = append(parts, fmt.Sprintf("7d %.0f%%", snap.SevenDay.Used))
	}
	if len(parts) > 0 {
		fmt.Fprintln(stdout, strings.Join(parts, " · "))
	}
}
