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

func TestProjectMDGuestAccess(t *testing.T) {
	for _, role := range []string{"guest", "grid-guest", " GRID-GUEST "} {
		for _, member := range []string{"gus", "nobody"} {
			for _, jsonMode := range []bool{false, true} {
				mode := "plain"
				if jsonMode {
					mode = "json"
				}
				t.Run(role+"/"+member+"/"+mode, func(t *testing.T) {
					h := newHarness(t)
					h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
					h.record(project.Record{Name: "beta", ID: "01BETA"})
					h.write(filepath.Join(h.gridHome, "projects", "alpha", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
					h.env.Getenv = func(k string) string {
						return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": member}[k]
					}
					for _, op := range []string{"get", "set"} {
						args := func(selector string) []string {
							a := []string{"project-md", op, selector, "id"}
							if op == "set" {
								a = append(a, "changed")
							}
							if jsonMode {
								a = append(a, "--json")
							}
							return a
						}
						unknownCode := h.run(h.home, args("unknown")...)
						unknownOut, unknownErr := h.out.String(), h.errb.String()
						if unknownCode != 1 || unknownOut != "" || unknownErr != "project-md: project not found\n" {
							t.Errorf("%s unknown: exit=%d stdout=%q stderr=%q", op, unknownCode, unknownOut, unknownErr)
						}
						for _, selector := range []string{"alpha", "01ALPHA", "beta", "01BETA"} {
							name := selector
							if selector == "01ALPHA" {
								name = "alpha"
							}
							if selector == "01BETA" {
								name = "beta"
							}
							path := filepath.Join(h.gridHome, "projects", name, "project.md")
							before, err := os.ReadFile(path)
							if err != nil {
								t.Fatal(err)
							}
							code := h.run(h.home, args(selector)...)
							invited := member == "gus" && name == "alpha"
							if invited && op == "get" {
								want := "id: 01ALPHA\n"
								if jsonMode {
									want = "{\"project\":\"alpha\",\"key\":\"id\",\"value\":\"01ALPHA\"}\n"
								}
								if code != 0 || h.out.String() != want || h.errb.Len() != 0 {
									t.Errorf("invited get %s: exit=%d stdout=%q stderr=%q", selector, code, h.out, h.errb)
								}
							} else if invited {
								if code != 1 || h.out.Len() != 0 || h.errb.String() != "project-md: guests cannot write project.md\n" {
									t.Errorf("invited set %s: exit=%d stdout=%q stderr=%q", selector, code, h.out, h.errb)
								}
							} else if code != unknownCode || h.out.String() != unknownOut || h.errb.String() != unknownErr {
								t.Errorf("%s %s differs from unknown: exit=%d/%d stdout=%q/%q stderr=%q/%q", op, selector, code, unknownCode, h.out, unknownOut, h.errb, unknownErr)
							}
							after, err := os.ReadFile(path)
							if err != nil {
								t.Fatal(err)
							}
							if !bytes.Equal(before, after) {
								t.Errorf("%s %s changed project.md", op, selector)
								h.write(path, string(before))
							}
						}
					}
					if _, err := os.Stat(filepath.Join(h.gridHome, ".projects.lock")); !os.IsNotExist(err) {
						t.Errorf("guest operation acquired the projects lock: %v", err)
					}
				})
			}
		}
	}
}

func TestProjectMDTeamAccess(t *testing.T) {
	for _, role := range []string{"owner", "member"} {
		t.Run(role, func(t *testing.T) {
			h := newHarness(t)
			h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
			h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "nobody"}[k] }
			if code := h.run(h.home, "project-md", "set", "01ALPHA", "lands", "reviewed", "--json"); code != 0 {
				t.Fatalf("set: exit=%d stderr=%s", code, h.errb)
			}
			want := h.out.String()
			if code := h.run(h.home, "project-md", "get", "alpha", "lands", "--json"); code != 0 || h.out.String() != want {
				t.Fatalf("get: exit=%d stdout=%s stderr=%s", code, h.out, h.errb)
			}
		})
	}
}

func TestProjectMDGuestHiddenBrokenRecord(t *testing.T) {
	for _, role := range []string{"guest", "grid-guest"} {
		t.Run(role, func(t *testing.T) {
			h := newHarness(t)
			h.write(filepath.Join(h.gridHome, "projects", "broken", "project.md"), "broken")
			h.env.Getenv = func(k string) string { return map[string]string{"GRID_ROLE": role, "GRID_MEMBER": "nobody"}[k] }
			for _, op := range []string{"get", "set"} {
				for _, selector := range []string{"broken", "unknown"} {
					if code := h.run(h.home, "project-md", op, selector, "id", "--json"); code != 1 || h.out.Len() != 0 || h.errb.String() != "project-md: project not found\n" {
						t.Errorf("%s %s: exit=%d stdout=%q stderr=%q", op, selector, code, h.out, h.errb)
					}
				}
			}
		})
	}
}
