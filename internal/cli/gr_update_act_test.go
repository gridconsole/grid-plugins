package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/updates"
)

// grUpdateNow is the minute every act in these tests runs at.
var grUpdateNow = time.Date(2026, 11, 11, 10, 0, 0, 0, time.UTC)

// grUpdateHarness is project alpha on a temp home with the update clock set.
func grUpdateHarness(t *testing.T) *harness {
	t.Helper()
	tmp := t.TempDir()
	for _, key := range []string{"HOME", "GRID_HOME", "CLAUDE_CONFIG_DIR", "CODEX_HOME"} {
		t.Setenv(key, filepath.Join(tmp, strings.ToLower(key)))
	}
	h := newHarness(t)
	h.env.Getenv = func(string) string { return "" }
	h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
	old := updates.Now
	updates.Now = func() time.Time { return grUpdateNow }
	t.Cleanup(func() { updates.Now = old })
	return h
}

// grUpdateTask makes a task in alpha, closed at the given moment unless it is zero.
func grUpdateTask(t *testing.T, h *harness, words string, closed time.Time) task.Task {
	t.Helper()
	at := closed
	if at.IsZero() {
		at = grUpdateNow.Add(-48 * time.Hour)
	}
	s := task.Store{Resolver: resolver(h.env), Now: func() time.Time { return at }}
	got, err := s.New(task.NewOptions{Cwd: h.home, Project: "alpha", Words: words, Size: "small", Workflow: "Check and ship", Who: "alex", Role: "member"})
	if err != nil {
		t.Fatal(err)
	}
	if !closed.IsZero() {
		if _, err := s.Change(task.WriteOptions{ID: got.ID, Action: "close"}); err != nil {
			t.Fatal(err)
		}
	}
	return got
}

func grUpdateDir(h *harness) string { return filepath.Join(h.gridHome, "projects", "alpha", "updates") }

func grUpdateRead(t *testing.T, h *harness, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(grUpdateDir(h), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func grUpdateBody(text string) []string {
	_, body, _ := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	return strings.Split(strings.TrimSuffix(body, "\n"), "\n")
}

// grUpdateGrammar is the app's grammar for one update file, copied from
// gridconsole-app internal/slices/projectupdates/updates.go: the name pattern
// (:38-42), the seven keys and their values (parseUpdate :84-168) and the body
// line (lineOf :169-184). A key's rule is "status", "text" (at most 300
// characters, no control character), "list" or "minute" (YYYY-MM-DDTHH:MM).
var grUpdateGrammar = struct {
	name, minute, line *regexp.Regexp
	keys               map[string]string
	needs              map[string]string
	sentOnly           []string
}{
	name:     regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2})-([a-z0-9][a-z0-9-]{0,59})\.md$`),
	minute:   regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}$`),
	line:     regexp.MustCompile(`^- (T-[0-9]+|[0-7][0-9A-HJKMNP-TV-Z]{25}) ·\s(\S.*)$`),
	keys:     map[string]string{"status": "status", "title": "text", "to": "list", "drafted": "minute", "sent": "minute", "read": "minute", "bounced": "minute"},
	needs:    map[string]string{"draft": "drafted", "sent": "sent", "skipped": ""},
	sentOnly: []string{"sent", "read", "bounced"},
}

// grUpdateCheckGrammar copies the file into a fresh folder and checks it there
// against grUpdateGrammar; it returns the front matter.
func grUpdateCheckGrammar(t *testing.T, src string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatal(err)
	}
	g := grUpdateGrammar
	if !g.name.MatchString(filepath.Base(dst)) {
		t.Fatalf("name %q does not match the app's name pattern", filepath.Base(dst))
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") || !strings.Contains(text, "\n---\n") {
		t.Fatalf("no front matter:\n%s", text)
	}
	head, _, _ := strings.Cut(strings.TrimPrefix(text, "---\n"), "\n---\n")
	front := map[string]string{}
	for _, row := range strings.Split(head, "\n") {
		key, value, ok := strings.Cut(row, ":")
		if !ok {
			t.Fatalf("front-matter row %q is not key: value", row)
		}
		rule, known := g.keys[key]
		if !known {
			t.Fatalf("key %q is not one of the app's seven keys", key)
		}
		value = strings.TrimSpace(value)
		switch rule {
		case "status":
			if _, ok := g.needs[value]; !ok {
				t.Fatalf("status %q", value)
			}
		case "text":
			value = strings.TrimSuffix(strings.TrimPrefix(value, `"`), `"`)
			if value == "" || len([]rune(value)) > 300 || strings.ContainsAny(value, "\n\r\t") {
				t.Fatalf("title %q", value)
			}
		case "minute":
			if _, err := time.Parse("2006-01-02T15:04", value); err != nil || !g.minute.MatchString(value) {
				t.Fatalf("%s %q is not a minute", key, value)
			}
		}
		front[key] = value
	}
	if need := g.needs[front["status"]]; need != "" && front[need] == "" {
		t.Fatalf("status %s without %s", front["status"], need)
	}
	for _, key := range g.sentOnly {
		if front["status"] != "sent" && front[key] != "" {
			t.Fatalf("%s on a %s update", key, front["status"])
		}
	}
	lines := grUpdateBody(text)
	if len(lines) == 0 || len(lines) > 100 {
		t.Fatalf("%d lines", len(lines))
	}
	for _, line := range lines {
		if !g.line.MatchString(line) {
			t.Fatalf("line %q does not match the app's line shape", line)
		}
	}
	return front
}

func TestGrUpdateActDraftWritesTheFile(t *testing.T) {
	h := grUpdateHarness(t)
	one := grUpdateTask(t, h, "Fix the label\n", grUpdateNow.Add(-2*time.Hour))
	two := grUpdateTask(t, h, "Ship the report\n", grUpdateNow.Add(-time.Hour))
	grUpdateTask(t, h, "Still open\n", time.Time{})
	if code := h.run(h.home, "update", "draft", "alpha", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), `"state":"applied"`) {
		t.Fatalf("stdout = %s", h.out)
	}
	var got updates.Result
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || len(got.Updates) != 1 || got.Updates[0].K != "2026-11-11-update" || got.Updates[0].Status != "draft" {
		t.Fatalf("result = %+v, %v", got, err)
	}
	text := grUpdateRead(t, h, "2026-11-11-update.md")
	for _, want := range []string{"status: draft\n", "drafted: 2026-11-11T10:00\n", `title: "Update 2026-11-11"` + "\n"} {
		if !strings.Contains(text, want) {
			t.Fatalf("file lacks %q:\n%s", want, text)
		}
	}
	want := []string{"- " + one.ID + " · " + one.Title, "- " + two.ID + " · " + two.Title}
	if lines := grUpdateBody(text); strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Fatalf("lines = %q, want %q", lines, want)
	}
}

func TestGrUpdateActFormatParity(t *testing.T) {
	h := grUpdateHarness(t)
	grUpdateTask(t, h, "Fix the label\n", grUpdateNow.Add(-2*time.Hour))
	grUpdateTask(t, h, "Ship the report\n", grUpdateNow.Add(-time.Hour))
	if code := h.run(h.home, "update", "draft", "alpha", "--title", "Week 46"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	path := filepath.Join(grUpdateDir(h), "2026-11-11-update.md")
	front := grUpdateCheckGrammar(t, path)
	if front["status"] != "draft" || front["title"] != "Week 46" || front["drafted"] != "2026-11-11T10:00" {
		t.Fatalf("front = %v", front)
	}
	before := grUpdateRead(t, h, "2026-11-11-update.md")
	code := h.run(h.home, "update", "draft", "alpha", "--json")
	stderr := h.errb.String()
	if code != 1 || !strings.Contains(stderr, "already drafted") {
		t.Fatalf("second draft exit %d: %s", code, stderr)
	}
	assertErrorOutput(t, h.out.String(), stderr, "changed")
	if after := grUpdateRead(t, h, "2026-11-11-update.md"); after != before {
		t.Fatalf("draft changed:\n%s\nwant:\n%s", after, before)
	}
	if entries, _ := os.ReadDir(grUpdateDir(h)); len(entries) != 1 {
		t.Fatalf("updates folder holds %d files", len(entries))
	}
}

func TestGrUpdateActEditSkipSendDiscard(t *testing.T) {
	h := grUpdateHarness(t)
	grUpdateTask(t, h, "Fix the label\n", grUpdateNow.Add(-2*time.Hour))
	run := func(want int, args ...string) {
		t.Helper()
		if code := h.run(h.home, append([]string{"update"}, args...)...); code != want {
			t.Fatalf("%v: exit %d, want %d: %s%s", args, code, want, h.out, h.errb)
		}
	}
	run(0, "draft", "alpha")
	run(0, "edit", "alpha", "--line", "T-01 · Label fixed on every page", "--line", "T-09 · Report ready for review")
	text := grUpdateRead(t, h, "2026-11-11-update.md")
	if got := grUpdateBody(text); strings.Join(got, "\n") != "- T-01 · Label fixed on every page\n- T-09 · Report ready for review" {
		t.Fatalf("edited lines = %q", got)
	}
	run(2, "edit", "alpha")
	if again := grUpdateRead(t, h, "2026-11-11-update.md"); again != text {
		t.Fatalf("edit with no line changed the draft:\n%s", again)
	}
	run(0, "send", "alpha")
	front := grUpdateCheckGrammar(t, filepath.Join(grUpdateDir(h), "2026-11-11-update.md"))
	if front["status"] != "sent" || front["sent"] != "2026-11-11T10:00" || front["drafted"] != "2026-11-11T10:00" {
		t.Fatalf("sent front = %v", front)
	}
	code := h.run(h.home, "update", "send", "alpha", "--json")
	if code != 1 {
		t.Fatalf("send with no draft: exit %d: %s", code, h.errb)
	}
	assertErrorOutput(t, h.out.String(), h.errb.String(), "changed")

	// Nothing closed since the sent minute: the draft holds the one line.
	run(0, "draft", "alpha")
	if got := grUpdateBody(grUpdateRead(t, h, "2026-11-11-update-2.md")); strings.Join(got, "\n") != "- T-00 · Nothing new since the last update." {
		t.Fatalf("draft after send = %q", got)
	}
	run(0, "skip", "alpha")
	if front := grUpdateCheckGrammar(t, filepath.Join(grUpdateDir(h), "2026-11-11-update-2.md")); front["status"] != "skipped" || front["sent"] != "" {
		t.Fatalf("skipped front = %v", front)
	}

	// A task closed after the sent minute is the next draft's one line.
	later := grUpdateTask(t, h, "Close the gap\n", grUpdateNow.Add(30*time.Minute))
	run(0, "draft", "alpha", "--json")
	if got := grUpdateBody(grUpdateRead(t, h, "2026-11-11-update-3.md")); strings.Join(got, "\n") != "- "+later.ID+" · "+later.Title {
		t.Fatalf("draft after a later close = %q", got)
	}
	var res updates.Result
	if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || len(res.Updates) != 3 || res.Updates[0].K != "2026-11-11-update" || res.Updates[0].At != "2026-11-11T10:00" {
		t.Fatalf("updates = %+v, %v", res.Updates, err)
	}
	run(0, "discard", "alpha")
	if _, err := os.Stat(filepath.Join(grUpdateDir(h), "2026-11-11-update-3.md")); !os.IsNotExist(err) {
		t.Fatalf("discarded draft still there: %v", err)
	}
	run(1, "discard", "alpha")
}

func TestGrUpdateActRefusals(t *testing.T) {
	h := grUpdateHarness(t)
	grUpdateTask(t, h, "Fix the label\n", grUpdateNow.Add(-time.Hour))
	if code := h.run(h.home, "update", "draft", "alpha"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	before := guestSnapshot(t, h.gridHome)
	owner := h.env.Getenv
	h.env.Getenv = func(k string) string {
		switch k {
		case "GRID_ROLE":
			return "guest"
		case "GRID_MEMBER":
			return "gus"
		}
		return ""
	}
	for _, args := range [][]string{{"draft", "alpha"}, {"edit", "alpha", "--line", "T-01 · x"}, {"discard", "alpha"}, {"skip", "alpha"}, {"send", "alpha"}} {
		if code := h.run(h.home, append([]string{"update"}, args...)...); code != 1 || !strings.Contains(h.errb.String(), guestSettingsRefusal) {
			t.Fatalf("guest %v: exit %d: %s", args, code, h.errb)
		}
	}
	if after := guestSnapshot(t, h.gridHome); after != before {
		t.Fatalf("a guest changed the Grid home:\n%s\nwant:\n%s", after, before)
	}
	h.env.Getenv = owner
	code := h.run(h.home, "update", "draft", "absent")
	stderr := h.errb.String()
	if code == 0 || stderr == "" || h.out.Len() != 0 {
		t.Fatalf("absent: exit %d, stdout %q, stderr %q", code, h.out, stderr)
	}
	if again := h.run(h.home, "update", "draft", "absent", "--json"); again != code || h.errb.String() != stderr {
		t.Fatalf("--json changed exit or stderr: %d %q, want %d %q", again, h.errb, code, stderr)
	}
	assertErrorOutput(t, h.out.String(), stderr, "no_such_project")
}
