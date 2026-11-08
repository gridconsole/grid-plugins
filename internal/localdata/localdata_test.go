package localdata

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestMachine(t *testing.T) {
	for _, tc := range []struct {
		name, initial string
		bad           bool
	}{
		{"new", "", false}, {"existing", "01ARZ3NDEKTSV4RRFFQ69G5FAV\n", false}, {"invalid", "not-an-id\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if tc.initial != "" {
				if err := os.WriteFile(filepath.Join(home, "machine"), []byte(tc.initial), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			id, err := Machine(home)
			if (err != nil) != tc.bad {
				t.Fatalf("id %q, error %v", id, err)
			}
			if tc.bad {
				return
			}
			again, err := Machine(home)
			if err != nil || id != again || !ValidID(id) {
				t.Fatalf("unstable identity: %q %q %v", id, again, err)
			}
		})
	}
}

func TestMachineConcurrent(t *testing.T) {
	home := t.TempDir()
	if err := Init(home); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	ids := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Go(func() {
			id, err := Machine(home)
			if err != nil {
				t.Error(err)
			}
			ids <- id
		})
	}
	wg.Wait()
	close(ids)
	first := <-ids
	for id := range ids {
		if id != first {
			t.Errorf("different machine ids")
		}
	}
}

func TestSafe(t *testing.T) {
	home := t.TempDir()
	if err := os.Symlink(t.TempDir(), filepath.Join(home, "linked")); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../escape", "linked/note.md"} {
		if _, err := Safe(home, path); err == nil {
			t.Errorf("accepted %s", path)
		}
	}
}
