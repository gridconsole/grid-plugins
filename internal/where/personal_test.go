package where

import (
	"errors"
	"reflect"
	"testing"
)

func TestDefaultPersonalAllowed(t *testing.T) {
	for _, role := range []string{"", "owner", "member", "guest", "unknown", "OWNER", " owner ", " "} {
		t.Run(role, func(t *testing.T) {
			want := role == "" || role == "owner"
			if got := DefaultPersonalAllowed(role); got != want {
				t.Errorf("DefaultPersonalAllowed(%q) = %t, want %t", role, got, want)
			}
		})
	}
}

func TestPersonalGuardAllResolutionRules(t *testing.T) {
	for _, tc := range resolveCases() {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			q, want := tc.setup(f)
			f.r.Role = "guest"
			got, err := f.r.Resolve(q)
			if want.Project == Personal {
				if !errors.Is(err, ErrPersonalRefused) {
					t.Fatalf("Resolve() error = %v, want ErrPersonalRefused", err)
				}
				if err.Error() != "this caller cannot use the Personal project" {
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
			f.r.Role = role
			calls := 0
			f.r.PersonalAllowed = func(got string) bool {
				calls++
				if got != role {
					t.Errorf("policy role = %q, want %q", got, role)
				}
				return got == "member"
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
				} else if !errors.Is(err, ErrPersonalRefused) {
					t.Errorf("owner override: error %v, want ErrPersonalRefused", err)
				}
			}
		})
	}
}
