package brain

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var keRnAna = Origin{Kind: "person", ID: "ana"}

func keRnStore(t *testing.T) Store {
	t.Helper()
	return Store{GridHome: filepath.Join(t.TempDir(), ".grid"), Project: "example", Now: func() time.Time {
		return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}}
}

// keRnDir is the folder of the project level inside the Grid home.
func keRnDir(s Store) string { return filepath.Join(s.GridHome, "projects", s.Project) }

func keRnLink(to string) Link {
	return Link{To: to, Rel: "related", Summary: "The note this one points at.", Why: "Shows why the choice was made."}
}

func keRnNote(topic, name string, links ...Link) Note {
	if links == nil {
		links = []Link{}
	}
	return Note{Name: name, Topic: topic, Type: "reference", Description: "Read before changing " + name + ".",
		Summary: "Summary of " + name + ".", Source: "job:T-01/J-01", Level: "project", Status: "proposed",
		Origin: Origin{Kind: "job", ID: "T-01/J-01"}, Links: links, Body: "# " + name + "\n\nBody of " + name + ".\n"}
}

func keRnAccept(t *testing.T, s Store, n Note) {
	t.Helper()
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept(n.Level, n.Name); err != nil {
		t.Fatal(err)
	}
}

func keRnPath(s Store, rel string) string { return filepath.Join(keRnDir(s), filepath.FromSlash(rel)) }

func keRnRead(t *testing.T, s Store, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(keRnPath(s, rel))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func keRnRev(t *testing.T, s Store, topic, name string) string {
	t.Helper()
	return NoteRev(keRnRead(t, s, "knowledge/"+topic+"/"+name+".md"))
}

func keRnLog(t *testing.T, s Store) string { return string(keRnRead(t, s, "knowledge/log.md")) }

// keRnVersions lists the stored copies of one note, by file name.
func keRnVersions(t *testing.T, s Store, topic, name string) map[string]string {
	t.Helper()
	out := map[string]string{}
	entries, err := os.ReadDir(keRnPath(s, "knowledge/versions/"+topic+"/"+name))
	if errors.Is(err, fs.ErrNotExist) {
		return out
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		out[e.Name()] = string(keRnRead(t, s, "knowledge/versions/"+topic+"/"+name+"/"+e.Name()))
	}
	return out
}

// keRnTree maps every path below the Grid home to its bytes, "dir" or the target
// of a link, without following links. The lock file is left out: taking the lock
// creates it.
func keRnTree(t *testing.T, s Store) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(s.GridHome, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(s.GridHome, p)
		switch {
		case d.Name() == lockName:
		case d.Type()&fs.ModeSymlink != 0:
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			out[rel] = "link:" + target
		case d.IsDir():
			out[rel] = "dir"
		default:
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			out[rel] = "file:" + string(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// keRnFixture builds the level of the brief: decisions/a, which supersedes
// decisions/d; c links a with an anchor; e links nothing; f links a and is
// dropped. It returns the store and the rev of a.
func keRnFixture(t *testing.T) (Store, string) {
	t.Helper()
	s := keRnStore(t)
	keRnAccept(t, s, keRnNote("decisions", "d"))
	supersedes := keRnLink("brain:project/d#old")
	supersedes.Rel = "supersedes"
	keRnAccept(t, s, keRnNote("decisions", "a", supersedes))
	keRnAccept(t, s, keRnNote("decisions", "c", keRnLink("brain:project/a#why")))
	keRnAccept(t, s, keRnNote("decisions", "e"))
	keRnAccept(t, s, keRnNote("decisions", "f", keRnLink("brain:project/a")))
	if _, err := s.Drop("project", "decisions", "f", OpDrop, keRnRev(t, s, "decisions", "f"), keRnAna, ""); err != nil {
		t.Fatal(err)
	}
	return s, keRnRev(t, s, "decisions", "a")
}

func TestKeRnStore(t *testing.T) {
	s, rev := keRnFixture(t)
	if !strings.Contains(string(keRnRead(t, s, "knowledge/decisions/d.md")), "superseded_by: brain:project/a") {
		t.Fatalf("fixture: d is not superseded by a:\n%s", keRnRead(t, s, "knowledge/decisions/d.md"))
	}
	oldC, oldD := keRnRead(t, s, "knowledge/decisions/c.md"), keRnRead(t, s, "knowledge/decisions/d.md")
	oldE, oldF := keRnRead(t, s, "knowledge/decisions/e.md"), keRnRead(t, s, "knowledge/decisions/f.md")
	versionsA := keRnVersions(t, s, "decisions", "a")
	versionsC, versionsD := keRnVersions(t, s, "decisions", "c"), keRnVersions(t, s, "decisions", "d")
	if len(versionsA) == 0 || len(versionsC) != 1 || len(versionsD) != 2 {
		t.Fatalf("fixture versions: a %d, c %d, d %d", len(versionsA), len(versionsC), len(versionsD))
	}

	res, err := s.Rename("project", "decisions", "a", "b", rev, keRnAna, "clearer name")
	if err != nil {
		t.Fatal(err)
	}
	b := keRnRead(t, s, "knowledge/decisions/b.md")
	if !bytes.Contains(b, []byte("name: b\n")) || bytes.Contains(b, []byte("name: a\n")) {
		t.Fatalf("b.md does not carry its new name:\n%s", b)
	}
	if _, err := os.Stat(keRnPath(s, "knowledge/decisions/a.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a.md is still there: %v", err)
	}
	if got := keRnVersions(t, s, "decisions", "b"); !reflect.DeepEqual(got, map[string]string{"1.md": string(b)}) {
		t.Fatalf("versions of b: %v", got)
	}
	if got := keRnVersions(t, s, "decisions", "a"); !reflect.DeepEqual(got, versionsA) {
		t.Fatalf("versions of a changed:\n%v\nwas\n%v", got, versionsA)
	}
	wantC := strings.ReplaceAll(string(oldC), "brain:project/a#why", "brain:project/b#why")
	wantD := strings.ReplaceAll(string(oldD), "superseded_by: brain:project/a", "superseded_by: brain:project/b")
	if got := string(keRnRead(t, s, "knowledge/decisions/c.md")); got != wantC || wantC == string(oldC) {
		t.Fatalf("c is:\n%s\nwant:\n%s", got, wantC)
	}
	if got := string(keRnRead(t, s, "knowledge/decisions/d.md")); got != wantD || wantD == string(oldD) {
		t.Fatalf("d is:\n%s\nwant:\n%s", got, wantD)
	}
	for name, want := range map[string]string{"c": wantC, "d": wantD} {
		versions := keRnVersions(t, s, "decisions", name)
		old := map[string]map[string]string{"c": versionsC, "d": versionsD}[name]
		if len(versions) != len(old)+1 || versions[strconv.Itoa(len(old)+1)+".md"] != want {
			t.Fatalf("versions of %s: %d, want %d with the new bytes last", name, len(versions), len(old)+1)
		}
	}
	if !bytes.Equal(keRnRead(t, s, "knowledge/decisions/e.md"), oldE) || !bytes.Equal(keRnRead(t, s, "knowledge/decisions/f.md"), oldF) {
		t.Fatal("e or f changed")
	}
	want := RenameResult{Status: "renamed", From: "decisions/a", To: "decisions/b", Version: 1, Rev: NoteRev(b),
		Linked: []string{"decisions/c", "decisions/d"}, Kept: []string{}, Skipped: []string{"decisions/f"}}
	if !reflect.DeepEqual(res, want) {
		t.Fatalf("result %+v, want %+v", res, want)
	}
	log := keRnLog(t, s)
	if n := strings.Count(log, "| renamed | b |"); n != 1 {
		t.Fatalf("%d renamed lines:\n%s", n, log)
	}
	for _, line := range strings.Split(log, "\n") {
		if strings.Contains(line, "| renamed | b |") && !strings.Contains(line, "| person:ana | from decisions/a: clearer name | decisions | v1 | ") {
			t.Fatalf("renamed line: %s", line)
		}
	}
	for _, name := range []string{"c", "d"} {
		if n := strings.Count(log, "| linked | "+name+" |"); n != 1 {
			t.Fatalf("%d linked lines for %s:\n%s", n, name, log)
		}
	}
	if strings.Contains(log, "| linked | e |") || strings.Contains(log, "| linked | f |") {
		t.Fatalf("e or f was logged as linked:\n%s", log)
	}
	// backlinks.json keys on the whole link target, anchor included, and lists a
	// dropped note's links too: so c's link is the key b#why, and the key a that
	// stays is f's, the dropped note that is not rewritten.
	var back struct {
		Links map[string][]struct {
			From string `json:"from"`
		} `json:"links"`
	}
	if err := json.Unmarshal(keRnRead(t, s, "knowledge/backlinks.json"), &back); err != nil {
		t.Fatal(err)
	}
	for key, from := range back.Links {
		switch key {
		case "brain:project/b#why":
			if len(from) != 1 || from[0].From != "brain:project/c" {
				t.Fatalf("backlinks of b#why: %+v", from)
			}
		case "brain:project/a":
			if len(from) != 1 || from[0].From != "brain:project/f" {
				t.Fatalf("backlinks of a: %+v", from)
			}
		case "brain:project/d#old":
		default:
			t.Fatalf("unexpected backlinks key %q in %+v", key, back.Links)
		}
	}
	if len(back.Links["brain:project/b#why"]) != 1 {
		t.Fatalf("no backlinks key for b: %+v", back.Links)
	}
	check, err := s.Check()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range check.Findings {
		if f.Code == CheckAuditHash {
			t.Fatalf("audit_hash finding: %+v", f)
		}
	}
}

func TestKeRnRefuse(t *testing.T) {
	s, rev := keRnFixture(t)
	keRnAccept(t, s, keRnNote("decisions", "g"))
	keRnAccept(t, s, keRnNote("conventions", "k"))
	// d is superseded by a, so it is the superseded note.
	wf := keRnNote("decisions", "w")
	wf.Type = "workflow"
	keRnAccept(t, s, wf)
	// A note placed by hand has no stored version.
	hand, err := Encode(func() Note { n := keRnNote("decisions", "h"); n.Status = "accepted"; return n }())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keRnPath(s, "knowledge/decisions/h.md"), hand, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("g.md", keRnPath(s, "knowledge/decisions/s.md")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name                string
		topic, from, to, at string
		check               func(error) bool
	}{
		{"stale rev", "decisions", "a", "b", "0000000000000000", func(err error) bool {
			var c *ConflictError
			return errors.As(err, &c) && c.Rev == rev && errors.Is(err, ErrConflict)
		}},
		{"missing note", "decisions", "nope", "b", rev, func(err error) bool { return errors.Is(err, ErrNoSuchNote) }},
		{"settings topic", "machines", "x", "y", rev, func(err error) bool { return errors.Is(err, ErrRenameNotVersioned) }},
		{"no stored version", "decisions", "h", "i", NoteRev(hand), func(err error) bool { return errors.Is(err, ErrRenameNotVersioned) }},
		{"held by the same topic", "decisions", "a", "g", rev, keRnRefused},
		{"held by another topic", "decisions", "a", "k", rev, keRnRefused},
		{"new name TOPIC", "decisions", "a", "TOPIC", rev, keRnRefused},
		{"new name is the old name", "decisions", "a", "a", rev, keRnRefused},
		{"new name is not a name", "decisions", "a", "b/c", rev, keRnRefused},
		{"dropped", "decisions", "f", "i", keRnRev(t, s, "decisions", "f"), keRnRefused},
		{"superseded", "decisions", "d", "i", keRnRev(t, s, "decisions", "d"), keRnRefused},
		{"workflow", "decisions", "w", "i", keRnRev(t, s, "decisions", "w"), keRnRefused},
		{"symlinked file", "decisions", "s", "i", rev, func(err error) bool { return err != nil && !errors.Is(err, ErrNoSuchNote) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := keRnTree(t, s)
			res, err := s.Rename("project", tc.topic, tc.from, tc.to, tc.at, keRnAna, "")
			if err == nil || !tc.check(err) {
				t.Fatalf("error %v (result %+v)", err, res)
			}
			if res.Status != "" {
				t.Fatalf("result %+v with an error", res)
			}
			if after := keRnTree(t, s); !reflect.DeepEqual(before, after) {
				t.Fatalf("the Grid home changed: %s", keRnDiff(before, after))
			}
		})
	}
}

func keRnRefused(err error) bool {
	var r *RefusedError
	return errors.As(err, &r) && errors.Is(err, ErrDropRefused)
}

func keRnDiff(before, after map[string]string) string {
	var out []string
	for k, v := range after {
		if before[k] != v {
			out = append(out, "changed or new: "+k)
		}
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			out = append(out, "gone: "+k)
		}
	}
	return strings.Join(out, ", ")
}

func TestKeRnHalfWay(t *testing.T) {
	s, rev := keRnFixture(t)
	oldC := keRnRead(t, s, "knowledge/decisions/c.md")
	versionsC := keRnVersions(t, s, "decisions", "c")
	logBefore := keRnLog(t, s)
	var calls []string
	keRnWriteHook = func(rel string) error {
		calls = append(calls, rel)
		if rel == "knowledge/decisions/c.md" {
			return errors.New("disk full")
		}
		return nil
	}
	t.Cleanup(func() { keRnWriteHook = nil })
	res, err := s.Rename("project", "decisions", "a", "b", rev, keRnAna, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Kept, []string{"decisions/c"}) || !reflect.DeepEqual(res.Linked, []string{"decisions/d"}) ||
		!reflect.DeepEqual(res.Skipped, []string{"decisions/f"}) {
		t.Fatalf("kept %v, linked %v, skipped %v", res.Kept, res.Linked, res.Skipped)
	}
	if !reflect.DeepEqual(calls, []string{"knowledge/decisions/c.md", "knowledge/decisions/d.md"}) {
		t.Fatalf("the hook was called for %v", calls)
	}
	if !bytes.Equal(keRnRead(t, s, "knowledge/decisions/c.md"), oldC) || !reflect.DeepEqual(keRnVersions(t, s, "decisions", "c"), versionsC) {
		t.Fatal("c or its versions changed")
	}
	log := keRnLog(t, s)
	if !strings.HasPrefix(log, logBefore) || strings.Contains(log[len(logBefore):], "| linked | c |") {
		t.Fatalf("c was logged:\n%s", log[len(logBefore):])
	}
	if !strings.Contains(log[len(logBefore):], "| linked | d |") || !strings.Contains(log[len(logBefore):], "| renamed | b |") {
		t.Fatalf("d or the rename is not logged:\n%s", log[len(logBefore):])
	}
	if !strings.Contains(string(keRnRead(t, s, "knowledge/decisions/d.md")), "superseded_by: brain:project/b") {
		t.Fatal("d is not relinked")
	}
	if _, err := os.Stat(keRnPath(s, "knowledge/decisions/b.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keRnPath(s, "knowledge/decisions/a.md")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a.md is still there: %v", err)
	}
}

func TestKeRnRace(t *testing.T) {
	s := keRnStore(t)
	keRnAccept(t, s, keRnNote("decisions", "a"))
	rev := keRnRev(t, s, "decisions", "a")
	before := keRnRead(t, s, "knowledge/decisions/a.md")
	start := make(chan struct{})
	var wg sync.WaitGroup
	var renameErr, dropErr error
	renameRoot, err := s.scope("project", false)
	if err != nil {
		t.Fatal(err)
	}
	defer renameRoot.Close()
	dropRoot, err := s.scope("project", false)
	if err != nil {
		t.Fatal(err)
	}
	defer dropRoot.Close()
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, renameErr = RenameWith(renameRoot, "project", "decisions", "a", "b", rev, keRnAna, "")
	}()
	go func() {
		defer wg.Done()
		<-start
		_, dropErr = DropWith(dropRoot, "decisions", "a", OpDrop, rev, keRnAna, "")
	}()
	close(start)
	wg.Wait()
	log := keRnLog(t, s)
	_, bErr := os.Stat(keRnPath(s, "knowledge/decisions/b.md"))
	switch {
	case renameErr == nil && dropErr != nil:
		if !errors.Is(dropErr, ErrNoSuchNote) || strings.Contains(log, "| dropped |") {
			t.Fatalf("rename won, drop: %v, log:\n%s", dropErr, log)
		}
		if bErr != nil {
			t.Fatalf("b is missing: %v", bErr)
		}
		if _, err := os.Stat(keRnPath(s, "knowledge/decisions/a.md")); !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("a.md is still there: %v", err)
		}
	case dropErr == nil && renameErr != nil:
		if !keRnRefused(renameErr) {
			t.Fatalf("drop won, rename: %v", renameErr)
		}
		if !bytes.Equal(keRnRead(t, s, "knowledge/decisions/a.md"), before) || !errors.Is(bErr, fs.ErrNotExist) {
			t.Fatalf("a changed or b exists: %v", bErr)
		}
		if strings.Contains(log, "| renamed |") {
			t.Fatalf("a renamed line after a drop:\n%s", log)
		}
	default:
		t.Fatalf("rename: %v, drop: %v; want exactly one to succeed", renameErr, dropErr)
	}
}
