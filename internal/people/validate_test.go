package people

import (
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

func validOwner() Entry {
	return Entry{Handle: "owner", Name: "Example Owner", Role: "owner", Links: []brain.Link{{To: "area", Rel: "approves"}}}
}

func TestValidateRefusals(t *testing.T) {
	const (
		handle = "person handle must be a plain name"
		name   = "person name is required on one line"
		role   = "person role must be owner, member or guest"
		link   = "person link needs a target and relation on one line"
	)
	cases := []struct {
		label string
		edit  func(*Entry)
		want  string
	}{
		{"handle TOPIC", func(e *Entry) { e.Handle = "TOPIC" }, handle},
		{"handle with slash", func(e *Entry) { e.Handle = "a/b" }, handle},
		{"blank name", func(e *Entry) { e.Name = "  " }, name},
		{"name with newline", func(e *Entry) { e.Name = "a\nb" }, name},
		{"role admin", func(e *Entry) { e.Role = "admin" }, role},
		{"link without target", func(e *Entry) { e.Links = []brain.Link{{To: "", Rel: "approves"}} }, link},
		{"link without relation", func(e *Entry) { e.Links = []brain.Link{{To: "area", Rel: ""}} }, link},
		{"link target with newline", func(e *Entry) { e.Links = []brain.Link{{To: "a\nb", Rel: "knows"}} }, link},
	}
	for _, c := range cases {
		t.Run(c.label, func(t *testing.T) {
			e := validOwner()
			c.edit(&e)
			err := e.Validate()
			if err == nil || err.Error() != c.want {
				t.Fatalf("Validate() = %v, want %q", err, c.want)
			}
		})
	}
}

func TestValidateAcceptsEachRole(t *testing.T) {
	for _, role := range []string{"owner", "member", "guest"} {
		e := validOwner()
		e.Role = role
		if err := e.Validate(); err != nil {
			t.Fatalf("role %q: %v", role, err)
		}
	}
}
