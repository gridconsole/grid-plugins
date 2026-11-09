package brain

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var keeper = Origin{Kind: "session", ID: "keeper-1", Tier: "agent"}

// threeVersions publishes v1, v2 and v3 of conventions/checks.
func threeVersions(t *testing.T) Store {
	t.Helper()
	s := testStore(t)
	accepted(t, s, sample())
	dir := t.TempDir()
	for _, proposal := range []string{"second", "third"} {
		ref := taskWithProposal(t, dir, extra(proposal))
		if _, err := s.DecideTask(ref, proposal, "merge-into", "brain:project/checks", "", func(string) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	wantChain(t, s, "conventions", "checks", 3, published(t, s, "conventions", "checks"))
	return s
}

func versionBytes(t *testing.T, s Store, n int) []byte {
	t.Helper()
	for _, v := range versionsOf(t, s, "conventions", "checks") {
		if v.N == n {
			b, err := os.ReadFile(filepath.Join(s.GridHome, filepath.FromSlash(v.Path)))
			if err != nil {
				t.Fatal(err)
			}
			return b
		}
	}
	t.Fatalf("no version %d", n)
	return nil
}

// snapshot reads every file below the Grid home, so "changes nothing" covers
// versions, entries, the log, the index and stray temporary files alike.
func snapshot(t *testing.T, s Store) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(s.GridHome, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		files[path] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func wantSame(t *testing.T, s Store, before map[string]string) {
	t.Helper()
	after := snapshot(t, s)
	for path, b := range before {
		if got, ok := after[path]; !ok || got != b {
			t.Fatalf("%s changed", path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			t.Fatalf("%s was created", path)
		}
	}
}

func TestRevertPublishesTheOldBytesAsANewVersion(t *testing.T) {
	s := threeVersions(t)
	v1 := versionBytes(t, s, 1)
	if bytes.Equal(v1, published(t, s, "conventions", "checks")) {
		t.Fatal("test needs different versions")
	}
	if _, err := s.RebuildIndex(); err != nil {
		t.Fatal(err)
	}
	before, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}

	entry, n, err := s.Revert("project", "conventions", "checks", 1, keeper)
	if err != nil || n != 4 {
		t.Fatalf("revert: v%d, %v", n, err)
	}
	if entry.ID != "brain:project/checks" || entry.Note.Topic != "conventions" || entry.Note.Status != "accepted" {
		t.Fatalf("entry: %+v", entry)
	}
	// Nothing was rewritten: v1 to v3 stay, v4 is a copy of v1, and the entry is v1.
	got := wantChain(t, s, "conventions", "checks", 4, v1)
	if !bytes.Equal(published(t, s, "conventions", "checks"), v1) || !bytes.Equal(versionBytes(t, s, 4), v1) {
		t.Fatal("the published entry is not version 1")
	}
	if got[3].Hash != got[0].Hash {
		t.Fatalf("v4 should hold v1's bytes: %+v", got)
	}
	changes, err := s.Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	last := changes[0]
	if last.Status != "reverted" || last.Version != 4 || last.Reason != "revert to v1" || last.Hash != got[3].Hash ||
		last.Topic != "conventions" || last.Name != "checks" || last.OriginKind != "session" || last.OriginID != "keeper-1" {
		t.Fatalf("newest change: %+v", last)
	}
	if strings.Count(readLog(t, s), "| reverted |") != 1 {
		t.Fatalf("log:\n%s", readLog(t, s))
	}
	// The generated index was rebuilt after the publication.
	after, err := s.Index()
	if err != nil || after.Generations["project"] != before.Generations["project"]+1 {
		t.Fatalf("index generations %v then %v, %v", before.Generations, after.Generations, err)
	}
}

func TestRevertToTheCurrentBytesWritesNothing(t *testing.T) {
	s := threeVersions(t)
	before := snapshot(t, s)
	entry, n, err := s.Revert("project", "conventions", "checks", 3, keeper)
	if err != nil || n != 3 || entry.Note.Name != "checks" {
		t.Fatalf("revert to the current version: v%d, %v", n, err)
	}
	wantSame(t, s, before)

	// Version 4 holds version 1's bytes, so asking for 1 or 4 is still a no-op.
	if _, n, err = s.Revert("project", "conventions", "checks", 1, keeper); err != nil || n != 4 {
		t.Fatalf("revert to v1: v%d, %v", n, err)
	}
	after := snapshot(t, s)
	for _, to := range []int{1, 4} {
		if _, n, err = s.Revert("project", "conventions", "checks", to, keeper); err != nil || n != 4 {
			t.Fatalf("revert to v%d again: v%d, %v", to, n, err)
		}
		wantSame(t, s, after)
	}
}

func TestRevertRefusesWithoutChangingAnything(t *testing.T) {
	s := threeVersions(t)
	before := snapshot(t, s)
	for _, c := range []struct {
		topic, name string
		to          int
	}{
		{"conventions", "checks", 9}, // no such version
		{"conventions", "checks", 0}, // versions start at 1
		{"conventions", "checks", -1},
		{"conventions", "unknown", 1},   // no such entry
		{"decisions", "checks", 1},      // entry lives in another topic
		{"../conventions", "checks", 1}, // not a plain name
		{"conventions", "../checks", 1},
		{"conventions", "a/b", 1},
		{"", "checks", 1},
		{"conventions", "", 1},
		{"versions", "conventions", 1},  // the reserved folder
		{"conventions", "checks.md", 1}, // a file name, not an entry name
		{"conventions", "CHECKS", 1},    // names are lower case
		{"conventions", "checks\n", 1},
		{"conventions", "checks", 1 << 30}, // far past the history
	} {
		if _, _, err := s.Revert("project", c.topic, c.name, c.to, keeper); err == nil {
			t.Errorf("revert %q/%q to %d was accepted", c.topic, c.name, c.to)
		}
	}
	if _, _, err := s.Revert("other", "conventions", "checks", 1, keeper); err == nil {
		t.Error("an unknown level was accepted")
	}
	wantSame(t, s, before)

	// A level without a Grid folder is the same refusal.
	empty := Store{GridHome: filepath.Join(t.TempDir(), ".grid"), Project: "example"}
	if _, _, err := empty.Revert("project", "conventions", "checks", 1, keeper); err == nil {
		t.Error("revert in an empty home was accepted")
	}
}

func TestRevertRefusesConfigTopicsAndDamagedCopies(t *testing.T) {
	s := threeVersions(t)
	valid := string(versionBytes(t, s, 1))
	for _, c := range []struct {
		name, path, data string
		topic, entry     string
		to               int
	}{
		{"config topic", "knowledge/versions/routines/daily/1.md", "---\nname: daily\n---\n", "routines", "daily", 1},
		{"not a note", "knowledge/versions/conventions/checks/4.md", "nope\n", "conventions", "checks", 4},
		{"other name", "knowledge/versions/conventions/checks/4.md", strings.Replace(valid, "name: checks", "name: other", 1), "conventions", "checks", 4},
		{"other topic", "knowledge/versions/conventions/checks/4.md", strings.Replace(valid, "topic: conventions", "topic: decisions", 1), "conventions", "checks", 4},
		{"other level", "knowledge/versions/conventions/checks/4.md", strings.Replace(valid, "level: project", "level: home", 1), "conventions", "checks", 4},
	} {
		t.Run(c.name, func(t *testing.T) {
			putFile(t, s, "projects/example/"+c.path, c.data)
			before := snapshot(t, s)
			if _, _, err := s.Revert("project", c.topic, c.entry, c.to, keeper); err == nil {
				t.Fatal("revert accepted it")
			}
			wantSame(t, s, before)
			if err := os.Remove(filepath.Join(s.GridHome, "projects", "example", filepath.FromSlash(c.path))); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRevertRefusesANameKeptByAnotherTopic(t *testing.T) {
	s := threeVersions(t)
	// A hand copy of the name in another topic would make "checks" ambiguous.
	other := strings.Replace(string(versionBytes(t, s, 1)), "topic: conventions", "topic: decisions", 1)
	putFile(t, s, "projects/example/knowledge/decisions/checks.md", other)
	before := snapshot(t, s)
	if _, _, err := s.Revert("project", "conventions", "checks", 1, keeper); err == nil {
		t.Fatal("revert published a name that another topic holds")
	}
	wantSame(t, s, before)
}

func TestRevertRestoresAMissingEntryFromItsNewestCopy(t *testing.T) {
	s := threeVersions(t)
	path := filepath.Join(s.GridHome, "projects", "example", "knowledge", "conventions", "checks.md")
	v3 := versionBytes(t, s, 3)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, n, err := s.Revert("project", "conventions", "checks", 3, keeper); err != nil || n != 3 {
		t.Fatalf("revert: v%d, %v", n, err)
	}
	// The newest copy already holds these bytes, so no version is added.
	wantChain(t, s, "conventions", "checks", 3, v3)
	if !bytes.Equal(published(t, s, "conventions", "checks"), v3) || !strings.Contains(readLog(t, s), "| reverted | checks |") {
		t.Fatalf("entry not restored:\n%s", readLog(t, s))
	}
}

func TestRevertFinishesAnInterruptedPublication(t *testing.T) {
	s := threeVersions(t)
	// Version 3 was stored but the entry still shows version 2.
	v2 := versionBytes(t, s, 2)
	putFile(t, s, "projects/example/knowledge/conventions/checks.md", string(v2))
	if _, n, err := s.Revert("project", "conventions", "checks", 2, keeper); err != nil || n != 4 {
		t.Fatalf("revert: v%d, %v", n, err)
	}
	wantChain(t, s, "conventions", "checks", 4, v2)
	if !bytes.Equal(versionBytes(t, s, 4), v2) {
		t.Fatal("v4 should hold version 2's bytes")
	}
}

func TestConcurrentRevertsKeepVersionsGapFree(t *testing.T) {
	s := threeVersions(t)
	v1 := versionBytes(t, s, 1)
	var wg sync.WaitGroup
	got := make([]int, 16)
	errs := make([]error, 16)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, got[i], errs[i] = s.Revert("project", "conventions", "checks", 1, keeper)
		}()
	}
	wg.Wait()
	for i := range got {
		if errs[i] != nil || got[i] != 4 {
			t.Fatalf("revert %d: v%d, %v", i, got[i], errs[i])
		}
	}
	wantChain(t, s, "conventions", "checks", 4, v1)
	if !bytes.Equal(published(t, s, "conventions", "checks"), v1) {
		t.Fatal("the published entry is not version 1")
	}
	if strings.Count(readLog(t, s), "| reverted |") != 1 {
		t.Fatalf("log:\n%s", readLog(t, s))
	}
}

func TestRolePoliciesDefaultToTheLocalOwnerAndMembers(t *testing.T) {
	for _, c := range []struct {
		role string
		want bool
	}{
		{"", true}, {"  ", true}, {"owner", true}, {"member", true}, {" member ", true},
		{"guest", false}, {"agent", false}, {"Owner", false}, {"admin", false}, {"owner,guest", false},
	} {
		if got := ChangesReadAllowed(c.role); got != c.want {
			t.Errorf("ChangesReadAllowed(%q) = %v", c.role, got)
		}
		if got := RevertAllowed(c.role); got != c.want {
			t.Errorf("RevertAllowed(%q) = %v", c.role, got)
		}
	}
}
