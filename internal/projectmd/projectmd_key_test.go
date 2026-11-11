package projectmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/where"
)

const projectmdKeyGuest = "guest-1"

// projectmdKeyHome holds the project "sample" with a project-roles.json that
// invites projectmdKeyGuest, and the project "other" which invites nobody.
func projectmdKeyHome(t *testing.T) (home, path string) {
	t.Helper()
	home = t.TempDir()
	path = writeProjectMD(t, home, "sample", keyFixture)
	roles := `{"people":[{"id":"` + projectmdKeyGuest + `"}]}`
	if err := os.WriteFile(filepath.Join(home, "projects", "sample", "project-roles.json"), []byte(roles), 0o644); err != nil {
		t.Fatal(err)
	}
	writeProjectMD(t, home, "other", "---\nid: other-project\nlands: hidden\n---\nbody\n")
	return home, path
}

func projectmdKeyFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func projectmdKeyResolver(home, role, member string) where.Resolver {
	return where.Resolver{GridHome: home, Caller: where.Caller{Role: role, Member: member}}
}

// Roles that read and write: TierFor maps owner and member to PersonTeam; the
// empty Caller is an Agent allowed by DefaultPersonalAllowed, so not a guest.
func TestKeyPerRoleNonGuestReadsAndSets(t *testing.T) {
	for _, c := range []struct{ name, role, member string }{
		{"owner", "owner", "alice"},
		{"member", "member", "alice"},
		{"empty caller", "", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			home, path := projectmdKeyHome(t)
			r := projectmdKeyResolver(home, c.role, c.member)
			got, err := Key(r, "sample", "lands", nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := (Result{Project: "sample", Key: "lands", Value: "reviewed"}); !reflect.DeepEqual(got, want) {
				t.Fatalf("read: got %+v; want %+v", got, want)
			}
			value := "shipped"
			got, err = Key(r, "sample", "lands", &value)
			if err != nil {
				t.Fatal(err)
			}
			if want := (Result{Project: "sample", Key: "lands", Value: "shipped"}); !reflect.DeepEqual(got, want) {
				t.Fatalf("set: got %+v; want %+v", got, want)
			}
			if want := strings.Replace(keyFixture, "lands: reviewed", "lands: shipped", 1); projectmdKeyFile(t, path) != want {
				t.Fatalf("project.md after set:\n%q\nwant:\n%q", projectmdKeyFile(t, path), want)
			}
		})
	}
}

// A guest reads an invited project and cannot set a key (projectmd.go:66-68).
func TestKeyPerRoleGuestReadsAndCannotSet(t *testing.T) {
	for _, role := range []string{"guest", "external", "grid-guest"} {
		t.Run(role, func(t *testing.T) {
			home, path := projectmdKeyHome(t)
			r := projectmdKeyResolver(home, role, projectmdKeyGuest)
			got, err := Key(r, "sample", "lands", nil)
			if err != nil {
				t.Fatal(err)
			}
			if want := (Result{Project: "sample", Key: "lands", Value: "reviewed"}); !reflect.DeepEqual(got, want) {
				t.Fatalf("read: got %+v; want %+v", got, want)
			}
			value := "shipped"
			if _, err = Key(r, "sample", "lands", &value); err == nil || err.Error() != "guests cannot write project.md" {
				t.Fatalf("set: err = %v; want %q", err, "guests cannot write project.md")
			}
			if projectmdKeyFile(t, path) != keyFixture {
				t.Fatalf("a refused guest set changed project.md:\n%q", projectmdKeyFile(t, path))
			}
		})
	}
}

func TestKeyNotFoundPerRole(t *testing.T) {
	home, _ := projectmdKeyHome(t)
	for _, role := range []string{"owner", "member"} {
		t.Run(role, func(t *testing.T) {
			_, err := Key(projectmdKeyResolver(home, role, "alice"), "nowhere", "lands", nil)
			if err == nil || err.Error() != `project "nowhere" not found` {
				t.Fatalf("err = %v; want %q", err, `project "nowhere" not found`)
			}
		})
	}
	for _, role := range []string{"guest", "external", "grid-guest"} {
		for _, name := range []string{"nowhere", "other"} {
			t.Run(role+" asks for "+name, func(t *testing.T) {
				_, err := Key(projectmdKeyResolver(home, role, projectmdKeyGuest), name, "lands", nil)
				if err == nil || err.Error() != "project not found" {
					t.Fatalf("err = %v; want %q", err, "project not found")
				}
			})
		}
	}
	// A guest who is not invited to "sample" does not find it either, set or read.
	value := "x"
	if _, err := Key(projectmdKeyResolver(home, "guest", "someone-else"), "sample", "lands", &value); err == nil || err.Error() != "project not found" {
		t.Fatalf("uninvited guest set: err = %v; want %q", err, "project not found")
	}
}

func TestKeySetRefusesValuesJSONCannotCarry(t *testing.T) {
	for _, c := range []struct{ name, value, want string }{
		{"nan", ".nan", "value cannot be represented as JSON: json: unsupported value: NaN"},
		{"inf", ".inf", "value cannot be represented as JSON: json: unsupported value: +Inf"},
		{"two documents", "a\n---\nb", "expected single document"},
	} {
		t.Run(c.name, func(t *testing.T) {
			home, path := projectmdKeyHome(t)
			v := c.value
			_, err := Key(projectmdKeyResolver(home, "owner", "alice"), "sample", "lands", &v)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err = %v; want it to contain %q", err, c.want)
			}
			if projectmdKeyFile(t, path) != keyFixture {
				t.Fatalf("a refused set changed project.md:\n%q", projectmdKeyFile(t, path))
			}
		})
	}
}

// "extra" is not a project.Record field, so project.Load accepts the file and
// Key itself refuses to read a value with a non-string mapping key.
func TestKeyReadRefusesNonStringMappingKeys(t *testing.T) {
	for _, c := range []struct{ name, stored, kept string }{
		{"mapping", "extra: {1: a}\n", "extra: {1: a}\n"},
		// yaml.Dump writes a block list without the extra indent.
		{"list holding a mapping", "extra:\n  - {1: a}\n", "extra:\n- {1: a}\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			home := t.TempDir()
			text := "---\nid: p\n" + c.stored + "---\nbody\n"
			path := writeProjectMD(t, home, "p", text)
			r := where.Resolver{GridHome: home}
			if _, err := Key(r, "p", "extra", nil); err == nil || err.Error() != "value needs string mapping keys" {
				t.Fatalf("read: err = %v; want %q", err, "value needs string mapping keys")
			}
			if projectmdKeyFile(t, path) != text {
				t.Fatalf("a read changed project.md:\n%q", projectmdKeyFile(t, path))
			}
			// A set of another key still works and keeps the extra line.
			value := "shipped"
			got, err := Key(r, "p", "lands", &value)
			if err != nil {
				t.Fatal(err)
			}
			if got.Value != "shipped" {
				t.Fatalf("set value = %#v; want shipped", got.Value)
			}
			after := projectmdKeyFile(t, path)
			if want := "---\nid: p\n" + c.kept + "lands: shipped\n---\nbody\n"; after != want {
				t.Fatalf("project.md after set:\n%q\nwant:\n%q", after, want)
			}
		})
	}
}

func TestProjectmdKeyJSONValue(t *testing.T) {
	for _, c := range []struct {
		name string
		in   any
		want bool
	}{
		{"list holding a map[any]any", []any{map[any]any{}}, false},
		{"map holding a list holding a map[any]any", map[string]any{"a": []any{map[any]any{}}}, false},
		{"map holding a list of strings", map[string]any{"a": []any{"x"}}, true},
		{"map[any]any itself", map[any]any{1: "a"}, false},
		{"string", "x", true},
		{"int", 3, true},
		{"nil", nil, true},
	} {
		if got := jsonValue(c.in); got != c.want {
			t.Errorf("%s: jsonValue(%#v) = %v; want %v", c.name, c.in, got, c.want)
		}
	}
}
