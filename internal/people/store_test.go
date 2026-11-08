package people

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

func writePerson(t *testing.T, root, name, text string) {
	t.Helper()
	dir := filepath.Join(root, "knowledge", "people")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestListAndGet(t *testing.T) {
	home := t.TempDir()
	writePerson(t, home, "reviewer", "---\nhandle: reviewer\nname: Example Reviewer\nrole: member\n---\nReview notes.\n")
	writePerson(t, home, "owner", "---\nhandle: owner\nname: Example Owner\nrole: owner\n---\n")
	legacy, err := os.ReadFile("testdata/legacy.md")
	if err != nil {
		t.Fatal(err)
	}
	writePerson(t, home, "legacy", string(legacy))
	s := Store{GridHome: home}
	entries, problems, err := s.List()
	if err != nil || len(entries) != 2 || len(problems) != 1 || problems[0].Name != "legacy" || problems[0].Reason == "" {
		t.Fatalf("List: %+v, %+v, %v", entries, problems, err)
	}
	if entries[0].Handle != "owner" || entries[1].Handle != "reviewer" || entries[1].Body != "Review notes.\n" {
		t.Fatalf("entries: %+v", entries)
	}
	got, err := s.Get("reviewer")
	if err != nil || !reflect.DeepEqual(got, entries[1]) {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if _, err := s.Get("absent"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing person: %v", err)
	}
	if _, err := s.Get("legacy"); err == nil {
		t.Fatal("old schema accepted")
	}
}

func TestProjectOverridesHome(t *testing.T) {
	home := t.TempDir()
	writePerson(t, home, "reviewer", "---\nhandle: reviewer\nname: Home Reviewer\nrole: owner\n---\n")
	writePerson(t, home, "other", "---\nhandle: other\nname: Other Member\nrole: member\n---\n")
	project := filepath.Join(home, "projects", "example")
	writePerson(t, project, "reviewer", "---\nhandle: reviewer\nname: Project Guest\nrole: guest\n---\n")
	s := Store{GridHome: home, Project: "example"}
	entries, problems, err := s.List()
	if err != nil || len(problems) != 0 || len(entries) != 2 || entries[1].Name != "Project Guest" {
		t.Fatalf("List: %+v, %+v, %v", entries, problems, err)
	}
	got, err := s.Get("reviewer")
	if err != nil || !reflect.DeepEqual(got, entries[1]) {
		t.Fatalf("Get: %+v, %v", got, err)
	}
	if got, err := s.Get("other"); err != nil || got.Handle != "other" {
		t.Fatalf("home fallback: %+v, %v", got, err)
	}
	writePerson(t, project, "reviewer", "---\nhandle: reviewer\nname: Invalid\nrole: unknown\n---\n")
	entries, problems, err = s.List()
	if err != nil || len(entries) != 1 || entries[0].Handle != "other" || len(problems) != 1 {
		t.Fatalf("invalid override: %+v, %+v, %v", entries, problems, err)
	}
	if _, err := s.Get("reviewer"); err == nil {
		t.Fatal("invalid project override revived home person")
	}
}

func TestValidateAndStrictDecode(t *testing.T) {
	valid := Entry{Handle: "reviewer", Name: "Example Reviewer", Role: "member"}
	for _, role := range []string{"owner", "member", "guest"} {
		valid.Role = role
		if err := valid.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	valid.Role = "administrator"
	if err := valid.Validate(); err == nil {
		t.Fatal("unknown role accepted")
	}
	for _, extra := range []string{"extra: value\n", "links: [person:other]\n", "links:\n  - to: area\n    rel: approves\n    extra: value\n"} {
		if _, err := decode("reviewer", []byte("---\nhandle: reviewer\nname: Reviewer\nrole: member\n"+extra+"---\n")); err == nil {
			t.Fatalf("accepted unknown schema: %s", extra)
		}
	}
	if _, err := decode("other", []byte("---\nhandle: reviewer\nname: Reviewer\nrole: member\n---\n")); err == nil {
		t.Fatal("mismatched handle accepted")
	}
}

func TestApprovers(t *testing.T) {
	entries := Entries{
		{Handle: "zeta", Role: "member", Links: []brain.Link{{To: "release", Rel: "approves"}, {To: "release", Rel: "decides"}}},
		{Handle: "alpha", Role: "owner", Links: []brain.Link{{To: "release", Rel: "decides"}}},
		{Handle: "guest", Role: "guest", Links: []brain.Link{{To: "release", Rel: "approves"}}},
		{Handle: "observer", Role: "member", Links: []brain.Link{{To: "release", Rel: "knows"}, {To: "release", Rel: "owns"}}},
	}
	entries = append(entries, entries[0])
	for _, tc := range []struct {
		area string
		want []string
	}{
		{"release", []string{"alpha", "zeta"}},
		{"Release", []string{"fallback"}},
		{"release/child", []string{"fallback"}},
		{"releas", []string{"fallback"}},
		{" release", []string{"fallback"}},
	} {
		if got := entries.Approvers(tc.area, "fallback"); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%q: %v, want %v", tc.area, got, tc.want)
		}
	}
}

func TestEmptyAndReadErrors(t *testing.T) {
	s := Store{GridHome: filepath.Join(t.TempDir(), "missing"), Project: "example"}
	entries, problems, err := s.List()
	if err != nil || entries == nil || problems == nil || len(entries)+len(problems) != 0 {
		t.Fatalf("missing cache: %+v, %+v, %v", entries, problems, err)
	}
	if _, err := s.Get("missing"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	s.GridHome = t.TempDir()
	writePerson(t, s.GridHome, "large", strings.Repeat("x", (1<<20)+1))
	if _, _, err := s.List(); err == nil {
		t.Fatal("read failure silently treated as invalid schema")
	}
	if _, err := s.Get("large"); err == nil {
		t.Fatal("read failure ignored")
	}
}
