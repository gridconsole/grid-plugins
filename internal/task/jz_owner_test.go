package task

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"testing"
)

// jzOwEnv is a Call's environment as a function, and the names it was asked for.
type jzOwEnv struct {
	vars  map[string]string
	asked []string
}

func (e *jzOwEnv) get(name string) string {
	e.asked = append(e.asked, name)
	return e.vars[name]
}

// jzOwHost is what a task made with no Who and no Getenv names: the account
// user.Current gives, else USER, else "owner" (create.go machineOwner).
func jzOwHost() string {
	if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" {
		return strings.TrimSpace(u.Username)
	}
	if name := strings.TrimSpace(os.Getenv("USER")); name != "" {
		return name
	}
	return "owner"
}

func TestJzOwOwnerRule(t *testing.T) {
	for _, tc := range []struct {
		name string
		vars map[string]string
		want string
	}{
		{"USER is trimmed", map[string]string{"USER": " ana "}, "ana"},
		{"only USERNAME", map[string]string{"USERNAME": "ben"}, "ben"},
		{"USERNAME is trimmed", map[string]string{"USERNAME": "\tben\n"}, "ben"},
		{"USER wins over USERNAME", map[string]string{"USER": "ana", "USERNAME": "ben"}, "ana"},
		{"a blank USER falls to USERNAME", map[string]string{"USER": "   ", "USERNAME": "ben"}, "ben"},
		{"neither", map[string]string{}, "owner"},
		{"both blank", map[string]string{"USER": " ", "USERNAME": ""}, "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := &jzOwEnv{vars: tc.vars}
			who, role := NewOptions{Getenv: env.get}.owner()
			if who != tc.want || role != "owner" {
				t.Fatalf("owner %q role %q, want %q owner", who, role, tc.want)
			}
			for _, name := range env.asked {
				if name != "USER" && name != "USERNAME" {
					t.Errorf("read %q from the Call's environment; only USER and USERNAME are the owner's", name)
				}
			}
		})
	}
	t.Run("Getenv nil is what the host said before", func(t *testing.T) {
		who, role := NewOptions{}.owner()
		if want := jzOwHost(); who != want || role != "owner" {
			t.Fatalf("owner %q role %q, want %q owner", who, role, want)
		}
		if u, err := user.Current(); err == nil && strings.TrimSpace(u.Username) != "" && who != strings.TrimSpace(u.Username) {
			t.Fatalf("owner %q is not user.Current %q", who, u.Username)
		}
	})
	t.Run("Getenv set never falls back to the host account", func(t *testing.T) {
		env := &jzOwEnv{vars: map[string]string{}}
		who, _ := NewOptions{Getenv: env.get}.owner()
		if who != "owner" {
			t.Fatalf("owner %q with an empty environment, want %q (the host is %q)", who, "owner", jzOwHost())
		}
	})
	t.Run("Who wins and the environment is not read", func(t *testing.T) {
		env := &jzOwEnv{vars: map[string]string{"USER": "ana"}}
		who, role := NewOptions{Who: " cy ", Role: "lead", Getenv: env.get}.owner()
		if who != "cy" || role != "lead" || len(env.asked) != 0 {
			t.Fatalf("owner %q role %q, environment read %v; want cy lead and no read", who, role, env.asked)
		}
	})
}

// A task made through Store.New records the owner of its own Getenv: two stores
// in one process, two people.
func TestJzOwNewRecordsTheCallsOwner(t *testing.T) {
	for _, tc := range []struct {
		name string
		vars map[string]string
		want string
	}{
		{"ana", map[string]string{"USER": "ana"}, "ana"},
		{"ben on Windows", map[string]string{"USERNAME": "ben"}, "ben"},
		{"nobody", map[string]string{}, "owner"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, home := storeFor(t)
			env := &jzOwEnv{vars: tc.vars}
			got, err := s.New(NewOptions{Cwd: home, Words: "Name the owner from the call", Getenv: env.get})
			if err != nil {
				t.Fatal(err)
			}
			text, err := os.ReadFile(filepath.Join(got.Path, "task.md"))
			if err != nil || got.Owner != tc.want || !strings.Contains(string(text), "\nowner: "+tc.want+"\n") {
				t.Fatalf("owner %q, task.md %s, %v; want owner: %s", got.Owner, text, err, tc.want)
			}
		})
	}
}
