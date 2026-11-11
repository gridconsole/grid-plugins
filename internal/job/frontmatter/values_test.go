package frontmatter

import (
	"strings"
	"testing"
)

func TestIntoRefusesUnmarshalableValue(t *testing.T) {
	if err := Into(map[string]any{"f": func() {}}, &struct{}{}); err == nil {
		t.Fatal("Into with a func value = nil, want an error")
	}
	if err := Into(map[string]any{"c": make(chan int)}, &struct{}{}); err == nil {
		t.Fatal("Into with a chan value = nil, want an error")
	}
}

func TestParseAliasValue(t *testing.T) {
	d, err := Parse("---\na: &x 1\nb: *x\n---\n")
	if err != nil {
		t.Fatal(err)
	}
	if d.Data["b"] != d.Data["a"] {
		t.Fatalf("b = %#v, want the anchored value %#v", d.Data["b"], d.Data["a"])
	}
}

// The opening --- is line 1, so a node on line N of the YAML source is file
// line N+1 (frontmatter.go:109). Only the prefix is asserted; the rest is the
// YAML library's text.
func TestParseUndecodableScalarNamesTheLine(t *testing.T) {
	tests := []struct {
		name, text, prefix string
	}{
		{"list item", "---\nlist:\n  - !!int abc\n---\n", "line 3: "},
		{"map value", "---\na: !!int abc\n---\n", "line 2: "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.text)
			if err == nil {
				t.Fatalf("Parse(%q) = nil error, want one starting %q", tt.text, tt.prefix)
			}
			if !strings.HasPrefix(err.Error(), tt.prefix) {
				t.Fatalf("Parse(%q) error = %q, want prefix %q", tt.text, err, tt.prefix)
			}
		})
	}
}
