package brain

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func readLog(t *testing.T, s Store) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", "log.md"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return string(b)
}

func versionsOf(t *testing.T, s Store, topic, name string) []Version {
	t.Helper()
	v, err := s.Versions("project", topic, name)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// wantChain checks that versions are 1..n, that each stored file hashes to its
// reported hash, and that the newest copy is byte for byte the published file.
func wantChain(t *testing.T, s Store, topic, name string, n int, published []byte) []Version {
	t.Helper()
	got := versionsOf(t, s, topic, name)
	if len(got) != n {
		t.Fatalf("%s/%s has %d versions, want %d: %+v", topic, name, len(got), n, got)
	}
	for i, v := range got {
		if v.N != i+1 {
			t.Fatalf("versions are not gap free: %+v", got)
		}
		b, err := os.ReadFile(filepath.Join(s.GridHome, filepath.FromSlash(v.Path)))
		if err != nil || auditHash(b) != v.Hash {
			t.Fatalf("version %d does not match its hash: %v", v.N, err)
		}
	}
	if got[n-1].Hash != auditHash(published) {
		t.Fatalf("newest version is not the published entry")
	}
	return got
}

func published(t *testing.T, s Store, topic, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.GridHome, "projects", "example", "knowledge", topic, name+".md"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func wantLine(t *testing.T, s Store, parts ...string) {
	t.Helper()
	log := readLog(t, s)
	if want := strings.Join(parts, " | "); strings.Count(log, want) != 1 {
		t.Fatalf("log should hold %q once:\n%s", want, log)
	}
}

func taskWithProposal(t *testing.T, dir string, n Note) TaskRef {
	t.Helper()
	b, err := Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "brain-proposals", n.Name+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return TaskRef{ID: "T-01", Path: dir}
}

func extra(name string) Note {
	n := sample()
	n.Name, n.Links, n.Body = name, nil, "More about "+name+".\n"
	return n
}

func TestAcceptKeepsVersionAndAuditHash(t *testing.T) {
	s := testStore(t)
	n := sample()
	accepted(t, s, n)
	v := wantChain(t, s, "conventions", "checks", 1, published(t, s, "conventions", "checks"))
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "accepted", "checks", "job:T-01/J-01", "", "conventions", "v1", v[0].Hash+"\n")

	// A repeated accept is the same decision: neither a version nor a line.
	if _, err := s.Accept("project", n.Name); err != nil {
		t.Fatal(err)
	}
	wantChain(t, s, "conventions", "checks", 1, published(t, s, "conventions", "checks"))
	if strings.Count(readLog(t, s), "| accepted |") != 1 {
		t.Fatalf("retry added a line:\n%s", readLog(t, s))
	}

	// A rejection publishes nothing: version 0, no hash.
	r := extra("unneeded")
	if _, err := s.Propose(r, r.Origin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reject("project", r.Name, "Already covered."); err != nil {
		t.Fatal(err)
	}
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "rejected", "unneeded", "job:T-01/J-01", "Already covered.", "conventions", "v0", "\n")
	if got := versionsOf(t, s, "conventions", "unneeded"); len(got) != 0 {
		t.Fatalf("rejection stored a version: %+v", got)
	}
}

func TestMergeIntoKeepsNextVersion(t *testing.T) {
	s := testStore(t)
	accepted(t, s, sample())
	dir := t.TempDir()
	noop := func(string) error { return nil }
	decide := func(proposal string) {
		t.Helper()
		ref := taskWithProposal(t, dir, extra(proposal))
		if _, err := s.DecideTask(ref, proposal, "merge-into", "brain:project/checks", "", noop); err != nil {
			t.Fatal(err)
		}
	}
	decide("second")
	v := wantChain(t, s, "conventions", "checks", 2, published(t, s, "conventions", "checks"))
	// The line names the entry that was changed, not the merged proposal.
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "accepted", "checks", "job:T-01/J-01", "merge-into brain:project/checks", "conventions", "v2", v[1].Hash+"\n")
	decide("third")
	wantChain(t, s, "conventions", "checks", 3, published(t, s, "conventions", "checks"))

	// An interrupted decision is finished by a retry that adds nothing.
	ref := taskWithProposal(t, dir, extra("fourth"))
	if _, err := s.DecideTask(ref, "fourth", "merge-into", "brain:project/checks", "", func(string) error { return fmt.Errorf("history is down") }); err == nil {
		t.Fatal("history failure was hidden")
	}
	before := readLog(t, s)
	wantChain(t, s, "conventions", "checks", 4, published(t, s, "conventions", "checks"))
	if _, err := s.DecideTask(ref, "fourth", "merge-into", "brain:project/checks", "", noop); err != nil {
		t.Fatal(err)
	}
	wantChain(t, s, "conventions", "checks", 4, published(t, s, "conventions", "checks"))
	if readLog(t, s) != before {
		t.Fatalf("retry changed the log:\n%s", readLog(t, s))
	}
}

func TestDecideTaskAcceptNamesThePublishedEntry(t *testing.T) {
	s := testStore(t)
	ref := taskWithProposal(t, t.TempDir(), extra("draft"))
	if _, err := s.DecideTask(ref, "draft", "accept", "brain:project/final", "", func(string) error { return nil }); err != nil {
		t.Fatal(err)
	}
	v := wantChain(t, s, "conventions", "final", 1, published(t, s, "conventions", "final"))
	wantLine(t, s, "- 2026-01-02T03:04:05.000Z", "accepted", "final", "job:T-01/J-01", "accept brain:project/final", "conventions", "v1", v[0].Hash+"\n")
}

func TestConfigEditKeepsVersionAndAuditHash(t *testing.T) {
	s := testStore(t)
	edit := func(text string) {
		t.Helper()
		if err := s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) {
			return EncodeConfig(map[string]string{"name": "daily"}, text)
		}); err != nil {
			t.Fatal(err)
		}
	}
	read := func() []byte {
		t.Helper()
		b, err := s.ReadConfig("project", "routines", "daily")
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	edit("First.")
	v1 := wantChain(t, s, "routines", "daily", 1, read())
	edit("Second.")
	v2 := wantChain(t, s, "routines", "daily", 2, read())
	log := readLog(t, s)
	for i, v := range []Version{v1[0], v2[1]} {
		if !strings.Contains(log, "| accepted | daily | config:") || !strings.Contains(log, fmt.Sprintf("| routines | v%d | %s\n", i+1, v.Hash)) {
			t.Fatalf("log misses version %d:\n%s", i+1, log)
		}
	}
	changes, err := s.Changes("project", time.Time{}, 0)
	if err != nil || len(changes) != 2 || changes[0].Version != 2 || changes[0].OriginKind != "config" || changes[0].OriginID == "" || changes[0].OriginID == changes[1].OriginID {
		t.Fatalf("config changes: %+v %v", changes, err)
	}

	// Writing the same bytes again is no change.
	edit("Second.")
	wantChain(t, s, "routines", "daily", 2, read())
	if got := readLog(t, s); got != log {
		t.Fatalf("identical edit added a line:\n%s", got)
	}
	// A callback error changes nothing.
	if err := s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) { return nil, os.ErrInvalid }); err == nil {
		t.Fatal("callback error hidden")
	}
	wantChain(t, s, "routines", "daily", 2, read())
	// Removal is not a write of new bytes: the history stays, no line is added.
	if err := s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	if got := versionsOf(t, s, "routines", "daily"); len(got) != 2 || readLog(t, s) != log {
		t.Fatalf("removal changed history: %+v", got)
	}
}

func TestSymlinkedVersionsFolderIsRefused(t *testing.T) {
	s := testStore(t)
	accepted(t, s, sample())
	versions := filepath.Join(s.GridHome, "projects", "example", "knowledge", versionsTopic)
	elsewhere := t.TempDir()
	if err := os.RemoveAll(versions); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, versions); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	n := extra("other")
	if _, err := s.Propose(n, n.Origin); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept("project", n.Name); err == nil {
		t.Fatal("accept followed a symlinked versions folder")
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "other.md")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("note was published without a version: %v", err)
	}
	err := s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) { return []byte("x"), nil })
	if err == nil {
		t.Fatal("config edit followed a symlinked versions folder")
	}
	if _, err := s.ReadConfig("project", "routines", "daily"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("config entry was published without a version: %v", err)
	}
	if _, err := s.Versions("project", "conventions", "checks"); err == nil {
		t.Fatal("Versions followed a symlinked folder")
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Fatalf("wrote through the symlink: %v", entries)
	}
}

func TestVersionsIsNotATopic(t *testing.T) {
	s := testStore(t)
	accepted(t, s, sample())
	topics, err := s.Topics()
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range topics {
		if topic.Name == versionsTopic {
			t.Fatalf("stored versions listed as a topic: %+v", topic)
		}
	}
	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range idx.Topics {
		if row.Topic == versionsTopic || row.Invalid != 0 {
			t.Fatalf("stored versions disturbed the index: %+v", row)
		}
	}
	// No note can claim the folder as its topic either.
	putFile(t, s, "projects/example/knowledge/versions/TOPIC.md", "---\nname: versions\nkind: knowledge\nlevel: project\n---\nHistory.\n")
	n := sample()
	n.Name, n.Topic = "claim", versionsTopic
	if _, err := s.Propose(n, n.Origin); err == nil {
		t.Fatal("versions accepted as a topic")
	}
}

func TestRecordVersionNumbering(t *testing.T) {
	s := testStore(t)
	root, err := s.scope("project", true)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	dir := versionDir("conventions", "checks")
	for _, name := range []string{"2.md", "3.md", "notes.md", "007.md", "4.txt", ".brain-x.tmp"} {
		if err := atomicWrite(root, filepath.Join(dir, name), []byte(name)); err != nil {
			t.Fatal(err)
		}
	}
	n, hash, err := recordVersion(root, "conventions", "checks", []byte("3.md"))
	if err != nil || n != 3 || hash != auditHash([]byte("3.md")) {
		t.Fatalf("identical bytes should reuse the newest version: %d %q %v", n, hash, err)
	}
	n, _, err = recordVersion(root, "conventions", "checks", []byte("new"))
	if err != nil || n != 4 {
		t.Fatalf("new bytes should take the next number: %d %v", n, err)
	}
	// The older copy is not the newest: going back to it is a new version.
	n, _, err = recordVersion(root, "conventions", "checks", []byte("3.md"))
	if err != nil || n != 5 {
		t.Fatalf("returning to old bytes should add a version: %d %v", n, err)
	}
}

func TestConcurrentWritersKeepVersionsGapFree(t *testing.T) {
	s := testStore(t)
	const writers = 8
	var wg sync.WaitGroup
	errs := make(chan error, 2*writers)
	for i := range writers {
		wg.Go(func() {
			errs <- s.EditConfig("project", "routines", "daily", func([]byte) ([]byte, error) {
				return EncodeConfig(map[string]string{"name": "daily"}, fmt.Sprintf("Edit %d.", i))
			})
		})
		wg.Go(func() {
			n := extra(fmt.Sprintf("note-%d", i))
			if _, err := s.Propose(n, n.Origin); err != nil {
				errs <- err
				return
			}
			_, err := s.Accept("project", n.Name)
			errs <- err
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	b, err := s.ReadConfig("project", "routines", "daily")
	if err != nil {
		t.Fatal(err)
	}
	wantChain(t, s, "routines", "daily", writers, b)
	for i := range writers {
		name := fmt.Sprintf("note-%d", i)
		wantChain(t, s, "conventions", name, 1, published(t, s, "conventions", name))
	}
	if got := strings.Count(readLog(t, s), "\n- "); got != 2*writers {
		t.Fatalf("log has %d lines, want %d:\n%s", got, 2*writers, readLog(t, s))
	}
}
