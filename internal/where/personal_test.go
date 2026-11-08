package where

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestDefaultPersonalAllowed(t *testing.T) {
	for _, tc := range []struct {
		caller  Caller
		allowed bool
	}{
		{Caller{}, true}, {Caller{Role: "owner", Member: "sam"}, true},
		{Caller{Role: "member"}, false}, {Caller{Role: "lead"}, false},
		{Caller{Role: "member", Member: "sam"}, false}, {Caller{Role: "lead", Member: "sam"}, false},
		{Caller{Role: "guest"}, false}, {Caller{Role: "external"}, false},
		{Caller{Role: "unknown"}, false}, {Caller{Role: "OWNER"}, true},
		{Caller{Role: " owner "}, true}, {Caller{Role: " "}, true},
		{Caller{Role: "cli"}, true}, {Caller{Role: "worker", Member: "sam"}, true}, {Caller{Role: "builder"}, true},
	} {
		t.Run(tc.caller.Role+"/"+tc.caller.Member, func(t *testing.T) {
			if got := DefaultPersonalAllowed(tc.caller); got != tc.allowed {
				t.Errorf("DefaultPersonalAllowed(%+v) = %t, want %t", tc.caller, got, tc.allowed)
			}
		})
	}
}

func TestPersonalGuardAllResolutionRules(t *testing.T) {
	for _, tc := range resolveCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			q, want := tc.setup(f)
			f.r.Caller = Caller{Role: "guest", Member: "gus"}
			got, err := f.r.Resolve(q)
			if want.Project == Personal {
				wantErr := ErrPersonalRefused
				if q.Project != "" {
					wantErr = ErrProjectUnregistered
				} else if q.Task != "" && strings.ContainsAny(q.Task, "/\\") {
					wantErr = nil
				}
				if wantErr == nil && (err == nil || err.Error() != "task not found") || wantErr != nil && !errors.Is(err, wantErr) {
					t.Fatalf("Resolve() error = %v, want %v (nil means task not found)", err, wantErr)
				}
				if strings.Contains(err.Error(), f.gridHome) {
					t.Errorf("refusal = %q, want path-free message", err)
				}
				if !reflect.DeepEqual(got, Result{}) {
					t.Errorf("refused resolution leaked a result: %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Resolve() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestPersonalAllowedOverride(t *testing.T) {
	for _, role := range []string{"member", "owner"} {
		t.Run(role, func(t *testing.T) {
			f := newFixture(t)
			path := f.personalTask("T-07-private")
			f.r.Caller = Caller{Role: role}
			calls := 0
			f.r.PersonalAllowed = func(got Caller) bool {
				calls++
				if got.Role != role {
					t.Errorf("policy role = %q, want %q", got, role)
				}
				return got.Role == "member"
			}
			for _, q := range []Query{
				{Cwd: f.home, Project: "PERSONAL"},
				{Cwd: f.gridHome},
				{Cwd: path},
				{Cwd: path, Task: "T-07"},
			} {
				calls = 0
				got, err := f.r.Resolve(q)
				if calls != 1 {
					t.Errorf("policy called %d times, want once", calls)
				}
				if role == "member" {
					if err != nil || got.Project != Personal {
						t.Errorf("member override: result %+v, error %v", got, err)
					}
				} else {
					want := ErrPersonalRefused
					if q.Project != "" {
						want = ErrProjectUnregistered
					}
					if !errors.Is(err, want) {
						t.Errorf("owner override: error %v, want %v", err, want)
					}
				}
			}
		})
	}
}

func TestCallerFrom(t *testing.T) {
	got := CallerFrom(func(key string) string {
		return map[string]string{"GRID_ROLE": " member \n", "GRID_MEMBER": " sam\t"}[key]
	})
	if got != (Caller{Role: "member", Member: "sam"}) {
		t.Fatalf("caller = %+v", got)
	}
}

func TestPersonalTasksDirs(t *testing.T) {
	f := newFixture(t)
	f.personalTask("T-01-private")
	f.task("alpha", "T-02-public")
	f.record(project.Record{Name: Personal}) // Must not reintroduce Personal via the registry.
	for _, role := range []string{"guest", "owner"} {
		f.r.Caller = Caller{Role: role, Member: "sam"}
		dirs, err := f.r.TasksDirs()
		if err != nil {
			t.Fatal(err)
		}
		personal := gridhome.TasksDir(f.gridHome, Personal)
		found := false
		for _, dir := range dirs {
			if dir == personal {
				found = true
			}
		}
		if found != (role == "owner") {
			t.Errorf("%s Personal visibility = %t", role, found)
		}
	}
}

func TestPersonalRefusedBeforeProjectLookup(t *testing.T) {
	f := newFixture(t)
	f.r.Caller = Caller{Role: "guest", Member: "gus"}
	// A broken registry must not be read to refuse an explicit Personal selector.
	if err := os.WriteFile(filepath.Join(f.gridHome, "projects"), []byte("broken"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, task := range []string{"T-01", "T-99"} {
		_, err := f.r.Resolve(Query{Cwd: f.gridHome, Project: "PERSONAL", Task: task})
		if !errors.Is(err, ErrProjectUnregistered) || err.Error() != `project "PERSONAL" is not registered` {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestPersonalRecordIDRefused(t *testing.T) {
	f := newFixture(t)
	f.record(project.Record{Name: Personal, ID: "01PERSONAL"})
	f.personalTask("T-01-private")
	f.r.Caller = Caller{Role: "guest", Member: "gus"}
	for _, task := range []string{"", "T-01", "T-99"} {
		_, err := f.r.Resolve(Query{Cwd: f.home, Project: "01PERSONAL", Task: task})
		if !errors.Is(err, ErrProjectUnregistered) {
			t.Fatalf("task %q error = %v", task, err)
		}
	}
}

func TestPersonalRolePrefix(t *testing.T) {
	for _, role := range []string{"owner", "member", "lead", "guest", "external", "cli", "host", "worker", "agent", "builder", "checker", "curator", "designer", "library", "operator", "planner", "researcher", "reviewer", "runner", "scout", "unknown", "grid-unknown"} {
		for _, member := range []string{"", "sam"} {
			want := DefaultPersonalAllowed(Caller{Role: role, Member: member})
			for _, spelling := range []string{"grid-" + role, " GRID-" + strings.ToUpper(role) + " "} {
				if got := DefaultPersonalAllowed(Caller{Role: spelling, Member: member}); got != want {
					t.Errorf("%q member=%q: %t, want %t", spelling, member, got, want)
				}
			}
		}
	}
}

func TestPersonalRepeatedRolePrefixRefused(t *testing.T) {
	for _, role := range []string{"grid-", "grid-grid-owner", "grid-grid-member", "grid-grid-worker"} {
		if DefaultPersonalAllowed(Caller{Role: role}) {
			t.Errorf("repeated prefix accepted: %q", role)
		}
	}
}
