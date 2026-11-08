package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestProjectMD(t *testing.T) {
	for _, tc := range []struct {
		name, key, value string
		code             int
	}{
		{"scalar", "lands", "reviewed", 0}, {"list", "labels", "[one, two]", 0}, {"empty", "summary", "", 0}, {"bad-key", "body.text", "x", 1}, {"bad-identity", "folders", "wrong type", 1}, {"bad-yaml", "lands", "[", 1}, {"non-finite", "limit", ".inf", 1}, {"mapping-key", "data", "{1: two}", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			if err := project.Write(home, project.Record{Name: "sample", ID: "example-project"}); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(home, "projects", "sample", "project.md")
			b, _ := os.ReadFile(path)
			b = append(b, []byte("# Notes\r\n\r\nKeep this body.\r\n")...)
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			var out, stderr bytes.Buffer
			env := Env{GridHome: home, Stdout: &out, Stderr: &stderr}
			code := Run([]string{"project-md", "set", "example-project", tc.key, tc.value, "--json"}, env)
			if code != tc.code {
				t.Fatalf("code %d: %s", code, &stderr)
			}
			after, _ := os.ReadFile(path)
			if tc.code != 0 {
				if !bytes.Equal(b, after) {
					t.Fatal("failed set changed file")
				}
				return
			}
			set := out.String()
			out.Reset()
			if Run([]string{"project-md", "get", "sample", tc.key, "--json"}, env) != 0 || out.String() != set {
				t.Fatalf("get mismatch: %s %s", &out, &stderr)
			}
			if !strings.HasSuffix(string(after), "# Notes\r\n\r\nKeep this body.\r\n") {
				t.Fatal("body changed")
			}
			golden, err := os.ReadFile(filepath.Join("testdata", "project-md-"+tc.name+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if set != string(golden) {
				t.Fatalf("got %s", set)
			}
		})
	}
}
