package localdata

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func TestMachine(t *testing.T) {
	for _, tc := range []struct {
		name    string
		initial *string // nil: no machine file
		keep    string  // the id the file already holds
		bad     bool
	}{
		{"new", nil, "", false},
		{"existing", ptr("01ARZ3NDEKTSV4RRFFQ69G5FAV\n"), "01ARZ3NDEKTSV4RRFFQ69G5FAV", false},
		{"empty file is missing", ptr(""), "", false},
		{"torn file is missing", ptr("01ARZ3NDEK"), "", false},
		{"zero-filled file is missing", ptr("\x00\x00\x00\x00"), "", false},
		{"invalid", ptr("not-an-id\n"), "", true},
		{"too long", ptr("01ARZ3NDEKTSV4RRFFQ69G5FAVX\n"), "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			path := filepath.Join(home, "machine")
			if tc.initial != nil {
				if err := os.WriteFile(path, []byte(*tc.initial), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			id, err := Machine(home)
			if (err != nil) != tc.bad {
				t.Fatalf("id %q, error %v", id, err)
			}
			if tc.bad {
				if got, _ := os.ReadFile(path); string(got) != *tc.initial {
					t.Fatalf("a file that is not torn was replaced: %q", got)
				}
				return
			}
			if tc.keep != "" && id != tc.keep {
				t.Fatalf("existing id replaced: %q", id)
			}
			again, err := Machine(home)
			if err != nil || id != again || !gridhome.ValidID(id) || gridhome.Machine(home) != id {
				t.Fatalf("unstable identity: %q %q %v", id, again, err)
			}
			if b, _ := os.ReadFile(path); string(b) != id+"\n" {
				t.Fatalf("machine file holds %q", b)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestMachineConcurrent(t *testing.T) {
	home := t.TempDir()
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
		if _, err := Safe(home, path); !errors.Is(err, ErrUnsafePath) {
			t.Errorf("want unsafe path for %s, got %v", path, err)
		}
	}
}
