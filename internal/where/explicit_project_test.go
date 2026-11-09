package where

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestExplicitProjectNameBeforeID(t *testing.T) {
	f := newFixture(t)
	f.record(project.Record{Name: "alpha", ID: "beta", Folders: here(f.dir("work", "alpha"))})
	folder := f.dir("work", "beta")
	f.record(project.Record{Name: "beta", Folders: here(folder)})
	got, err := f.r.Resolve(Query{Cwd: f.tmp, Project: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Project != "beta" {
		t.Fatalf("--project beta resolved to %q, want %q", got.Project, "beta")
	}
	w := want(RuleExplicit, "beta", folder)
	w.Record = true
	if !equal(got, w) {
		t.Fatal("name lookup did not preserve the selected record's result")
	}
}

func TestFindProjectNameAndID(t *testing.T) {
	alpha := project.Record{Name: "alpha", ID: "beta"}
	beta := project.Record{Name: "beta"}
	gamma := project.Record{Name: "gamma", ID: "beta"}
	for _, tc := range []struct {
		name    string
		records []project.Record
		query   string
		want    project.Record
		ok      bool
	}{
		{"name after id", []project.Record{alpha, beta}, "beta", beta, true},
		{"name before id", []project.Record{beta, alpha}, "beta", beta, true},
		{"name after duplicate ids", []project.Record{alpha, gamma, beta}, "beta", beta, true},
		{"unknown id", []project.Record{beta, alpha}, "beta-id", project.Record{}, false},
		{"duplicate ids reversed", []project.Record{gamma, alpha}, "beta", project.Record{}, false},
		{"id without name match", []project.Record{alpha}, "beta", alpha, true},
		{"duplicate ids", []project.Record{alpha, gamma}, "beta", project.Record{}, false},
		{"empty id", []project.Record{beta}, "", project.Record{}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := findProject(tc.records, tc.query)
			if ok != tc.ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("findProject(%q) = (%+v, %t), want (%+v, %t)", tc.query, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestExplicitProjectDuplicateIDs(t *testing.T) {
	f := newFixture(t)
	for _, name := range []string{"alpha", "gamma"} {
		f.record(project.Record{Name: name, ID: "beta", Folders: here(f.dir("work", name))})
		f.task(name, "T-01-example")
	}
	for _, task := range []string{"", "T-01"} {
		got, err := f.r.Resolve(Query{Cwd: f.tmp, Project: "beta", Task: task})
		if !errors.Is(err, ErrProjectUnregistered) {
			t.Fatalf("task %q: error = %v, want ErrProjectUnregistered", task, err)
		}
		// Exact text also ensures neither competing project nor its folders leaks.
		if err.Error() != `project "beta" is not registered: ambiguous project id` {
			t.Fatalf("unexpected ambiguity diagnostic: %v", err)
		}
		if !reflect.DeepEqual(got, Result{}) {
			t.Fatal("ambiguous lookup returned a project result")
		}
	}
}

func TestExplicitPersonalBeforeProjectID(t *testing.T) {
	for _, selector := range []string{"personal", "Personal", "PERSONAL"} {
		t.Run(selector, func(t *testing.T) {
			f := newFixture(t)
			f.record(project.Record{Name: "alpha", ID: selector})
			for _, role := range []string{"owner", "member"} {
				f.r.Caller = Caller{Role: role}
				got, err := f.r.Resolve(Query{Cwd: f.tmp, Project: selector})
				if role == "member" {
					if !errors.Is(err, ErrProjectUnregistered) || !reflect.DeepEqual(got, Result{}) {
						t.Fatal("Personal refusal changed for a member")
					}
				} else if err != nil || !equal(got, want(RuleExplicit, Personal, f.tmp)) {
					t.Fatal("Personal no-record resolution changed for an owner")
				}
			}
		})
	}
}
