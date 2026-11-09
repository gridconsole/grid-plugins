package braincmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// threeVersions publishes decisions/checks as v1, then v2 and v3 through
// merge-into, the only way an entry gets a new version.
func (h *harness) threeVersions(t *testing.T) brain.Store {
	t.Helper()
	h.propose(t)
	if code := h.run("accept", "checks", "--level", "home"); code != 0 {
		t.Fatalf("accept: %d %s", code, &h.err)
	}
	store := brain.Store{GridHome: h.env.GridHome}
	dir := t.TempDir()
	for _, name := range []string{"second", "third"} {
		b, err := brain.Encode(brain.Note{Name: name, Topic: "decisions", Type: "reference", Description: "Before changing checks.",
			Summary: "Run the checks.", Source: "job:T-01/J-01", Level: "home", Status: "proposed",
			Origin: brain.Origin{Kind: "job", ID: "T-01/J-01", Tier: "agent"}, Links: []brain.Link{}, Body: "# " + name + "\n"})
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, "brain-proposals", name+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.DecideTask(brain.TaskRef{ID: "T-01", Path: dir}, name, "merge-into", "brain:home/checks", "", func(string) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if v, err := store.Versions("home", "decisions", "checks"); err != nil || len(v) != 3 {
		t.Fatalf("versions: %+v, %v", v, err)
	}
	return store
}

func (h *harness) published(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type changesJSON struct {
	Changes []brain.Change `json:"changes"`
}

func (h *harness) changes(t *testing.T, args ...string) changesJSON {
	t.Helper()
	if code := h.run(append([]string{"changes", "--level", "home", "--json"}, args...)...); code != 0 {
		t.Fatalf("changes %v: %d %s", args, code, &h.err)
	}
	var got changesJSON
	if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, &h.out)
	}
	return got
}

func TestChangesListsNewestFirst(t *testing.T) {
	h := newHarness(t)
	store := h.threeVersions(t)
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home"); code != 0 {
		t.Fatalf("revert: %d %s", code, &h.err)
	}
	want, err := store.Changes("home", time.Time{}, 0)
	if err != nil || len(want) != 4 {
		t.Fatalf("store changes: %+v, %v", want, err)
	}
	got := h.changes(t)
	if !reflect.DeepEqual(got.Changes, want) {
		t.Fatalf("json differs from Store.Changes:\n%+v\n%+v", got.Changes, want)
	}
	if first := got.Changes[0]; first.Status != "reverted" || first.Version != 4 || first.Reason != "revert to v1" || first.Topic != "decisions" || first.Name != "checks" {
		t.Fatalf("newest change: %+v", first)
	}

	// Plain: at, status, topic/name, v<n>, hash, reason; one line each.
	if code := h.run("changes", "--level", "home"); code != 0 {
		t.Fatalf("plain: %d %s", code, &h.err)
	}
	lines := strings.Split(strings.TrimSuffix(h.out.String(), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("plain lines: %q", lines)
	}
	first := want[0]
	if lines[0] != first.At+" reverted decisions/checks v4 "+first.Hash+" revert to v1" {
		t.Fatalf("plain newest: %q", lines[0])
	}
	if f := strings.Fields(lines[3]); len(f) != 5 || f[1] != "accepted" || f[3] != "v1" || !strings.HasPrefix(f[4], "sha256:") {
		t.Fatalf("plain oldest (an empty reason adds no field): %q", lines[3])
	}
}

func TestChangesSinceAndLimitFilterAsTheStoreDoes(t *testing.T) {
	h := newHarness(t)
	store := h.threeVersions(t)
	all, err := store.Changes("home", time.Time{}, 0)
	if err != nil || len(all) != 3 {
		t.Fatalf("store changes: %+v, %v", all, err)
	}
	if got := h.changes(t, "--limit", "2"); !reflect.DeepEqual(got.Changes, all[:2]) {
		t.Fatalf("--limit 2: %+v", got.Changes)
	}
	// Zero or less caps nothing, as in the store.
	for _, limit := range []string{"0", "-1"} {
		if got := h.changes(t, "--limit", limit); len(got.Changes) != 3 {
			t.Fatalf("--limit %s: %+v", limit, got.Changes)
		}
	}
	for _, since := range []string{"2000-01-01T00:00:00Z", all[1].At[:19] + "Z", "2100-01-01T00:00:00Z", "2000-01-01T05:00:00+05:00"} {
		at, err := time.Parse(time.RFC3339, since)
		if err != nil {
			t.Fatal(err)
		}
		want, err := store.Changes("home", at, 0)
		if err != nil {
			t.Fatal(err)
		}
		got := h.changes(t, "--since", since)
		if len(got.Changes) != len(want) || len(want) > 0 && !reflect.DeepEqual(got.Changes, want) {
			t.Fatalf("--since %s: %+v, want %+v", since, got.Changes, want)
		}
	}
	if got := h.changes(t, "--since", "2100-01-01T00:00:00Z"); len(got.Changes) != 0 || !strings.Contains(h.out.String(), `"changes":[]`) {
		t.Fatalf("a future --since should list nothing: %s", &h.out)
	}
	if got := h.changes(t, "--since", "2000-01-01T00:00:00Z", "--limit", "1"); len(got.Changes) != 1 || got.Changes[0] != all[0] {
		t.Fatalf("--since with --limit: %+v", got.Changes)
	}
}

func TestChangesWithoutALogIsAnEmptyList(t *testing.T) {
	h := newHarness(t)
	if code := h.run("changes", "--level", "home", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != `{"changes":[]}` {
		t.Fatalf("missing log json: %d %s %s", code, &h.out, &h.err)
	}
	if code := h.run("changes", "--level", "home"); code != 0 || h.out.Len() != 0 || h.err.Len() != 0 {
		t.Fatalf("plain: %d %q %q", code, &h.out, &h.err)
	}
	if err := os.MkdirAll(filepath.Join(h.env.GridHome, "knowledge"), 0o700); err != nil {
		t.Fatal(err)
	}
	if code := h.run("changes", "--level", "home", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != `{"changes":[]}` {
		t.Fatalf("empty folder: %d %s", code, &h.out)
	}
}

func TestChangesUsageErrors(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"changes", "--since", "yesterday"},
		{"changes", "--since", "2026-01-02"},
		{"changes", "--limit", "many"},
		{"changes", "--level", "everywhere"},
		{"changes", "--level", "home", "--project", "p"},
		{"changes", "extra"},
		{"changes", "--bogus"},
	} {
		if code := h.run(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, &h.err)
		}
	}
}

func TestRevertPublishesAgainAndReportsTheNoOp(t *testing.T) {
	h := newHarness(t)
	store := h.threeVersions(t)
	v1, err := os.ReadFile(filepath.Join(h.env.GridHome, "knowledge", "versions", "decisions", "checks", "1.md"))
	if err != nil {
		t.Fatal(err)
	}
	if h.published(t) == string(v1) {
		t.Fatal("test needs different versions")
	}
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home"); code != 0 || h.out.String() != "v4\n" || h.err.Len() != 0 {
		t.Fatalf("revert: %d %q %q", code, &h.out, &h.err)
	}
	if h.published(t) != string(v1) {
		t.Fatal("the published entry is not version 1")
	}
	versions, err := store.Versions("home", "decisions", "checks")
	if err != nil || len(versions) != 4 {
		t.Fatalf("versions: %+v, %v", versions, err)
	}
	// The same request again changes nothing and says so.
	before := h.changes(t)
	for _, to := range []string{"1", "4"} {
		if code := h.run("revert", "decisions/checks", "--to", to, "--level", "home"); code != 0 || h.out.String() != "already at v4\n" {
			t.Fatalf("no-op to %s: %d %q %s", to, code, &h.out, &h.err)
		}
	}
	if code := h.run("revert", "decisions/checks", "--to", "4", "--level", "home", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != `{"version":4,"noop":true}` {
		t.Fatalf("no-op json: %d %q", code, &h.out)
	}
	if after := h.changes(t); !reflect.DeepEqual(after, before) {
		t.Fatalf("a no-op logged something: %+v", after)
	}
	if code := h.run("revert", "decisions/checks", "--to", "2", "--level", "home", "--json"); code != 0 || strings.TrimSpace(h.out.String()) != `{"version":5,"noop":false}` {
		t.Fatalf("json: %d %q", code, &h.out)
	}
}

// storedCopy reads version n of decisions/checks.
func (h *harness) storedCopy(t *testing.T, n int) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.env.GridHome, "knowledge", "versions", "decisions", "checks", strconv.Itoa(n)+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// noop means nothing was written: a revert that republishes an entry which
// differs from the newest stored copy is a write, even though no version is added.
func TestRevertReportsAWriteWhenItRepublishedTheEntry(t *testing.T) {
	publishedPath := func(h *harness) string {
		return filepath.Join(h.env.GridHome, "knowledge", "decisions", "checks.md")
	}
	damage := map[string]func(t *testing.T, h *harness){
		"entry holds v2": func(t *testing.T, h *harness) {
			if err := os.WriteFile(publishedPath(h), []byte(h.storedCopy(t, 2)), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"entry deleted": func(t *testing.T, h *harness) {
			if err := os.Remove(publishedPath(h)); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, hurt := range damage {
		for _, c := range []struct {
			format string
			args   []string
			want   string
		}{
			{"text", nil, "v3\n"},
			{"json", []string{"--json"}, `{"version":3,"noop":false}` + "\n"},
		} {
			t.Run(name+"/"+c.format, func(t *testing.T) {
				h := newHarness(t)
				h.threeVersions(t)
				v3 := h.storedCopy(t, 3)
				hurt(t, h)
				before := h.changes(t)
				args := append([]string{"revert", "decisions/checks", "--to", "3", "--level", "home"}, c.args...)
				if code := h.run(args...); code != 0 || h.out.String() != c.want || h.err.Len() != 0 {
					t.Fatalf("revert: %d %q %q, want %q", code, &h.out, &h.err, c.want)
				}
				if h.published(t) != v3 {
					t.Fatal("the entry does not hold version 3")
				}
				after := h.changes(t)
				if len(after.Changes) != len(before.Changes)+1 || after.Changes[0].Status != "reverted" || after.Changes[0].Version != 3 {
					t.Fatalf("want one new reverted line for v3: %+v then %+v", before.Changes, after.Changes)
				}
				if versions, err := (brain.Store{GridHome: h.env.GridHome}).Versions("home", "decisions", "checks"); err != nil || len(versions) != 3 {
					t.Fatalf("a copy was added: %+v, %v", versions, err)
				}
				// Now the entry is current, and the same request is the real no-op.
				if code := h.run("revert", "decisions/checks", "--to", "3", "--level", "home"); code != 0 || h.out.String() != "already at v3\n" {
					t.Fatalf("second revert: %d %q %s", code, &h.out, &h.err)
				}
			})
		}
	}
}

// The no-op writes nothing: not the entry, not the log, not a copy.
func TestRevertToTheCurrentVersionWritesNothingAndSaysSo(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	log := filepath.Join(h.env.GridHome, "knowledge", "log.md")
	logBefore, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	entryBefore := h.published(t)
	linesBefore := len(h.changes(t).Changes)
	for _, c := range []struct {
		args []string
		want string
	}{
		{nil, "already at v3\n"},
		{[]string{"--json"}, `{"version":3,"noop":true}` + "\n"},
	} {
		args := append([]string{"revert", "decisions/checks", "--to", "3", "--level", "home"}, c.args...)
		if code := h.run(args...); code != 0 || h.out.String() != c.want || h.err.Len() != 0 {
			t.Fatalf("%v: %d %q %q, want %q", c.args, code, &h.out, &h.err, c.want)
		}
	}
	logAfter, err := os.ReadFile(log)
	if err != nil || !bytes.Equal(logAfter, logBefore) {
		t.Fatalf("the log changed: %v\n%s", err, logAfter)
	}
	if got := len(h.changes(t).Changes); got != linesBefore {
		t.Fatalf("audit lines %d, want %d", got, linesBefore)
	}
	if h.published(t) != entryBefore {
		t.Fatal("the entry changed")
	}
}

func TestRevertFailures(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	before := h.published(t)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"decisions/checks", "--to", "9"}, "no version 9"},
		{[]string{"decisions/nothing", "--to", "1"}, "no version 1"},
		{[]string{"conventions/checks", "--to", "1"}, "not available at home level"},
		{[]string{"routines/daily", "--to", "1"}, "config topics"},
	} {
		if code := h.run(append([]string{"revert", "--level", "home"}, c.args...)...); code != 1 || h.out.Len() != 0 || !strings.Contains(h.err.String(), c.want) {
			t.Errorf("%v: %d %q %q", c.args, code, &h.out, &h.err)
		}
	}
	if code := h.run("revert", "decisions/checks", "--to", "9", "--level", "home", "--json"); code != 1 || !json.Valid(h.err.Bytes()) {
		t.Errorf("json failure: %d %q", code, &h.err)
	}
	if h.published(t) != before {
		t.Fatal("a failed revert changed the entry")
	}
	if code := h.run("changes", "--level", "home", "--json"); code != 0 || strings.Contains(h.out.String(), "reverted") {
		t.Fatalf("a failed revert was logged: %s", &h.out)
	}
}

func TestRevertUsageErrors(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	before := h.published(t)
	for _, args := range [][]string{
		{"revert"},
		{"revert", "decisions/checks"},
		{"revert", "decisions/checks", "--to", "0"},
		{"revert", "decisions/checks", "--to", "-1"},
		{"revert", "decisions/checks", "--to", "one"},
		{"revert", "checks", "--to", "1"},
		{"revert", "decisions/", "--to", "1"},
		{"revert", "/checks", "--to", "1"},
		{"revert", "decisions/a/b", "--to", "1"},
		{"revert", "../decisions/checks", "--to", "1"},
		{"revert", "decisions/..", "--to", "1"},
		{"revert", "decisions/checks", "decisions/checks", "--to", "1"},
		{"revert", "decisions/checks", "--to", "1", "--level", "everywhere"},
		{"revert", "decisions/checks", "--to", "1", "--level", "home", "--project", "p"},
		{"revert", "decisions/checks", "--to", "1", "--bogus"},
	} {
		if code := h.run(args...); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, &h.err)
		}
	}
	if h.published(t) != before {
		t.Fatal("a usage error changed the entry")
	}
}

func TestRolesDecideBothVerbs(t *testing.T) {
	for _, c := range []struct {
		role    string
		allowed bool
	}{
		{"", true}, {"owner", true}, {"member", true}, {" member ", true},
		{"guest", false}, {"agent", false}, {"Owner", false}, {"other", false},
	} {
		t.Run("role="+c.role, func(t *testing.T) {
			h := newHarness(t)
			h.threeVersions(t)
			before := h.published(t)
			h.env.Role = c.role
			changes := h.run("changes", "--level", "home")
			wantChanges := "grid brain changes: caller role is not allowed to read brain changes\n"
			if c.allowed && (changes != 0 || h.err.Len() != 0 || strings.Count(h.out.String(), "\n") != 3) {
				t.Fatalf("changes: %d %q %q", changes, &h.out, &h.err)
			}
			if !c.allowed && (changes != 3 || h.err.String() != wantChanges || h.out.Len() != 0) {
				t.Fatalf("changes: %d %q %q", changes, &h.out, &h.err)
			}
			revert := h.run("revert", "decisions/checks", "--to", "1", "--level", "home")
			wantRevert := "grid brain revert: caller role is not allowed to revert brain entries\n"
			if c.allowed && (revert != 0 || h.out.String() != "v4\n" || h.published(t) == before) {
				t.Fatalf("revert: %d %q %q", revert, &h.out, &h.err)
			}
			if !c.allowed && (revert != 3 || h.err.String() != wantRevert || h.out.Len() != 0 || h.published(t) != before) {
				t.Fatalf("revert: %d %q %q", revert, &h.out, &h.err)
			}
		})
	}
}

func TestRoleRefusalComesBeforeAnyReadOrWrite(t *testing.T) {
	h := newHarness(t)
	h.env.Role = "guest"
	// Nothing exists under this Grid home, and a refused caller must not create it or
	// learn whether a project is there.
	for _, args := range [][]string{
		{"changes", "--level", "project", "--project", "ghost"},
		{"revert", "decisions/checks", "--to", "1", "--level", "project", "--project", "ghost"},
		{"changes", "--json"},
		{"revert", "decisions/checks", "--to", "1", "--json"},
	} {
		if code := h.run(args...); code != 3 || !strings.HasPrefix(h.err.String(), "grid brain "+args[0]+": caller role is not allowed to ") || h.out.Len() != 0 {
			t.Errorf("%v: %d %q %q", args, code, &h.out, &h.err)
		}
	}
	if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
		t.Fatalf("a refused caller touched the Grid home: %v", err)
	}
}

func TestChangeLineNeverPrintsControlCharacters(t *testing.T) {
	line := changeLine(brain.Change{At: "2026-01-02T03:04:05.000Z", Status: "rejected", Name: "x\x1b[2Jy", Reason: "bell\a and\x00 nul\u0085"})
	if bytes.ContainsAny([]byte(line), "\x1b\a\x00\u0085") || !strings.Contains(line, "x?[2Jy") || !strings.Contains(line, "bell? and? nul?") {
		t.Fatalf("%q", line)
	}
	if want := "2026-01-02T03:04:05.000Z rejected x?[2Jy v0 - bell? and? nul?"; line != want {
		t.Fatalf("%q, want %q", line, want)
	}
}

// A revert whose index rebuild fails has still written the version: the caller
// must be told which one, with the error and exit 1.
func TestRevertReportsTheVersionWhenTheIndexRebuildFails(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		name := "text"
		if asJSON {
			name = "json"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			store := h.threeVersions(t)
			v1, err := os.ReadFile(filepath.Join(h.env.GridHome, "knowledge", "versions", "decisions", "checks", "1.md"))
			if err != nil {
				t.Fatal(err)
			}
			before := h.changes(t)
			h.out.Reset()
			h.err.Reset()
			idx := filepath.Join(h.env.GridHome, "knowledge", "index.md")
			if err := os.RemoveAll(idx); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(idx, "keep"), 0o700); err != nil {
				t.Fatal(err)
			}
			args := []string{"revert", "decisions/checks", "--to", "1", "--level", "home"}
			if asJSON {
				args = append(args, "--json")
			}
			code := h.run(args...)
			if code != 1 {
				t.Fatalf("exit %d, want 1: %q %q", code, &h.out, &h.err)
			}
			if asJSON {
				var got struct {
					Version *int   `json:"version"`
					Noop    *bool  `json:"noop"`
					Error   string `json:"error"`
				}
				if err := json.Unmarshal(h.out.Bytes(), &got); err != nil || got.Version == nil || *got.Version != 4 || got.Noop == nil || *got.Noop || got.Error == "" {
					t.Fatalf("stdout %q (%v)", &h.out, err)
				}
			} else if h.out.String() != "v4\n" {
				t.Fatalf("stdout %q, want v4", &h.out)
			}
			if !strings.Contains(h.err.String(), "rebuilding the index failed") {
				t.Fatalf("stderr %q", &h.err)
			}
			if h.published(t) != string(v1) {
				t.Fatal("entry does not hold the reverted bytes")
			}
			if v, err := store.Versions("home", "decisions", "checks"); err != nil || len(v) != 4 {
				t.Fatalf("versions: %+v, %v", v, err)
			}
			after := h.changes(t)
			if len(after.Changes) != len(before.Changes)+1 || after.Changes[0].Status != "reverted" || after.Changes[0].Version != 4 {
				t.Fatalf("want one new reverted line: %+v then %+v", before.Changes, after.Changes)
			}
		})
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("stdout closed") }

// When the index rebuild fails and the version line cannot be written either,
// the message keeps the rebuild error beside the write error.
func TestRevertKeepsTheRebuildErrorWhenStdoutFails(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	idx := filepath.Join(h.env.GridHome, "knowledge", "index.md")
	if err := os.RemoveAll(idx); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(idx, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	h.env.Stdout = failingWriter{}
	h.err.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home"); code != 1 {
		t.Fatalf("exit %d, want 1: %q", code, &h.err)
	}
	for _, want := range []string{"stdout closed", "rebuilding the index failed"} {
		if !strings.Contains(h.err.String(), want) {
			t.Fatalf("stderr %q lacks %q", &h.err, want)
		}
	}
}

// The same in --json mode: the failed stdout write must not drop the rebuild error.
func TestRevertKeepsTheRebuildErrorWhenStdoutFailsJSON(t *testing.T) {
	h := newHarness(t)
	h.threeVersions(t)
	idx := filepath.Join(h.env.GridHome, "knowledge", "index.md")
	if err := os.RemoveAll(idx); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(idx, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	h.env.Stdout = failingWriter{}
	h.err.Reset()
	if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home", "--json"); code != 1 {
		t.Fatalf("exit %d, want 1: %q", code, &h.err)
	}
	for _, want := range []string{"stdout closed", "rebuilding the index failed"} {
		if !strings.Contains(h.err.String(), want) {
			t.Fatalf("stderr %q lacks %q", &h.err, want)
		}
	}
}
