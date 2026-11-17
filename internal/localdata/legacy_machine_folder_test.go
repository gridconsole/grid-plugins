package localdata

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func oldAppFolderHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "machine")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.json", "host.lock", "machine-key", "machine-token"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("dummy "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestMachineMovesTheOldAppFolderAside(t *testing.T) {
	home := oldAppFolderHome(t)
	id, err := Machine(home)
	if err != nil || !gridhome.ValidID(id) {
		t.Fatalf("Machine: %q, %v", id, err)
	}
	if b, err := os.ReadFile(filepath.Join(home, "machine")); err != nil || string(b) != id+"\n" {
		t.Errorf("machine file holds %q, %v", b, err)
	}
	for _, name := range []string{"config.json", "host.lock", "machine-key", "machine-token"} {
		if b, err := os.ReadFile(filepath.Join(home, "machine.old-app", name)); err != nil || string(b) != "dummy "+name+"\n" {
			t.Errorf("machine.old-app/%s: %q, %v", name, b, err)
		}
	}
	if again, err := Machine(home); err != nil || again != id {
		t.Errorf("second call: %q, %v; want %q", again, err, id)
	}
	if _, err := os.Lstat(filepath.Join(home, "machine.old-app-2")); err == nil {
		t.Error("the second call moved something aside again")
	}
}

func TestMachineConcurrentOverTheOldAppFolder(t *testing.T) {
	home := oldAppFolderHome(t)
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
			t.Errorf("different machine ids: %q and %q", first, id)
		}
	}
	if got := gridhome.Machine(home); got != first || !gridhome.ValidID(got) {
		t.Errorf("machine file holds %q, callers saw %q", got, first)
	}
}
