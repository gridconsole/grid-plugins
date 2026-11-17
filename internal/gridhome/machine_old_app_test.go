package gridhome

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// oldAppHome makes a home whose machine entry is the older app's folder.
func oldAppHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, "machine")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.json", "host.lock", "licence", "licence.moved", "machine-key", "machine-token"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("dummy "+name+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func wantFolderKept(t *testing.T, dir string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "machine-token"))
	if err != nil || string(b) != "dummy machine-token\n" {
		t.Errorf("%s lost its files: %q, %v", dir, b, err)
	}
}

func TestSetAsideOldMachine(t *testing.T) {
	home := oldAppHome(t)
	moved, err := SetAsideOldMachine(home)
	if err != nil || moved != filepath.Join(home, "machine.old-app") {
		t.Fatalf("moved to %q, %v", moved, err)
	}
	wantFolderKept(t, moved)
	if _, err := os.Lstat(filepath.Join(home, "machine")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("machine is still there: %v", err)
	}
	// The id file is now free to be made, and a second call leaves everything as it is.
	if err := os.WriteFile(filepath.Join(home, "machine"), []byte("01ARZ3NDEKTSV4RRFFQ69G5FAV\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if again, err := SetAsideOldMachine(home); again != "" || err != nil {
		t.Fatalf("second call: %q, %v", again, err)
	}
	if id, err := ReadMachine(home); id != "01ARZ3NDEKTSV4RRFFQ69G5FAV" || err != nil {
		t.Fatalf("id after the second call: %q, %v", id, err)
	}
}

func TestSetAsideOldMachineNeverOverwrites(t *testing.T) {
	home := oldAppHome(t)
	taken := filepath.Join(home, "machine.old-app")
	if err := os.Mkdir(taken, 0o755); err != nil { // even an empty folder is not replaced
		t.Fatal(err)
	}
	moved, err := SetAsideOldMachine(home)
	if err != nil || moved != taken+"-2" {
		t.Fatalf("moved to %q, %v", moved, err)
	}
	wantFolderKept(t, moved)
	if entries, _ := os.ReadDir(taken); len(entries) != 0 {
		t.Errorf("the earlier folder was changed: %v", entries)
	}
}

func TestSetAsideOldMachineLeavesOtherEntriesAlone(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV\n"
	for name, setup := range map[string]func(t *testing.T, home string){
		"missing": func(t *testing.T, home string) {},
		"id file": func(t *testing.T, home string) {
			if err := os.WriteFile(filepath.Join(home, "machine"), []byte(id), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"hand-edited file": func(t *testing.T, home string) {
			if err := os.WriteFile(filepath.Join(home, "machine"), []byte("my notes\n"), 0o600); err != nil {
				t.Fatal(err)
			}
		},
		"link to a folder": func(t *testing.T, home string) {
			if err := os.Symlink(t.TempDir(), filepath.Join(home, "machine")); err != nil {
				t.Skip("no symlinks here")
			}
		},
	} {
		t.Run(name, func(t *testing.T) {
			home := t.TempDir()
			setup(t, home)
			before, _ := os.Lstat(filepath.Join(home, "machine"))
			if moved, err := SetAsideOldMachine(home); moved != "" || err != nil {
				t.Fatalf("moved %q, %v", moved, err)
			}
			after, _ := os.Lstat(filepath.Join(home, "machine"))
			if (before == nil) != (after == nil) || (before != nil && !os.SameFile(before, after)) {
				t.Errorf("machine changed")
			}
			entries, _ := os.ReadDir(home)
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "machine.old-app") || strings.HasPrefix(e.Name(), ".machine-move") {
					t.Errorf("left %s behind", e.Name())
				}
			}
		})
	}
}

func TestSetAsideOldMachineWhenTheMoveIsRefused(t *testing.T) {
	home := oldAppHome(t)
	old := renameFolder
	renameFolder = func(string, string) error {
		return &os.LinkError{Op: "rename", Err: errors.New("The process cannot access the file because it is being used by another process.")}
	}
	t.Cleanup(func() { renameFolder = old })

	moved, err := SetAsideOldMachine(home)
	if moved != "" || err == nil {
		t.Fatalf("a refused move gave %q, %v", moved, err)
	}
	for _, want := range []string{filepath.Join(home, "machine"), "close the older Grid app", "being used by another process"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	wantFolderKept(t, filepath.Join(home, "machine"))
}

func TestSetAsideOldMachineConcurrently(t *testing.T) {
	home := oldAppHome(t)
	var wg sync.WaitGroup
	moved := make(chan string, 16)
	for i := 0; i < 16; i++ {
		wg.Go(func() {
			to, err := SetAsideOldMachine(home)
			if err != nil {
				t.Error(err)
			}
			// What a writer does next: make the id file when it is missing.
			if f, err := os.OpenFile(filepath.Join(home, "machine"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600); err == nil {
				f.WriteString("01ARZ3NDEKTSV4RRFFQ69G5FAV\n")
				f.Close()
			}
			moved <- to
		})
	}
	wg.Wait()
	close(moved)
	n := 0
	for to := range moved {
		if to != "" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("%d callers moved the folder, want 1", n)
	}
	if id, err := ReadMachine(home); id != "01ARZ3NDEKTSV4RRFFQ69G5FAV" || err != nil {
		t.Errorf("the id file was moved or replaced: %q, %v", id, err)
	}
	entries, _ := os.ReadDir(home)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "machine.old-app-") {
			t.Errorf("a second copy of the folder: %s", e.Name())
		}
	}
	wantFolderKept(t, filepath.Join(home, "machine.old-app"))
}

func TestReadMachineSaysWhenTheOldAppFolderIsThere(t *testing.T) {
	home := oldAppHome(t)
	_, err := ReadMachine(home)
	if err == nil || !strings.Contains(err.Error(), "older Grid app") || strings.Contains(err.Error(), "regular ULID file") {
		t.Fatalf("ReadMachine said %v", err)
	}
	if Machine(home) != "" {
		t.Errorf("Machine read an id out of a folder")
	}
	wantFolderKept(t, filepath.Join(home, "machine")) // a reader moves nothing
}
