package account_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/cli"
)

type slHome struct {
	t           *testing.T
	home, grid  string
	configDir   string // CLAUDE_CONFIG_DIR, "" when unset
	out, errb   *bytes.Buffer
	stdin       string
	settingsRaw string
}

func newSLHome(t *testing.T) *slHome {
	t.Helper()
	home := t.TempDir()
	if r, err := filepath.EvalSymlinks(home); err == nil {
		home = r
	}
	h := &slHome{t: t, home: home, grid: filepath.Join(home, ".grid")}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *slHome) settingsPath() string {
	dir := filepath.Join(h.home, ".claude")
	if h.configDir != "" {
		dir = h.configDir
	}
	return filepath.Join(dir, "settings.json")
}

func (h *slHome) writeSettings(text string) {
	h.t.Helper()
	if err := os.MkdirAll(filepath.Dir(h.settingsPath()), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.WriteFile(h.settingsPath(), []byte(text), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *slHome) run(args ...string) int {
	h.t.Helper()
	h.out, h.errb = &bytes.Buffer{}, &bytes.Buffer{}
	env := cli.Env{
		Stdout: h.out, Stderr: h.errb, Stdin: strings.NewReader(h.stdin),
		Home: h.home, GridHome: h.grid, Cwd: h.home, Ceiling: h.home,
		Getenv: func(k string) string {
			if k == "CLAUDE_CONFIG_DIR" {
				return h.configDir
			}
			return ""
		},
	}
	return cli.Run(args, env)
}

func (h *slHome) settings() map[string]json.RawMessage {
	h.t.Helper()
	data, err := os.ReadFile(h.settingsPath())
	if err != nil {
		h.t.Fatal(err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		h.t.Fatalf("settings.json is not JSON: %v\n%s", err, data)
	}
	return m
}

func (h *slHome) statusLineCommand() (string, bool) {
	h.t.Helper()
	raw, ok := h.settings()["statusLine"]
	if !ok {
		return "", false
	}
	var sl struct {
		Type    string `json:"type"`
		Command string `json:"command"`
	}
	if err := json.Unmarshal(raw, &sl); err != nil || sl.Type != "command" {
		h.t.Fatalf("statusLine is not a command object: %s", raw)
	}
	return sl.Command, true
}

type slAnswer struct {
	On   bool   `json:"on"`
	Path string `json:"path"`
	Kept string `json:"kept"`
}

func (h *slHome) answer() slAnswer {
	h.t.Helper()
	var a slAnswer
	if err := json.Unmarshal(h.out.Bytes(), &a); err != nil {
		h.t.Fatalf("not JSON: %v\n%s", err, h.out)
	}
	return a
}

func without(m map[string]json.RawMessage, key string) map[string]json.RawMessage {
	out := map[string]json.RawMessage{}
	for k, v := range m {
		if k != key {
			// compare values as parsed JSON, not as text
			var x any
			_ = json.Unmarshal(v, &x)
			b, _ := json.Marshal(x)
			out[k] = b
		}
	}
	return out
}

const slSettings = `{
  "theme": "dark",
  "permissions": {"allow": ["Bash(git status)"], "deny": []},
  "big": 12345678901234567890
}
`

func TestStatusLineOnKeepsEveryOtherKeyAndIsIdempotent(t *testing.T) {
	h := newSLHome(t)
	h.writeSettings(slSettings)
	before := without(h.settings(), "statusLine")
	if code := h.run("account", "statusline", "on", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	a := h.answer()
	if !a.On || a.Kept != "" || a.Path != h.settingsPath() {
		t.Fatalf("answer %+v", a)
	}
	if cmd, ok := h.statusLineCommand(); !ok || cmd != "grid account note --show" {
		t.Fatalf("statusLine command %q present %v", cmd, ok)
	}
	if after := without(h.settings(), "statusLine"); !reflect.DeepEqual(before, after) {
		t.Fatalf("other keys changed:\nbefore %s\nafter  %s", before, after)
	}
	if !strings.Contains(string(mustRead(t, h.settingsPath())), "12345678901234567890") {
		t.Fatal("a number lost its precision")
	}

	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(h.settingsPath(), past, past); err != nil {
		t.Fatal(err)
	}
	bytesBefore := mustRead(t, h.settingsPath())
	infoBefore, _ := os.Stat(h.settingsPath())
	if code := h.run("account", "statusline", "on", "--json"); code != 0 {
		t.Fatalf("second on: exit %d: %s", code, h.errb)
	}
	infoAfter, _ := os.Stat(h.settingsPath())
	if !infoAfter.ModTime().Equal(infoBefore.ModTime()) || !bytes.Equal(bytesBefore, mustRead(t, h.settingsPath())) {
		t.Fatal("a second on rewrote the file")
	}
	if a := h.answer(); !a.On || a.Kept != "" {
		t.Fatalf("second answer %+v", a)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestStatusLineOnWrapsTheirCommandAndOffPutsItBack(t *testing.T) {
	h := newSLHome(t)
	original := `{
  "theme": "dark",
  "statusLine": {"type": "command", "command": "my-line.sh", "padding": 2},
  "z": [1, 2]
}
`
	h.writeSettings(original)
	if code := h.run("account", "statusline", "on", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if a := h.answer(); !a.On || a.Kept != "my-line.sh" {
		t.Fatalf("answer %+v", a)
	}
	if cmd, _ := h.statusLineCommand(); cmd != "grid account note --then my-line.sh" {
		t.Fatalf("command %q", cmd)
	}
	if code := h.run("account", "statusline", "on", "--json"); code != 0 || h.answer().Kept != "my-line.sh" {
		t.Fatalf("second on: exit %d, %s", code, h.out)
	}
	if cmd, _ := h.statusLineCommand(); cmd != "grid account note --then my-line.sh" {
		t.Fatalf("second on double-wrapped: %q", cmd)
	}
	if code := h.run("account", "statusline", "off", "--json"); code != 0 {
		t.Fatalf("off: exit %d: %s", code, h.errb)
	}
	if a := h.answer(); a.On || a.Kept != "my-line.sh" {
		t.Fatalf("off answer %+v", a)
	}
	var want, got any
	_ = json.Unmarshal([]byte(original), &want)
	_ = json.Unmarshal(mustRead(t, h.settingsPath()), &got)
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("off did not restore:\n%s", mustRead(t, h.settingsPath()))
	}
	line := h.settings()["statusLine"]
	if i, j := bytes.Index(line, []byte("command")), bytes.Index(line, []byte("padding")); i < 0 || j < i {
		t.Fatalf("statusLine field order lost: %s", line)
	}
}

func TestStatusLineOffRemovesTheKeyWeMade(t *testing.T) {
	h := newSLHome(t)
	h.writeSettings(slSettings)
	if code := h.run("account", "statusline", "on"); code != 0 {
		t.Fatalf("on: exit %d: %s", code, h.errb)
	}
	if code := h.run("account", "statusline", "off", "--json"); code != 0 {
		t.Fatalf("off: exit %d: %s", code, h.errb)
	}
	if _, ok := h.settings()["statusLine"]; ok {
		t.Fatal("off left a statusLine key")
	}
	if a := h.answer(); a.On || a.Kept != "" {
		t.Fatalf("answer %+v", a)
	}
	// off on a file with no statusLine, and with no file at all, writes nothing
	before := mustRead(t, h.settingsPath())
	if code := h.run("account", "statusline", "off"); code != 0 || !bytes.Equal(before, mustRead(t, h.settingsPath())) {
		t.Fatalf("second off: exit %d", code)
	}
	h2 := newSLHome(t)
	if code := h2.run("account", "statusline", "off"); code != 0 {
		t.Fatalf("off, no file: exit %d: %s", code, h2.errb)
	}
	if _, err := os.Stat(h2.settingsPath()); !os.IsNotExist(err) {
		t.Fatal("off made a settings file")
	}
}

func TestStatusLineQuotesACommandThatIsNotOneWord(t *testing.T) {
	h := newSLHome(t)
	their := `~/bin/line.sh --compact | tr a-z A-Z; echo 'it''s'`
	b, _ := json.Marshal(map[string]any{"statusLine": map[string]any{"type": "command", "command": their}})
	h.writeSettings(string(b))
	if code := h.run("account", "statusline", "on", "--json"); code != 0 || h.answer().Kept != their {
		t.Fatalf("on: exit %d: %s", code, h.out)
	}
	cmd, _ := h.statusLineCommand()
	if !strings.HasPrefix(cmd, "grid account note --then '") {
		t.Fatalf("command not quoted: %q", cmd)
	}
	if code := h.run("account", "statusline", "off"); code != 0 {
		t.Fatalf("off: exit %d", code)
	}
	if cmd, _ := h.statusLineCommand(); cmd != their {
		t.Fatalf("restored %q, want %q", cmd, their)
	}
}

func TestStatusLineRefusals(t *testing.T) {
	h := newSLHome(t)
	h.writeSettings(`{"statusLine": {"type": "static", "text": "x"}}`)
	before := mustRead(t, h.settingsPath())
	if code := h.run("account", "statusline", "on"); code != 1 || !strings.Contains(h.errb.String(), "statusline") {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	h.writeSettings(`{not json`)
	if code := h.run("account", "statusline", "on"); code == 0 {
		t.Fatal("on accepted a settings file that is not JSON")
	}
	if string(mustRead(t, h.settingsPath())) != `{not json` || len(before) == 0 {
		t.Fatal("a refused run wrote the file")
	}
	if code := h.run("account", "statusline"); code != 2 {
		t.Fatalf("no on|off: exit %d", code)
	}
	if code := h.run("account", "statusline", "maybe"); code != 2 {
		t.Fatalf("bad word: exit %d", code)
	}
}

func TestStatusLineFollowsClaudeConfigDir(t *testing.T) {
	h := newSLHome(t)
	h.configDir = filepath.Join(h.home, ".claude-work")
	h.writeSettings(`{"theme":"light"}`)
	if code := h.run("account", "statusline", "on", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if a := h.answer(); a.Path != filepath.Join(h.configDir, "settings.json") {
		t.Fatalf("path %q", a.Path)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Fatal("wrote the default folder's settings")
	}
}

const payload42 = `{"session_id":"s","rate_limits":{"five_hour":{"used_percentage":42,"resets_at":4102444800},"seven_day":{"used_percentage":18,"resets_at":4102444800}}}`

func shellOK(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell for --then")
	}
}

func snapshotFiles(t *testing.T, h *slHome) []string {
	t.Helper()
	files, _ := filepath.Glob(filepath.Join(h.grid, "accounts", "windows", "*.json"))
	return files
}

func TestNoteThenKeepsNumbersAndPassesTheirOutput(t *testing.T) {
	shellOK(t)
	h := newSLHome(t)
	h.stdin = payload42
	if code := h.run("account", "note", "--then", "printf hi"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if h.out.String() != "hi" {
		t.Fatalf("stdout %q", h.out)
	}
	if len(snapshotFiles(t, h)) != 1 {
		t.Fatal("no snapshot written")
	}
	// the person's command reads the same stdin
	h.stdin = payload42
	if code := h.run("account", "note", "--then", "cat"); code != 0 || h.out.String() != payload42 {
		t.Fatalf("exit %d, stdout %q", code, h.out)
	}
	// and its exit code is passed through
	h.stdin = payload42
	if code := h.run("account", "note", "--then", "exit 3"); code != 3 {
		t.Fatalf("exit %d, want 3", code)
	}
}

func TestNoteThenSurvivesAPayloadItCannotParse(t *testing.T) {
	shellOK(t)
	for _, in := range []string{"this is not json", ""} {
		h := newSLHome(t)
		h.stdin = in
		if code := h.run("account", "note", "--then", "printf hi"); code != 0 || h.out.String() != "hi" {
			t.Fatalf("stdin %q: exit %d, stdout %q, stderr %q", in, code, h.out, h.errb)
		}
		if len(snapshotFiles(t, h)) != 0 {
			t.Fatal("a snapshot was written for nothing")
		}
	}
	// no Grid home: still the person's line
	h := newSLHome(t)
	h.grid = ""
	h.stdin = payload42
	if code := h.run("account", "note", "--then", "printf hi"); code != 0 || h.out.String() != "hi" {
		t.Fatalf("no grid home: exit %d, stdout %q", code, h.out)
	}
}

func TestNoteShowPrintsBothWindowsUnderTheConfigFolder(t *testing.T) {
	h := newSLHome(t)
	h.stdin = payload42
	if code := h.run("account", "note", "--show"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	line := h.out.String()
	if !strings.Contains(line, "42%") || !strings.Contains(line, "18%") || strings.Count(line, "\n") != 1 {
		t.Fatalf("line %q", line)
	}
	first := snapshotFiles(t, h)
	if len(first) != 1 {
		t.Fatalf("snapshots %v", first)
	}

	h.configDir = filepath.Join(h.home, ".claude-second")
	if err := os.MkdirAll(h.configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	h.stdin = payload42
	if code := h.run("account", "note", "--show"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	both := snapshotFiles(t, h)
	if len(both) != 2 {
		t.Fatalf("a second folder's reading did not get its own snapshot: %v", both)
	}
}

func TestNoteShowIsEmptyWithoutAWindow(t *testing.T) {
	for _, in := range []string{`{"session_id":"s"}`, "garbage", ""} {
		h := newSLHome(t)
		h.stdin = in
		if code := h.run("account", "note", "--show"); code != 0 || h.out.Len() != 0 {
			t.Fatalf("stdin %q: exit %d, stdout %q", in, code, h.out)
		}
	}
	h := newSLHome(t)
	h.stdin = `{"rate_limits":{"seven_day":{"used_percentage":7.6,"resets_at":1}}}`
	if code := h.run("account", "note", "--show"); code != 0 || strings.TrimSpace(h.out.String()) != "7d 8%" {
		t.Fatalf("exit %d, stdout %q", code, h.out)
	}
}
