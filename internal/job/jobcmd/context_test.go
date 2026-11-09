package jobcmd

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func ownerResolver(home string) where.Resolver {
	return where.Resolver{GridHome: home, Caller: where.Caller{Role: "owner"}}
}

func TestMissingMemberContext(t *testing.T) {
	base, out, errs := fixture(t)
	// Deliberately omit caller context, as a new adapter might accidentally do.
	env := Env{Cwd: base.Cwd, GridHome: base.GridHome, Machine: base.Machine, Stdout: out, Stderr: errs}
	if code := Run([]string{"render", "T-01", "1"}, env); code != 2 {
		t.Fatalf("exit %d, want 2; stdout %s stderr %s", code, out, errs)
	}
	if !strings.Contains(errs.String(), "missing member context") {
		t.Fatalf("missing context diagnostic: %s", errs)
	}
	if out.Len() != 0 {
		t.Fatalf("unexpected output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(env.Cwd, "T-01", "jobs")); !os.IsNotExist(err) {
		t.Fatalf("refused render created artifacts: %v", err)
	}
}

func TestResolverContextRequiredForEveryCommand(t *testing.T) {
	for _, context := range []struct {
		name     string
		resolver where.Resolver
	}{
		{"empty", where.Resolver{}},
		{"paths only", where.Resolver{GridHome: "grid"}},
		{"blank role", where.Resolver{Caller: where.Caller{Role: " \t\n"}}},
		{"member without role", where.Resolver{Caller: where.Caller{Member: "sam"}}},
	} {
		for _, command := range [][]string{
			{"render", "T-01", "1"},
			{"lint", "brief.md"},
			{"show", "T-01", "J-1"},
			{"verify", "J-1", "--task", "T-01"},
			{"trace", "brain:project/x"},
		} {
			t.Run(context.name+"/"+command[0], func(t *testing.T) {
				env, out, errs := fixture(t)
				env.Resolver = context.resolver
				for _, asJSON := range []bool{false, true} {
					out.Reset()
					errs.Reset()
					args := append([]string{}, command...)
					if asJSON {
						args = append(args, "--json")
					}
					if code := Run(args, env); code != 2 {
						t.Fatalf("%v: exit %d, want 2; stdout %s stderr %s", args, code, out, errs)
					}
					message := errs.String()
					if asJSON {
						if !json.Valid(out.Bytes()) || errs.Len() != 0 {
							t.Fatalf("invalid JSON response: stdout %s stderr %s", out, errs)
						}
						message = out.String()
					} else if out.Len() != 0 {
						t.Fatalf("unexpected output: %s", out)
					}
					if !strings.Contains(message, "missing member context") {
						t.Fatalf("missing context diagnostic: %s", message)
					}
				}
				if _, err := os.Stat(filepath.Join(env.Cwd, "T-01", "jobs")); !os.IsNotExist(err) {
					t.Fatalf("refused command created artifacts: %v", err)
				}
			})
		}
	}
}

func TestExplicitOwnerContext(t *testing.T) {
	env, out, errs := fixture(t)
	env.Resolver = ownerResolver(env.GridHome)
	r := call(t, env, out, errs, 0, "render", "T-01", "1", "--json")
	if !r.OK {
		t.Fatalf("explicit owner render failed: %+v", r)
	}
	if _, err := os.Stat(r.Path); err != nil {
		t.Fatal(err)
	}
}

func TestTaskPathMembership(t *testing.T) {
	for _, state := range []string{"uninvited", "no-roles", "unknown", "invited", "member", "owner"} {
		t.Run(state, func(t *testing.T) {
			env, _, _ := fixture(t)
			taskPath := filepath.Join(env.Cwd, "T-01")
			projectPath := filepath.Dir(env.Cwd)
			env.Resolver.Caller = where.Caller{Role: "guest", Member: "gus"}
			switch state {
			case "unknown":
				if err := os.RemoveAll(projectPath); err != nil {
					t.Fatal(err)
				}
			case "uninvited", "invited":
				member := "someone-else"
				if state == "invited" {
					member = "gus"
				}
				if err := os.WriteFile(filepath.Join(projectPath, "project-roles.json"), []byte(`{"people":[{"id":"`+member+`"}]}`), 0o644); err != nil {
					t.Fatal(err)
				}
			case "member", "owner":
				env.Resolver.Caller.Role = state
			}
			// Owners and members get the path they gave; the resolver's symlink-free
			// answer is only for guests.
			want := taskPath
			if state == "invited" {
				want = gridhome.Real(taskPath)
			}
			for _, selector := range []string{taskPath, "./T-01", "T-01"} {
				got, err := resolveTask(selector, env)
				if state == "uninvited" || state == "no-roles" || state == "unknown" {
					if got != "" || err == nil || err.Error() != "task not found" {
						t.Fatalf("%q: got %q, %v; want task not found", selector, got, err)
					}
				} else if err != nil || got != want {
					t.Fatalf("%q: got %q, %v; want %q", selector, got, err, want)
				}
			}
			// A task folder outside the Grid home: kept for owners and members,
			// refused for every guest.
			outside := filepath.Join(t.TempDir(), "T-09")
			if err := os.MkdirAll(filepath.Join(outside, "nested"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outside, "plan.md"), []byte("# plan\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, form := range []struct{ name, selector, cwd string }{
				{"outside path", outside, env.Cwd},
				{"outside relative path", "./T-09", filepath.Dir(outside)},
				{"outside cwd", "", filepath.Join(outside, "nested")},
			} {
				e := env
				e.Cwd = form.cwd
				got, err := resolveTask(form.selector, e)
				if state == "owner" || state == "member" {
					if err != nil || got != outside {
						t.Fatalf("%s: got %q, %v; want %q", form.name, got, err, outside)
					}
				} else if got != "" || err == nil {
					t.Fatalf("%s: got %q, %v; want a refusal", form.name, got, err)
				}
			}
			if state == "owner" || state == "member" || state == "invited" {
				link := filepath.Join(t.TempDir(), "link")
				if err := os.Symlink(taskPath, link); err != nil {
					t.Skipf("symlink: %v", err)
				}
				wantLink := link
				if state == "invited" {
					wantLink = want
				}
				got, err := resolveTask(link, env)
				if err != nil || got != wantLink {
					t.Fatalf("symlink: got %q, %v; want %q", got, err, wantLink)
				}
			}
		})
	}
}

func TestTaskPathResolverRefusal(t *testing.T) {
	env, _, _ := fixture(t)
	taskPath := filepath.Join(env.Cwd, "T-01")
	// Only guests go through the resolver; owners and members keep the local path.
	env.Resolver.Caller = where.Caller{Role: "guest", Member: "gus"}
	refused := errors.New("task not found")
	calls := 0
	env.ResolveTask = func(selector string) (string, error) {
		calls++
		if selector != taskPath {
			t.Fatalf("resolver got %q, want %q", selector, taskPath)
		}
		return "", refused
	}
	for _, selector := range []string{taskPath, "./T-01", "T-01", ""} {
		if selector == "" {
			env.Cwd = filepath.Join(taskPath, "nested")
		}
		if got, err := resolveTask(selector, env); got != "" || !errors.Is(err, refused) {
			t.Fatalf("%q: got %q, %v; want resolver refusal", selector, got, err)
		}
	}
	if calls != 4 {
		t.Fatalf("resolver called %d times, want 4", calls)
	}
}
