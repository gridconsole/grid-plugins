package feed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMachineIDMovesTheOldAppFolderAside(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "machine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "machine", "machine-token"), []byte("dummy token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := MachineID(home)
	if err != nil || !IsID(id) {
		t.Fatalf("MachineID: %q, %v", id, err)
	}
	if b, err := os.ReadFile(filepath.Join(home, "machine")); err != nil || string(b) != id+"\n" {
		t.Errorf("machine file holds %q, %v", b, err)
	}
	if b, err := os.ReadFile(filepath.Join(home, "machine.old-app", "machine-token")); err != nil || string(b) != "dummy token\n" {
		t.Errorf("the old folder lost its file: %q, %v", b, err)
	}
	if again, err := MachineID(home); err != nil || again != id {
		t.Errorf("second call: %q, %v; want %q", again, err, id)
	}
}
