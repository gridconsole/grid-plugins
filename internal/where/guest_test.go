package where

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func guestRoles(t *testing.T, home, name, data string) {
	t.Helper()
	dir := filepath.Join(home, "projects", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "project-roles.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGuestTasksDirs(t *testing.T) {
	f := newFixture(t)
	f.personalTask("T-01-private")
	f.task("alpha", "T-02-invited")
	f.task("beta", "T-03-hidden")
	guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"gus","perm":"Client"}]}`)
	guestRoles(t, f.gridHome, "beta", `{"people":[{"id":"someone-else","perm":"Member"}]}`)
	for _, tc := range []struct {
		role, member string
		want         []string
	}{
		{"guest", "gus", []string{"alpha"}},
		{"grid-guest", "gus", []string{"alpha"}},
		{" GRID-GUEST ", " GUS ", []string{"alpha"}},
		{" GUEST ", " GUS ", []string{"alpha"}},
		{"guest", "nobody", []string{}},
		{"guest", "", []string{}},
		{"owner", "gus", []string{Personal, "alpha", "beta"}},
		{"member", "gus", []string{"alpha", "beta"}},
	} {
		t.Run(tc.role+"/"+tc.member, func(t *testing.T) {
			f.r.Caller = Caller{Role: tc.role, Member: tc.member}
			dirs, err := f.r.TasksDirs()
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, dir := range dirs {
				got = append(got, filepath.Base(filepath.Dir(dir)))
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("visible projects = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGuestMembership(t *testing.T) {
	for _, tc := range []struct {
		name, data string
		allowed    bool
	}{
		{"client", `{"people":[{"id":"gus","perm":"Client"}]}`, true},
		{"member", `{"people":[{"id":"gus","perm":"Member"}]}`, true},
		{"case", `{"people":[{"id":"GUS","perm":"Client"}]}`, true},
		{"missing", "", false},
		{"malformed", "{", false},
		{"null", "null", false},
		{"wrong shape", `{"people":"gus"}`, false},
		{"display name", `{"people":[{"id":"other","name":"gus"}]}`, false},
		{"pending invite", `{"invites":[{"id":"gus","state":"sent"}]}`, false},
		{"removed", `{"people":[],"removed":[{"id":"gus"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			f.task("alpha", "T-01")
			f.r.Caller = Caller{Role: "guest", Member: "gus"}
			if tc.data != "" {
				guestRoles(t, f.gridHome, "alpha", tc.data)
			}
			dirs, err := f.r.TasksDirs()
			if err != nil || (len(dirs) == 1) != tc.allowed {
				t.Fatalf("membership allowed = %t, want %t; error = %v", len(dirs) == 1, tc.allowed, err)
			}
		})
	}
}

func TestGuestResolve(t *testing.T) {
	f := newFixture(t)
	alpha := f.dir("work", "alpha")
	beta := f.dir("work", "beta")
	f.record(project.Record{Name: "alpha", ID: "01ALPHA", Folders: here(alpha)})
	f.record(project.Record{Name: "beta", ID: "01BETA", Folders: here(beta)})
	invited := f.task("alpha", "T-01-invited")
	hidden := f.task("beta", "T-02-hidden")
	guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"gus","perm":"Client"}]}`)
	f.r.Caller = Caller{Role: "guest", Member: "gus"}
	for _, q := range []Query{
		{Cwd: f.home, Project: "alpha"}, {Cwd: f.home, Project: "01ALPHA"},
		{Cwd: alpha}, {Cwd: invited}, {Cwd: alpha, Task: "T-01"},
		{Cwd: f.home, Task: invited},
	} {
		got, err := f.r.Resolve(q)
		if err != nil || got.Project != "alpha" {
			t.Errorf("invited project refused: rule=%s error=%v", got.Rule, err)
		}
	}
	for _, selector := range []string{"beta", "01BETA", "unknown"} {
		got, err := f.r.Resolve(Query{Cwd: alpha, Project: selector})
		if !errors.Is(err, ErrProjectUnregistered) || err.Error() != unregisteredProject(selector).Error() || !reflect.DeepEqual(got, Result{}) {
			t.Errorf("selector %q: expected unregistered and empty result, got project=%q error=%v", selector, got.Project, err)
		}
	}
	for _, q := range []Query{
		{Cwd: beta}, {Cwd: hidden}, {Cwd: beta, Task: "T-02"},
		{Cwd: f.home, Task: hidden}, {Cwd: alpha, Task: hidden, Project: "alpha"},
		{Cwd: f.home, Task: filepath.Join(f.gridHome, "projects", "unknown", "tasks", "T-99")},
	} {
		got, err := f.r.Resolve(q)
		if err == nil || !reflect.DeepEqual(got, Result{}) {
			t.Errorf("hidden project returned: project=%q error=%v", got.Project, err)
		} else if strings.Contains(err.Error(), f.root) || strings.Contains(err.Error(), "beta") {
			t.Error("hidden project refusal disclosed its path or name")
		}
		if q.Task != "" && (err == nil || err.Error() != "task not found") {
			t.Errorf("hidden or unknown task: got %v, want task not found", err)
		}
	}
}

func TestGuestResolutionRules(t *testing.T) {
	for _, tc := range resolveCases() {
		for _, invited := range []bool{false, true} {
			t.Run(tc.name+"/invited="+strconv.FormatBool(invited), func(t *testing.T) {
				f := newFixture(t)
				q, want := tc.setup(f)
				f.r.Caller = Caller{Role: "guest", Member: "gus"}
				allowed := invited && want.Project != Personal && (want.Record || want.Task != "")
				if allowed {
					guestRoles(t, f.gridHome, want.Project, `{"people":[{"id":"gus","perm":"Client"}]}`)
				}
				got, err := f.r.Resolve(q)
				if allowed {
					if err != nil || !reflect.DeepEqual(got, want) {
						t.Errorf("invited resolution differs: rule=%s project=%q error=%v", got.Rule, got.Project, err)
					}
				} else if err == nil || !reflect.DeepEqual(got, Result{}) {
					t.Errorf("uninvited resolution returned: rule=%s project=%q error=%v", got.Rule, got.Project, err)
				}
			})
		}
	}
}

func TestGuestMembershipRevocation(t *testing.T) {
	f := newFixture(t)
	f.task("alpha", "T-01")
	f.r.Caller = Caller{Role: "guest", Member: "gus"}
	for _, data := range []string{`{"people":[{"id":"gus"}]}`, `{"people":[]}`} {
		guestRoles(t, f.gridHome, "alpha", data)
		dirs, err := f.r.TasksDirs()
		want := strings.Contains(data, "gus")
		if err != nil || (len(dirs) == 1) != want {
			t.Fatalf("membership change: visible=%d want access=%t error=%v", len(dirs), want, err)
		}
	}
}

func TestGuestUnsafePaths(t *testing.T) {
	for _, kind := range []string{"roles link", "oversize roles", "invalid UTF8", "task root link", "project link", "task link"} {
		t.Run(kind, func(t *testing.T) {
			f := newFixture(t)
			f.r.Caller = Caller{Role: "guest", Member: "gus"}
			f.record(project.Record{Name: "alpha"})
			hidden := f.task("beta", "T-02-hidden")
			guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"gus"}]}`)
			alpha := filepath.Join(f.gridHome, "projects", "alpha")
			link := func(from, to string) {
				t.Helper()
				if err := os.Symlink(from, to); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "roles link":
				guestRoles(t, f.gridHome, "beta", `{"people":[{"id":"gus"}]}`)
				if err := os.Remove(filepath.Join(alpha, "project-roles.json")); err != nil {
					t.Fatal(err)
				}
				link(filepath.Join(f.gridHome, "projects", "beta", "project-roles.json"), filepath.Join(alpha, "project-roles.json"))
			case "oversize roles":
				guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"gus"}],"padding":"`+strings.Repeat("x", 1<<20)+`"}`)
			case "invalid UTF8":
				guestRoles(t, f.gridHome, "alpha", "{\"people\":[{\"id\":\"gus\"}],\"padding\":\"\xff\"}")
			case "task root link":
				link(filepath.Dir(hidden), filepath.Join(alpha, "tasks"))
			case "project link":
				guestRoles(t, f.gridHome, "beta", `{"people":[{"id":"gus"}]}`)
				link(filepath.Join(f.gridHome, "projects", "beta"), filepath.Join(f.gridHome, "projects", "alias"))
			case "task link":
				f.task("alpha", "T-01")
				link(hidden, filepath.Join(alpha, "tasks", "T-02-link"))
			}
			dirs, err := f.r.TasksDirs()
			if err != nil {
				t.Fatal(err)
			}
			for _, dir := range dirs {
				name := filepath.Base(filepath.Dir(dir))
				if name == "alias" || name == "alpha" && kind != "project link" && kind != "task link" {
					t.Errorf("unsafe project listed: %s", kind)
				}
			}
			if kind == "task root link" || kind == "task link" {
				_, err := f.r.Resolve(Query{Cwd: f.home, Project: "alpha", Task: "T-02"})
				if err == nil || err.Error() != "task not found" {
					t.Errorf("linked task error = %v, want task not found", err)
				}
			}
		})
	}
}

func TestGuestHiddenBrokenRecord(t *testing.T) {
	f := newFixture(t)
	f.r.Caller = Caller{Role: "guest", Member: "gus"}
	f.record(project.Record{Name: "beta", ID: "01BETA"})
	if err := os.WriteFile(filepath.Join(f.gridHome, "projects", "beta", "project.md"), []byte("broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, selector := range []string{"beta", "01BETA", "unknown"} {
		_, err := f.r.Resolve(Query{Cwd: f.home, Project: selector})
		if !errors.Is(err, ErrProjectUnregistered) {
			t.Errorf("hidden record error = %v, want unregistered", err)
		}
	}
}

func TestProjectRoleScope(t *testing.T) {
	for _, tc := range []struct {
		role       string
		restricted bool
	}{
		{"", false}, {"owner", false}, {"member", false}, {"lead", false},
		{"cli", false}, {"host", false}, {"worker", false}, {"agent", false},
		{"builder", false}, {"checker", false}, {"curator", false}, {"designer", false},
		{"library", false}, {"operator", false}, {"planner", false}, {"researcher", false},
		{"reviewer", false}, {"runner", false}, {"scout", false},
		{"guest", true}, {"external", true}, {"unknown", true}, {"grid-", true},
		{"grid-grid-owner", true}, {"grid-grid-member", true}, {"grid-grid-worker", true},
	} {
		spellings := []string{tc.role, " " + strings.ToUpper(tc.role) + " "}
		if tc.role != "" {
			spellings = append(spellings, "grid-"+tc.role, " GRID-"+strings.ToUpper(tc.role)+" ")
		}
		for _, role := range spellings {
			t.Run(role, func(t *testing.T) {
				f := newFixture(t)
				f.record(project.Record{Name: "alpha", ID: "id-alpha"})
				f.record(project.Record{Name: "beta", ID: "id-beta"})
				guestRoles(t, f.gridHome, "alpha", `{"people":[{"id":"gus"}]}`)
				f.r.Caller = CallerFrom(func(key string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "gus"}[key] })
				if f.r.guest() != tc.restricted {
					t.Errorf("guest=%t, want %t", f.r.guest(), tc.restricted)
				}
				records, err := f.r.Projects()
				want := 2
				if tc.restricted {
					want = 1
				}
				if err != nil || len(records) != want {
					t.Errorf("visible records=%d, want %d; err=%v", len(records), want, err)
				}
				for _, selector := range []string{"beta", "id-beta"} {
					_, err := f.r.Resolve(Query{Project: selector})
					if (err != nil) != tc.restricted {
						t.Errorf("hidden selector %q: err=%v, restricted=%t", selector, err, tc.restricted)
					}
				}
			})
		}
	}
}
