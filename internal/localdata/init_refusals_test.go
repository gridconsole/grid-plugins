package localdata

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

func wantErr(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want one containing %q", err, text)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestNoHomeIsRefused(t *testing.T) {
	wantErr(t, Init(""), "no Grid home; set GRID_HOME")
	_, err := Safe("")
	wantErr(t, err, "no Grid home; set GRID_HOME")
	_, err = Machine("")
	if err == nil {
		t.Fatal("Machine(\"\") gave no error")
	}
}

func TestInitHomeIsARegularFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "plain")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Init(f); err == nil {
		t.Fatal("Init on a regular file gave no error")
	}
	if got := readFile(t, f); got != "x" {
		t.Fatalf("file now holds %q, want it untouched", got)
	}
}

func TestInitRefusesAVersionSymlink(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "real")
	if err := os.WriteFile(target, []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	version := filepath.Join(home, "VERSION")
	if err := os.Symlink(target, version); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	err := Init(home)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Init error = %v, want ErrUnsafePath", err)
	}
	if st, err := os.Lstat(version); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("VERSION is no longer a symlink: %v, %v", st, err)
	}
	if got := readFile(t, target); got != "1\n" {
		t.Fatalf("target now holds %q", got)
	}
}

func TestInitRefusesAVersionDirectory(t *testing.T) {
	home := t.TempDir()
	version := filepath.Join(home, "VERSION")
	if err := os.Mkdir(version, 0o755); err != nil {
		t.Fatal(err)
	}
	wantErr(t, Init(home), "VERSION is not a regular file")
	if st, err := os.Lstat(version); err != nil || !st.IsDir() {
		t.Fatalf("VERSION is no longer a directory: %v, %v", st, err)
	}
}

func TestInitRefusesABadOrNewerVersion(t *testing.T) {
	cases := []struct{ name, content, want string }{
		{"not a number", "abc\n", "VERSION does not hold a number"},
		{"newer", fmt.Sprintf("%d\n", gridhome.FormatVersion+1), "Update grid."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			version := filepath.Join(home, "VERSION")
			if err := os.WriteFile(version, []byte(c.content), 0o644); err != nil {
				t.Fatal(err)
			}
			wantErr(t, Init(home), c.want)
			if got := readFile(t, version); got != c.content {
				t.Fatalf("VERSION now holds %q, want %q", got, c.content)
			}
		})
	}
}

func TestMachineRefusesASymlink(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "real")
	if err := os.WriteFile(target, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	machine := filepath.Join(home, "machine")
	if err := os.Symlink(target, machine); err != nil {
		t.Skipf("no symlinks here: %v", err)
	}
	if _, err := Machine(home); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("Machine error = %v, want ErrUnsafePath", err)
	}
	if st, err := os.Lstat(machine); err != nil || st.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("machine is no longer a symlink: %v, %v", st, err)
	}
}

func TestMachineNeverReplacesAForeignFile(t *testing.T) {
	home := t.TempDir()
	machine := filepath.Join(home, "machine")
	if err := os.WriteFile(machine, []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id, err := Machine(home)
	wantErr(t, err, "machine file does not contain a ULID")
	if id != "" {
		t.Fatalf("Machine gave id %q with the error", id)
	}
	if got := readFile(t, machine); got != "hello\n" {
		t.Fatalf("machine now holds %q, want it untouched", got)
	}
}

func TestLockExistingOnADirectory(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	unlock, err := LockExisting(home, "sub")
	if err == nil {
		unlock()
		t.Fatal("LockExisting on a directory gave no error")
	}
	if unlock != nil {
		t.Fatal("LockExisting gave an unlock func with the error")
	}
}
