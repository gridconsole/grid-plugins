package mdfile

import (
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantFront string
		wantBody  string
		wantErr   string
	}{
		{name: "front matter and body", in: "---\na: 1\nb: 2\n---\n# Title\n\ntext\n", wantFront: "a: 1\nb: 2\n", wantBody: "# Title\n\ntext\n"},
		{name: "empty front matter", in: "---\n---\nbody", wantFront: "", wantBody: "body"},
		{name: "closing line is last and has no newline", in: "---\na: 1\n---", wantFront: "a: 1\n", wantBody: ""},
		{name: "later --- stays in the body", in: "---\na: 1\n---\nx\n---\ny\n", wantFront: "a: 1\n", wantBody: "x\n---\ny\n"},
		{name: "no front matter", in: "# Title\nbody\n", wantErr: "must start with ---"},
		{name: "unterminated block", in: "---\na: 1\nb: 2\n", wantErr: "no closing ---"},
		{name: "opening line only", in: "---\n", wantErr: "no closing ---"},
		{name: "CRLF line ends", in: "---\r\na: 1\r\nb: 2\r\n---\r\nline one\r\nline two\r\n", wantFront: "a: 1\nb: 2\n", wantBody: "line one\r\nline two\r\n"},
		{name: "empty file", in: "", wantErr: "must start with ---"},
		{name: "dashes with trailing text are not a fence", in: "--- x\na: 1\n---\n", wantErr: "must start with ---"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			front, body, err := Split([]byte(tc.in))
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Split error = %v, want containing %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Split: %v", err)
			}
			if string(front) != tc.wantFront {
				t.Errorf("front = %q, want %q", front, tc.wantFront)
			}
			if body != tc.wantBody {
				t.Errorf("body = %q, want %q", body, tc.wantBody)
			}
		})
	}
}

type target struct {
	Name string `yaml:"name"`
	N    int    `yaml:"n"`
}

func TestLoadStrictRefusesUnknownField(t *testing.T) {
	var out target
	err := Load([]byte("name: a\nextra: 1\n"), &out, true)
	if err == nil || !strings.Contains(err.Error(), "front matter:") {
		t.Fatalf("strict Load error = %v, want front matter error for unknown field", err)
	}
}

func TestLoadNonStrictDropsUnknownField(t *testing.T) {
	var out target
	if err := Load([]byte("name: a\nn: 3\nextra: 1\n"), &out, false); err != nil {
		t.Fatalf("non-strict Load: %v", err)
	}
	if out != (target{Name: "a", N: 3}) {
		t.Fatalf("out = %+v, want known fields kept and extra left out", out)
	}
}

func TestLoadKnownFieldsStrict(t *testing.T) {
	var out target
	if err := Load([]byte("name: a\nn: 3\n"), &out, true); err != nil {
		t.Fatalf("strict Load of known fields: %v", err)
	}
	if out != (target{Name: "a", N: 3}) {
		t.Fatalf("out = %+v", out)
	}
}

func TestLoadRefusesDuplicateKey(t *testing.T) {
	var out target
	if err := Load([]byte("name: a\nname: b\n"), &out, false); err == nil {
		t.Fatal("Load with a duplicate key = nil, want error")
	}
}

// plainNodes rule (markdown.go:33): "n.Kind == yaml.AliasNode || n.Anchor != ""
// || n.Style&yaml.TaggedStyle != 0" is refused with "anchors, aliases and tags
// are not supported (line N)".
func TestLoadRefusesAnchorAliasAndTag(t *testing.T) {
	tests := map[string]string{
		"anchor":        "name: &a x\n",
		"alias":         "base: &a x\nname: *a\n",
		"tag":           "name: !!str x\n",
		"custom tag":    "name: !custom x\n",
		"nested tag":    "list:\n  - !!str 1\n",
		"anchor on map": "base: &m\n  k: v\n",
	}
	for name, in := range tests {
		t.Run(name, func(t *testing.T) {
			var out map[string]any
			err := Load([]byte(in), &out, false)
			if err == nil || !strings.Contains(err.Error(), "anchors, aliases and tags are not supported") {
				t.Fatalf("Load(%q) error = %v, want the anchors/aliases/tags refusal", in, err)
			}
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("error %q does not name a line", err)
			}
		})
	}
}

func TestLoadAllowsPlainNesting(t *testing.T) {
	var out map[string]any
	if err := Load([]byte("a:\n  - 1\n  - b: 2\nc: \"x\"\n"), &out, true); err != nil {
		t.Fatalf("plain document refused: %v", err)
	}
}

func TestLoadSyntaxError(t *testing.T) {
	// A bad YAML document fails before plainNodes runs, and says so.
	var out target
	err := Load([]byte("name: [unclosed\n"), &out, false)
	if err == nil || !strings.HasPrefix(err.Error(), "front matter:") {
		t.Fatalf("syntax error = %v, want front matter prefix", err)
	}
}
