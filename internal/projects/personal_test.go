package projects

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestPersonalListVisibility(t *testing.T) {
	s := Store{}
	s.Resolver.GridHome = t.TempDir()
	for _, name := range []string{"alpha", "personal"} {
		if err := project.Write(s.Resolver.GridHome, project.Record{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	for _, role := range []string{"guest", "owner"} {
		s.Resolver.Caller = where.Caller{Role: role, Member: "sam"}
		entries, err := s.List()
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, entry := range entries {
			if entry.Name == "personal" {
				found = true
			}
		}
		if found != (role == "owner") {
			t.Errorf("%s Personal visibility = %t", role, found)
		}
	}
}
