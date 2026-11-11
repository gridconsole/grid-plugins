package projectmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

// writeProjectMD puts text at <home>/projects/<name>/project.md.
func writeProjectMD(t *testing.T, home, name, text string) string {
	t.Helper()
	dir := filepath.Join(home, "projects", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "project.md")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

const keyFixtureBody = "# Sample\n\nBody line one.\n\n    indented code\n\nLast line, no tab\tchange.\n"

// keyFixture is front matter in the writer's own layout plus a body, so a set
// of one key can be compared with the whole file.
const keyFixture = "---\nid: example-project\nlands: reviewed\nowner: someone\n---\n" + keyFixtureBody

func TestKeyReadsAnExistingKey(t *testing.T) {
	home := t.TempDir()
	writeProjectMD(t, home, "sample", keyFixture)
	got, err := Key(where.Resolver{GridHome: home}, "sample", "lands", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := Result{Project: "sample", Key: "lands", Value: "reviewed"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
}

func TestKeyFindsTheProjectByIDAndReadsTypedValues(t *testing.T) {
	home := t.TempDir()
	writeProjectMD(t, home, "sample", "---\nid: example-project\ncount: 3\non: true\ntags:\n  - a\n  - b\n---\n")
	r := where.Resolver{GridHome: home}
	cases := []struct {
		key  string
		want any
	}{
		{"count", 3},
		{"on", true},
		{"tags", []any{"a", "b"}},
	}
	for _, c := range cases {
		got, err := Key(r, "example-project", c.key, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.key, err)
		}
		if got.Project != "sample" || got.Key != c.key || !reflect.DeepEqual(got.Value, c.want) {
			t.Fatalf("%s: got %+v (%T); want value %#v in project sample", c.key, got, got.Value, c.want)
		}
	}
}

func TestKeyMissingKeyIsAnErrorAndKeepsTheFile(t *testing.T) {
	home := t.TempDir()
	path := writeProjectMD(t, home, "sample", keyFixture)
	got, err := Key(where.Resolver{GridHome: home}, "sample", "absent", nil)
	if err == nil || !strings.Contains(err.Error(), `front-matter key "absent" is not set`) {
		t.Fatalf("err = %v; want the key-is-not-set error", err)
	}
	if got.Project != "sample" || got.Key != "absent" || got.Value != nil {
		t.Fatalf("got %+v; want project and key filled, value nil", got)
	}
	if b, _ := os.ReadFile(path); string(b) != keyFixture {
		t.Fatalf("a read changed the file:\n%s", b)
	}
}

func TestKeyMissingProjectFileAndFolderAreNotFound(t *testing.T) {
	home := t.TempDir()
	// A project folder without project.md is skipped by the registry, so it is not found.
	if err := os.MkdirAll(filepath.Join(home, "projects", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := where.Resolver{GridHome: home}
	for _, name := range []string{"empty", "nowhere"} {
		got, err := Key(r, name, "lands", nil)
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Fatalf("%s: err = %v; want a not-found error", name, err)
		}
		if got.Project != "" || got.Value != nil {
			t.Fatalf("%s: got %+v; want an empty result", name, got)
		}
	}
	// No projects folder at all.
	if _, err := Key(where.Resolver{GridHome: t.TempDir()}, "sample", "lands", nil); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("no projects folder: err = %v; want a not-found error", err)
	}
}

func TestKeyRefusesNoHomeAndABadKey(t *testing.T) {
	if _, err := Key(where.Resolver{}, "sample", "lands", nil); err == nil || !strings.Contains(err.Error(), "GRID_HOME") {
		t.Fatalf("no home: err = %v", err)
	}
	home := t.TempDir()
	writeProjectMD(t, home, "sample", keyFixture)
	for _, k := range []string{"", "a b", "1x", "a:b", "a\nb"} {
		if _, err := Key(where.Resolver{GridHome: home}, "sample", k, nil); err == nil || !strings.Contains(err.Error(), "single front-matter key") {
			t.Fatalf("key %q: err = %v; want the single-key error", k, err)
		}
	}
}

func TestKeySetAddsANewKeyAndKeepsTheRestByteEqual(t *testing.T) {
	home := t.TempDir()
	path := writeProjectMD(t, home, "sample", keyFixture)
	value := "42"
	got, err := Key(where.Resolver{GridHome: home}, "sample", "budget", &value)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Result{Project: "sample", Key: "budget", Value: 42}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nid: example-project\nlands: reviewed\nowner: someone\nbudget: 42\n---\n" + keyFixtureBody
	if string(b) != want {
		t.Fatalf("file after set:\n%q\nwant:\n%q", b, want)
	}
}

func TestKeySetReplacesAnExistingKeyAndKeepsTheRestByteEqual(t *testing.T) {
	home := t.TempDir()
	path := writeProjectMD(t, home, "sample", keyFixture)
	value := "shipped"
	got, err := Key(where.Resolver{GridHome: home}, "sample", "lands", &value)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Result{Project: "sample", Key: "lands", Value: "shipped"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v; want %+v", got, want)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(keyFixture, "lands: reviewed", "lands: shipped", 1)
	if string(b) != want {
		t.Fatalf("file after set:\n%q\nwant:\n%q", b, want)
	}
	// A second read returns the new value and not the old one.
	again, err := Key(where.Resolver{GridHome: home}, "sample", "lands", nil)
	if err != nil || again.Value != "shipped" {
		t.Fatalf("read back %+v, %v", again, err)
	}
}

func TestKeySetOfTheSameValueLeavesTheFileUnchanged(t *testing.T) {
	home := t.TempDir()
	path := writeProjectMD(t, home, "sample", keyFixture)
	value := "reviewed"
	if _, err := Key(where.Resolver{GridHome: home}, "sample", "lands", &value); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(path); string(b) != keyFixture {
		t.Fatalf("file changed:\n%q", b)
	}
}

func TestKeySetOmittedValueIsTheEmptyString(t *testing.T) {
	home := t.TempDir()
	path := writeProjectMD(t, home, "sample", keyFixture)
	value := ""
	got, err := Key(where.Resolver{GridHome: home}, "sample", "lands", &value)
	if err != nil {
		t.Fatal(err)
	}
	if got.Value != "" {
		t.Fatalf("value %#v; want the empty string", got.Value)
	}
	b, _ := os.ReadFile(path)
	if want := strings.Replace(keyFixture, "lands: reviewed", `lands: ""`, 1); string(b) != want {
		t.Fatalf("file after set:\n%q\nwant:\n%q", b, want)
	}
}

func TestKeySetRefusesABadValueAndKeepsTheFile(t *testing.T) {
	home := t.TempDir()
	path := writeProjectMD(t, home, "sample", keyFixture)
	for name, value := range map[string]string{
		"two documents":      "a\n---\nb",
		"non-string map key": "{1: a}",
		"broken yaml":        "[unclosed",
	} {
		v := value
		if _, err := Key(where.Resolver{GridHome: home}, "sample", "lands", &v); err == nil {
			t.Fatalf("%s: set %q succeeded; want an error", name, value)
		}
		if b, _ := os.ReadFile(path); string(b) != keyFixture {
			t.Fatalf("%s: a refused set changed the file:\n%q", name, b)
		}
	}
}

func TestPlainRendersEachKind(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"string", "reviewed", "reviewed"},
		{"empty string", "", ""},
		{"string with newline and return", "a\nb\r\nc", `a\nb\r\nc`},
		{"int", 42, "42"},
		{"float", 1.5, "1.5"},
		{"bool true", true, "true"},
		{"bool false", false, "false"},
		{"list", []any{"a", "b"}, "- a\n- b"},
		{"list of numbers", []any{1, 2}, "- 1\n- 2"},
		{"map", map[string]any{"k": "v"}, "k: v"},
		{"nil", nil, "null"},
	}
	for _, c := range cases {
		if got := Plain(c.in); got != c.want {
			t.Errorf("%s: Plain(%#v) = %q; want %q", c.name, c.in, got, c.want)
		}
	}
}
