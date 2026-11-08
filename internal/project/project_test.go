package project

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"go.yaml.in/yaml/v4"
)

func TestNormalizeRemote(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://example.com/Org/Repo.git", "example.com/org/repo"},
		{"git@example.com:org/repo.git", "example.com/org/repo"},
		{"ssh://git@example.com:2222/org/repo/", "example.com/org/repo"},
		{"https://user:token@example.com/org/repo", "example.com/org/repo"},
		{"HTTP://EXAMPLE.com/org/repo.git/", "example.com/org/repo"},
		{"file:///srv/repo.git", "/srv/repo"},
		{"/srv/repo.git", "/srv/repo"},
		{"  ", ""},
		{"", ""},
	}
	for _, tc := range tests {
		if got := NormalizeRemote(tc.in); got != tc.want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// sample is a multi-repo project known on two machines; the paths need YAML quoting.
var sample = Record{
	Name: "suite",
	ID:   "01SUITE",
	Folders: []Folder{
		{Machine: "01LAPTOP", Path: "/work/suite #1"},
		{Machine: "01DESK", Path: `C:\work\suite`},
	},
	Repos: []Repo{
		{Path: "app", Remote: "example.com/org/app"},
		{Path: "libs/core"},
	},
	Keys: []Key{
		{Key: "remote:example.com/org/app"},
		{Key: "root:0123abcd"},
		{Key: "common:/work/suite #1/libs/core/.git", Machine: "01LAPTOP"},
	},
}

// The written record is the contract other tools read: valid YAML front matter.
func TestFormatGolden(t *testing.T) {
	text, err := Format(sample)
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "project-record", string(text))
	body, ok := strings.CutPrefix(string(text), "---\n")
	body, ok2 := strings.CutSuffix(body, "---\n")
	if !ok || !ok2 {
		t.Fatalf("not front matter:\n%s", text)
	}
	var doc map[string]any
	if err := yaml.Load([]byte(body), &doc); err != nil {
		t.Fatalf("front matter is not valid YAML: %v\n%s", err, body)
	}
	for _, k := range []string{"id", "folders", "repos", "keys"} {
		if _, ok := doc[k]; !ok {
			t.Errorf("front matter has no %q", k)
		}
	}
}

func TestParseReadsWhatFormatWrites(t *testing.T) {
	text, err := Format(sample)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse("suite", string(text)+"\nNotes about the project.\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, sample) {
		t.Errorf("got  %+v\nwant %+v", got, sample)
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		want    Record
		wantErr string
	}{
		{"flow lists", "---\nid: '01ABC'\nfolders: [{machine: M1, path: /w/a}]\nkeys: [{key: 'remote:example.com/a'}]\n---\n",
			Record{Name: "p", ID: "01ABC", Folders: []Folder{{"M1", "/w/a"}}, Keys: []Key{{Key: "remote:example.com/a"}}}, ""},
		{"unknown keys are kept out", "---\nid: 01ABC\ncreated: 2026-11-08\n---\n", Record{Name: "p", ID: "01ABC"}, ""},
		{"empty front matter", "---\n---\nbody\n", Record{Name: "p"}, ""},
		{"no front matter", "id: 01ABC\n", Record{}, "no front matter"},
		{"unclosed front matter", "---\nid: 01ABC\n", Record{}, "no front matter"},
		{"the old repeated lines", "---\nfolder: /w/a\nfolder: /w/b\n---\n", Record{}, "front matter"},
		{"a list where a map belongs", "---\nfolders: /w/a\n---\n", Record{}, "front matter"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse("p", tc.text)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestMachineViews(t *testing.T) {
	rec := Record{
		Folders: []Folder{{"01LAPTOP", "/w/a"}, {"01DESK", "/d/a"}, {"", "/nobody"}},
		Keys: []Key{
			{Key: "remote:example.com/a"},
			{Key: "root:1", Machine: "01DESK"}, // a shared kind stays shared, whatever it carries
			{Key: "common:/w/a/.git", Machine: "01LAPTOP"},
			{Key: "common:/d/a/.git", Machine: "01DESK"},
			{Key: "common:/x/.git"}, // a machine-local key without its machine counts nowhere
		},
	}
	tests := []struct {
		machine     string
		wantFolders []string
		wantKeys    []string
	}{
		{"01LAPTOP", []string{"/w/a"}, []string{"remote:example.com/a", "root:1", "common:/w/a/.git"}},
		{"01DESK", []string{"/d/a"}, []string{"remote:example.com/a", "root:1", "common:/d/a/.git"}},
		{"", nil, []string{"remote:example.com/a", "root:1"}},
	}
	for _, tc := range tests {
		if got := rec.FoldersOn(tc.machine); !reflect.DeepEqual(got, tc.wantFolders) {
			t.Errorf("FoldersOn(%q) = %v, want %v", tc.machine, got, tc.wantFolders)
		}
		if got := rec.KeysOn(tc.machine); !reflect.DeepEqual(got, tc.wantKeys) {
			t.Errorf("KeysOn(%q) = %v, want %v", tc.machine, got, tc.wantKeys)
		}
	}
	if !rec.HasAny("01LAPTOP", []string{"x", "common:/w/a/.git"}) || rec.HasAny("01DESK", []string{"common:/w/a/.git"}) || rec.HasAny("01LAPTOP", nil) {
		t.Error("HasAny gave the wrong answer")
	}
}

func TestNewKey(t *testing.T) {
	tests := []struct {
		key  string
		want Key
	}{
		{"remote:example.com/a", Key{Key: "remote:example.com/a"}},
		{"root:1", Key{Key: "root:1"}},
		{"common:/w/a/.git", Key{Key: "common:/w/a/.git", Machine: "01LAPTOP"}},
	}
	for _, tc := range tests {
		if got := NewKey("01LAPTOP", tc.key); got != tc.want {
			t.Errorf("NewKey(%q) = %+v, want %+v", tc.key, got, tc.want)
		}
	}
}

func TestLoadAndWrite(t *testing.T) {
	home := t.TempDir()
	if recs, err := Load(home); err != nil || recs != nil {
		t.Fatalf("no projects folder: got %v, %v", recs, err)
	}
	for _, rec := range []Record{{Name: "b", ID: "02"}, {Name: "a", ID: "01", Folders: []Folder{{"M1", "/w/a"}}}} {
		if err := Write(home, rec); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(home, "projects", "no-record", "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	recs, err := Load(home)
	if err != nil {
		t.Fatal(err)
	}
	want := []Record{{Name: "a", ID: "01", Folders: []Folder{{"M1", "/w/a"}}}, {Name: "b", ID: "02"}}
	if !reflect.DeepEqual(recs, want) {
		t.Errorf("got %+v, want %+v", recs, want)
	}
	if err := Write(home, Record{Name: "../escape"}); err == nil {
		t.Error("Write took a name that is not one folder")
	}
}

// A broken record stops resolution: skipping it could split a project in two.
func TestLoadRefusesABrokenRecord(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "projects", "bad")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project.md"), []byte("---\nkeys: [\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(home); err == nil || !strings.Contains(err.Error(), filepath.Join("bad", "project.md")) {
		t.Errorf("err = %v, want it to name bad/project.md", err)
	}
}
