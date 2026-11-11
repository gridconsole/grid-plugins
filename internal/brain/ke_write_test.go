package brain

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
)

var keWrSession = Origin{Kind: "session", ID: "s1"}

func keWrStore(t *testing.T) Store {
	t.Helper()
	return Store{GridHome: filepath.Join(t.TempDir(), ".grid")}
}

// keWrNote is a valid accepted home-level note as the caller would send it.
func keWrNote(topic, name, body string, mods ...func(*Note)) []byte {
	n := Note{Name: name, Topic: topic, Type: "reference", Description: "Read before changing the " + name + ".",
		Summary: "What the " + name + " says.", Source: "person:ana", Level: "home", Status: "accepted",
		Origin: Origin{Kind: "session", ID: "s0"}, Body: body}
	for _, m := range mods {
		m(&n)
	}
	b, err := Encode(n)
	if err != nil {
		panic(err)
	}
	return b
}

// keWrSnapshot records every folder, file (mode and bytes) and link below the
// roots, so two snapshots can be compared byte for byte.
func keWrSnapshot(t *testing.T, roots ...string) map[string]string {
	t.Helper()
	out := map[string]string{}
	for i, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, p)
			key := fmt.Sprintf("%d:%s", i, rel)
			switch {
			case d.Type()&os.ModeSymlink != 0:
				target, err := os.Readlink(p)
				if err != nil {
					return err
				}
				out[key] = "link " + target
			case d.IsDir():
				out[key] = "dir"
			default:
				b, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				info, err := d.Info()
				if err != nil {
					return err
				}
				out[key] = fmt.Sprintf("file %o %q", info.Mode().Perm(), b)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func keWrSame(t *testing.T, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if got, ok := after[k]; !ok {
			t.Errorf("%s was removed", k)
		} else if got != v {
			t.Errorf("%s changed:\n before %s\n after  %s", k, v, got)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s was added", k)
		}
	}
}

func keWrRead(t *testing.T, s Store, rel string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.GridHome, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func keWrLogLines(t *testing.T, s Store) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.GridHome, "knowledge", "log.md"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "- ") {
			lines = append(lines, l)
		}
	}
	return lines
}

func keWrVersions(t *testing.T, s Store, topic, name string) []int {
	t.Helper()
	root, err := os.OpenRoot(s.GridHome)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	numbers, err := versionNumbers(root, versionDir(topic, name))
	if err != nil {
		t.Fatal(err)
	}
	return numbers
}

func keWrMust(t *testing.T, s Store, topic, name, expect string, data []byte, allowWorkflow bool) WriteResult {
	t.Helper()
	res, err := s.Write("home", topic, name, expect, data, keWrSession, "", allowWorkflow)
	if err != nil {
		t.Fatalf("write %s/%s: %v", topic, name, err)
	}
	return res
}

func TestKeWrStore(t *testing.T) {
	s := keWrStore(t)
	first := keWrNote("decisions", "x", "# X\n\nThe zebra rule.\n")
	res, err := s.Write("home", "decisions", "x", "none", first, keWrSession, "first draft", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "created" || res.Version != 1 || res.Rev != NoteRev(first) {
		t.Fatalf("new note result: %+v", res)
	}
	if got := keWrRead(t, s, "knowledge/decisions/x.md"); string(got) != string(first) {
		t.Fatalf("note on disk:\n%s\nwant:\n%s", got, first)
	}
	if got := keWrRead(t, s, "knowledge/versions/decisions/x/1.md"); string(got) != string(first) {
		t.Fatalf("version 1:\n%s", got)
	}
	lines := keWrLogLines(t, s)
	if len(lines) != 1 {
		t.Fatalf("log lines: %q", lines)
	}
	created := regexp.MustCompile(`^- \S+ \| created \| x \| session:s1 \| first draft \| decisions \| v1 \| sha256:[0-9a-f]{16}$`)
	if !created.MatchString(lines[0]) || !strings.HasSuffix(lines[0], "sha256:"+res.Rev) {
		t.Fatalf("created line: %q", lines[0])
	}

	second := keWrNote("decisions", "x", "# X\n\nThe quokka rule.\n")
	res, err = s.Write("home", "decisions", "x", NoteRev(first), second, Origin{Kind: "person", ID: "bo"}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != "edited" || res.Version != 2 || res.Rev != NoteRev(second) {
		t.Fatalf("replace result: %+v", res)
	}
	if got := keWrRead(t, s, "knowledge/decisions/x.md"); string(got) != string(second) {
		t.Fatalf("note after replace:\n%s", got)
	}
	if got := keWrRead(t, s, "knowledge/versions/decisions/x/2.md"); string(got) != string(second) {
		t.Fatalf("version 2:\n%s", got)
	}
	if got := keWrRead(t, s, "knowledge/versions/decisions/x/1.md"); string(got) != string(first) {
		t.Fatalf("version 1 changed:\n%s", got)
	}
	lines = keWrLogLines(t, s)
	if len(lines) != 2 || !strings.Contains(lines[1], "| edited | x | person:bo |") || !strings.Contains(lines[1], "| decisions | v2 | sha256:") {
		t.Fatalf("log after replace: %q", lines)
	}
	// The index is rebuilt: search sees the new body and no longer the old one.
	if hits, err := s.Search("quokka", "", 5); err != nil || len(hits) != 1 || hits[0].ID != "brain:home/x" {
		t.Fatalf("search quokka: %+v %v", hits, err)
	}
	if hits, err := s.Search("zebra", "", 5); err != nil || len(hits) != 0 {
		t.Fatalf("search zebra: %+v %v", hits, err)
	}
	// The note is what scan lists, so every reader sees it.
	if idx, err := s.Index(); err != nil {
		t.Fatal(err)
	} else {
		for _, row := range idx.Topics {
			if row.Topic == "decisions" && (row.Entries != 1 || row.Invalid != 0) {
				t.Fatalf("index row: %+v", row)
			}
		}
	}
	// An owner may write a workflow note; it is a note like any other.
	wf := keWrNote("decisions", "flow", "# Flow\n", func(n *Note) { n.Type = "workflow" })
	if res := keWrMust(t, s, "decisions", "flow", "none", wf, true); res.Status != "created" || res.Version != 1 {
		t.Fatalf("workflow note by an owner: %+v", res)
	}
}

func TestKeWrRefuse(t *testing.T) {
	s := keWrStore(t)
	x := keWrNote("decisions", "x", "# X\n")
	keWrMust(t, s, "decisions", "x", "none", x, false)
	keWrMust(t, s, "decisions", "seed", "none", keWrNote("decisions", "seed", "# Seed\n"), false)
	flow := keWrNote("decisions", "flow", "# Flow\n", func(n *Note) { n.Type = "workflow" })
	keWrMust(t, s, "decisions", "flow", "none", flow, true)
	// A note placed by hand has no stored version.
	hand := keWrNote("decisions", "y", "# Y\n")
	if err := os.WriteFile(filepath.Join(s.GridHome, "knowledge", "decisions", "y.md"), hand, 0o600); err != nil {
		t.Fatal(err)
	}
	// A link to a file and a link to a folder, both outside the brain.
	outside := t.TempDir()
	target := filepath.Join(outside, "target.md")
	if err := os.WriteFile(target, hand, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(s.GridHome, "knowledge", "decisions", "z.md")); err != nil {
		t.Fatal(err)
	}
	linkedDir := filepath.Join(outside, "folder")
	if err := os.Mkdir(linkedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(linkedDir, filepath.Join(s.GridHome, "knowledge", "linked")); err != nil {
		t.Fatal(err)
	}
	// "linked" and "notes" are custom knowledge topics at home level.
	for _, topic := range []string{"linked", "notes"} {
		if err := os.MkdirAll(filepath.Join(s.GridHome, "topics", topic), 0o700); err != nil {
			t.Fatal(err)
		}
		def := "---\nname: " + topic + "\nkind: knowledge\nlevel: home\ndescription: A custom topic.\n---\n# " + topic + "\n"
		if err := os.WriteFile(filepath.Join(s.GridHome, "topics", topic, "TOPIC.md"), []byte(def), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	stale := "0000000000000000"
	cases := []struct {
		name          string
		topic, nm     string
		expect        string
		data          []byte
		allowWorkflow bool
		check         func(t *testing.T, err error)
	}{
		{name: "stale rev", topic: "decisions", nm: "x", expect: stale, data: keWrNote("decisions", "x", "# New\n"), check: func(t *testing.T, err error) {
			var c *ConflictError
			if !errors.As(err, &c) || !errors.Is(err, ErrConflict) || c.Rev != NoteRev(x) {
				t.Fatalf("err = %v, want a conflict at %s", err, NoteRev(x))
			}
		}},
		{name: "none on an existing note", topic: "decisions", nm: "x", expect: "none", data: keWrNote("decisions", "x", "# New\n"), check: func(t *testing.T, err error) {
			var c *ConflictError
			if !errors.As(err, &c) || c.Rev != NoteRev(x) {
				t.Fatalf("err = %v, want a conflict at %s", err, NoteRev(x))
			}
		}},
		{name: "a rev on a missing note", topic: "decisions", nm: "gone", expect: NoteRev(x), data: keWrNote("decisions", "gone", "# G\n"), check: func(t *testing.T, err error) {
			var c *ConflictError
			if !errors.As(err, &c) || c.Rev != "none" {
				t.Fatalf("err = %v, want a conflict at none", err)
			}
		}},
		{name: "config topic", topic: "machines", nm: "x", expect: "none", data: keWrNote("machines", "x", "# M\n"), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrNotVersioned) {
				t.Fatalf("err = %v, want ErrNotVersioned", err)
			}
		}},
		{name: "topic with no definition", topic: "nothere", nm: "x", expect: "none", data: keWrNote("nothere", "x", "# M\n"), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrNotVersioned) {
				t.Fatalf("err = %v, want ErrNotVersioned", err)
			}
		}},
		{name: "hand-placed note", topic: "decisions", nm: "y", expect: NoteRev(hand), data: keWrNote("decisions", "y", "# Y2\n"), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrNotVersioned) {
				t.Fatalf("err = %v, want ErrNotVersioned", err)
			}
		}},
		{name: "linked topic folder", topic: "linked", nm: "x", expect: "none", data: keWrNote("linked", "x", "# L\n"), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrLinkedTarget) || !strings.Contains(err.Error(), "linked or non-regular target") {
				t.Fatalf("err = %v, want a linked target", err)
			}
		}},
		{name: "linked note file", topic: "decisions", nm: "z", expect: NoteRev(hand), data: keWrNote("decisions", "z", "# Z\n"), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrLinkedTarget) || !strings.Contains(err.Error(), "linked or non-regular target") {
				t.Fatalf("err = %v, want a linked target", err)
			}
		}},
		{name: "member replaces a workflow note", topic: "decisions", nm: "flow", expect: NoteRev(flow), data: keWrNote("decisions", "flow", "# Plain\n"), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrWorkflowNote) {
				t.Fatalf("err = %v, want ErrWorkflowNote", err)
			}
		}},
		{name: "member sends workflow text", topic: "decisions", nm: "fresh", expect: "none", data: keWrNote("decisions", "fresh", "# F\n", func(n *Note) { n.Type = "Workflow " }), check: func(t *testing.T, err error) {
			if !errors.Is(err, ErrWorkflowNote) {
				t.Fatalf("err = %v, want ErrWorkflowNote", err)
			}
		}},
		{name: "status proposed", topic: "decisions", nm: "p", expect: "none", data: keWrNote("decisions", "p", "# P\n", func(n *Note) { n.Status = "proposed" }), check: keWrInvalidCheck},
		{name: "wrong topic in the bytes", topic: "decisions", nm: "w", expect: "none", data: keWrNote("conventions", "w", "# W\n"), check: keWrInvalidCheck},
		{name: "wrong name in the bytes", topic: "decisions", nm: "w", expect: "none", data: keWrNote("decisions", "other", "# W\n"), check: keWrInvalidCheck},
		{name: "wrong level in the bytes", topic: "decisions", nm: "w", expect: "none", data: keWrNote("decisions", "w", "# W\n", func(n *Note) { n.Level = "project" }), check: keWrInvalidCheck},
		{name: "secret", topic: "decisions", nm: "sec", expect: "none", data: keWrNote("decisions", "sec", "password: hunter2hunter2\n"), check: keWrInvalidCheck},
		{name: "import origin", topic: "decisions", nm: "imp", expect: "none", data: keWrNote("decisions", "imp", "# I\n", func(n *Note) { n.Origin.Kind = "import" }), check: keWrInvalidCheck},
		{name: "not a note", topic: "decisions", nm: "junk", expect: "none", data: []byte("just text\n"), check: keWrInvalidCheck},
		{name: "larger than a note may be", topic: "decisions", nm: "big", expect: "none", data: keWrNote("decisions", "big", strings.Repeat("a ", 600<<10)), check: keWrInvalidCheck},
		{name: "name held by another topic", topic: "notes", nm: "x", expect: "none", data: keWrNote("notes", "x", "# X again\n"), check: func(t *testing.T, err error) {
			var r *RefusedError
			if !errors.As(err, &r) || !errors.Is(err, ErrDropRefused) || !strings.Contains(r.Reason, "decisions") {
				t.Fatalf("err = %v, want a refusal naming topic decisions", err)
			}
		}},
		{name: "TOPIC is not an entry", topic: "decisions", nm: "TOPIC", expect: "none", data: keWrNote("decisions", "TOPIC", "# T\n"), check: func(t *testing.T, err error) {
			var r *RefusedError
			if !errors.As(err, &r) {
				t.Fatalf("err = %v, want a refusal", err)
			}
		}},
		{name: "bad expect", topic: "decisions", nm: "x", expect: "ABC", data: x, check: func(t *testing.T, err error) {
			if err == nil {
				t.Fatal("no error")
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			before := keWrSnapshot(t, s.GridHome, outside)
			_, err := s.Write("home", c.topic, c.nm, c.expect, c.data, keWrSession, "nope", c.allowWorkflow)
			c.check(t, err)
			keWrSame(t, before, keWrSnapshot(t, s.GridHome, outside))
		})
	}
}

func keWrInvalidCheck(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidNote) {
		t.Fatalf("err = %v, want ErrInvalidNote", err)
	}
}

// TestKeWrRace: two writers at one rev, released together by one closed channel.
func TestKeWrRace(t *testing.T) {
	s := keWrStore(t)
	for round := range 12 {
		name := fmt.Sprintf("x%d", round)
		seed := keWrNote("decisions", name, "# Seed\n")
		keWrMust(t, s, "decisions", name, "none", seed, false)
		versions, logs := len(keWrVersions(t, s, "decisions", name)), len(keWrLogLines(t, s))
		inputs := [2][]byte{keWrNote("decisions", name, "# A\n"), keWrNote("decisions", name, "# B\n")}
		var errs [2]error
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := range inputs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = s.Write("home", "decisions", name, NoteRev(seed), inputs[i], Origin{Kind: "session", ID: fmt.Sprintf("w%d", i)}, "", false)
			}()
		}
		close(start)
		wg.Wait()
		winner := -1
		for i, err := range errs {
			var c *ConflictError
			switch {
			case err == nil && winner < 0:
				winner = i
			case err == nil:
				t.Fatalf("round %d: both writers succeeded", round)
			case !errors.As(err, &c):
				t.Fatalf("round %d: writer %d: %v, want a conflict", round, i, err)
			}
		}
		if winner < 0 {
			t.Fatalf("round %d: no writer succeeded: %v", round, errs)
		}
		if got := keWrRead(t, s, "knowledge/decisions/"+name+".md"); string(got) != string(inputs[winner]) {
			t.Fatalf("round %d: file is not the winner's bytes:\n%s", round, got)
		}
		if got := len(keWrVersions(t, s, "decisions", name)); got != versions+1 {
			t.Fatalf("round %d: versions %d, want %d", round, got, versions+1)
		}
		if got := len(keWrLogLines(t, s)); got != logs+1 {
			t.Fatalf("round %d: log lines %d, want %d", round, got, logs+1)
		}
	}
}
