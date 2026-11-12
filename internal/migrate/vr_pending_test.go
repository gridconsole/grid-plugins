package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

// A migration cut between its two renames leaves no Grid home and an open journal:
// Pending says so, and recovery moves the staged copy forward.
func TestVrPendCutBetweenTheRenamesLeavesNoHomeAndAPendingJournal(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	l := crashLayouts()[0] // old home in place
	want := cleanRun(t, l, root)

	wipe(t, root)
	from, home := l.build(t, root)
	if Pending(home) {
		t.Fatal("Pending is true before any migration ran")
	}
	crashAt(t, "displaced", from, home)

	if exists(home) {
		t.Fatalf("the home exists after a stop at %q", "displaced")
	}
	if !Pending(home) {
		t.Fatalf("Pending(%s) is false with the journal %s beside an absent home", home, journalPath(home))
	}

	finishAfterCrash(t, from, home)
	if !exists(filepath.Join(home, "VERSION")) {
		t.Fatal("the home was not put back by the recovery")
	}
	if got := migrated(t, root, home); !sameView(got, want) {
		t.Fatalf("the recovered home differs from a clean migration:\nwant %v\n got %v", want, got)
	}
	if Pending(home) {
		t.Fatalf("Pending(%s) is still true after the recovery; journal %v", home, exists(journalPath(home)))
	}
}

// The state this lane keeps grid from making: a grid writer makes the home in the
// gap, and recovery can no longer tell which step the folders are at.
func TestVrPendAHomeMadeInTheGapStopsTheRecovery(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	from, home := crashLayouts()[0].build(t, root)
	crashAt(t, "displaced", from, home)
	if exists(home) || !Pending(home) {
		t.Fatalf("the crash did not leave the gap: home exists %v, pending %v", exists(home), Pending(home))
	}

	if err := os.MkdirAll(home, 0o700); err != nil { // what brain.NewStore's MkdirAll does
		t.Fatal(err)
	}
	const want = "do not match any step of a migration"
	_, err := Scan(from, home)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("Scan after a home was made in the gap: %v; want an error holding %q", err, want)
	}
	if !Pending(home) {
		t.Error("Pending is false: the journal is still open and the person must sort the folders by hand")
	}
}

func TestVrPendIsTheJournalFileWhateverItsType(t *testing.T) {
	root := gridhome.Real(t.TempDir())
	home := filepath.Join(root, ".grid")
	j := journalPath(home)

	if Pending(home) {
		t.Error("Pending is true with no journal")
	}
	if Pending("") {
		t.Error("Pending is true for an empty home path")
	}
	if err := os.WriteFile(j, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, h := range map[string]string{
		"the path as it is":     home,
		"a trailing slash":      home + string(filepath.Separator),
		"dot and dot-dot":       filepath.Join(root, "x", "..", ".grid"),
		"a doubled separator":   root + string(filepath.Separator) + string(filepath.Separator) + ".grid",
		"a dot inside the path": filepath.Join(root, ".") + string(filepath.Separator) + ".grid",
	} {
		if !Pending(h) {
			t.Errorf("Pending is false for %s (%q) with a journal beside the home", name, h)
		}
	}

	if err := os.Remove(j); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(j, 0o700); err != nil {
		t.Fatal(err)
	}
	if !Pending(home) {
		t.Error("Pending is false when the journal path is a folder")
	}
	if err := os.Remove(j); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), j); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if !Pending(home) {
		t.Error("Pending is false when the journal path is a dangling link: Lstat sees the link")
	}
}
