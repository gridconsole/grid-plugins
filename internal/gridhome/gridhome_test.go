package gridhome

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDir(t *testing.T) {
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "GRID_HOME" {
				return v
			}
			return ""
		}
	}
	tests := []struct {
		name, gridHome, userHome, want string
	}{
		{"env wins", "/data/grid/", "/home/example", "/data/grid"},
		{"default", "", "/home/example", filepath.Join("/home/example", ".grid")},
		{"nothing known", "", "", ""},
	}
	for _, tc := range tests {
		if got := Dir(env(tc.gridHome), tc.userHome); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestTilde(t *testing.T) {
	tests := []struct{ path, home, want string }{
		{"/home/example", "/home/example", "~"},
		{"/home/example/.grid/projects", "/home/example", "~/.grid/projects"},
		{"/home/examplex/y", "/home/example", "/home/examplex/y"},
		{"/srv/data", "/home/example", "/srv/data"},
		{"/srv/data", "", "/srv/data"},
	}
	for _, tc := range tests {
		if got := Tilde(tc.path, tc.home); got != tc.want {
			t.Errorf("Tilde(%q, %q) = %q, want %q", tc.path, tc.home, got, tc.want)
		}
	}
}

func TestReadFormat(t *testing.T) {
	tests := []struct {
		name    string
		content *string
		want    int
		wantErr bool
	}{
		{"number", ptr("1\n"), 1, false},
		{"padded", ptr("  3 "), 3, false},
		{"not a number", ptr("abc"), 0, true},
		{"missing", nil, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.content != nil {
				if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(*tc.content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ReadFormat(dir)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Errorf("got %d, %v", got, err)
			}
		})
	}
}

func ptr(s string) *string { return &s }

func TestReal(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(root, link); err != nil {
		t.Skip("symlinks not available")
	}
	if got, want := Real(filepath.Join(link, "not", "yet")), filepath.Join(root, "not", "yet"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTasksDir(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grid")
	if got, want := TasksDir(home, Personal), filepath.Join(home, "personal", "tasks"); got != want {
		t.Errorf("Personal: got %q, want %q", got, want)
	}
	if got, want := TasksDir(home, "example"), filepath.Join(home, "projects", "example", "tasks"); got != want {
		t.Errorf("project: got %q, want %q", got, want)
	}
}

func TestTasksDirs(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".grid")
	dirs, err := TasksDirs(home)
	if err != nil || len(dirs) != 1 || dirs[0] != TasksDir(home, Personal) {
		t.Fatalf("empty home: %v, %v", dirs, err)
	}
	for _, name := range []string{"b", "a"} {
		if err := os.MkdirAll(filepath.Join(home, "projects", name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(home, "projects", "stray.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	dirs, err = TasksDirs(home)
	want := []string{TasksDir(home, Personal), TasksDir(home, "a"), TasksDir(home, "b")}
	if err != nil || !reflect.DeepEqual(dirs, want) {
		t.Fatalf("got %v, %v; want %v", dirs, err, want)
	}
}

func TestMachine(t *testing.T) {
	const id = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	for _, tc := range []struct {
		name    string
		content *string
		want    string
		wantErr bool
		legacy  string // what Machine returns for another grid's id
	}{
		{"missing", nil, "", false, ""},
		{"valid", ptr(id + "\n"), id, false, ""},
		{"padded", ptr("  " + id + "  "), id, false, ""},
		{"empty is missing", ptr(""), "", false, ""},
		{"torn is missing", ptr(id[:9]), "", false, ""},
		{"zero-filled is missing", ptr("\x00\x00\x00"), "", false, ""},
		{"not an id", ptr("not-an-id\n"), "", true, "not-an-id"},
		{"too long", ptr(id + "X"), "", true, id + "X"},
		{"first character out of range", ptr("91ARZ3NDEKTSV4RRFFQ69G5FAV"), "", true, "91ARZ3NDEKTSV4RRFFQ69G5FAV"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.content != nil {
				if err := os.WriteFile(filepath.Join(dir, "machine"), []byte(*tc.content), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			got, err := ReadMachine(dir)
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Errorf("ReadMachine: %q, %v", got, err)
			}
			want := tc.want
			if tc.wantErr {
				want = tc.legacy
			}
			if Machine(dir) != want {
				t.Errorf("Machine: %q, want %q", Machine(dir), want)
			}
		})
	}
}

func TestMachineNotAFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "machine"), 0o755); err != nil {
		t.Fatal(err)
	}
	if id, err := ReadMachine(dir); id != "" || err == nil {
		t.Errorf("a folder read as an id: %q, %v", id, err)
	}
}
