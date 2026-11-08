package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

func TestGuestProjectLookups(t *testing.T) {
	h, cwd, _ := bypassFixture(t)
	h.write(filepath.Join(h.gridHome, "projects", "alpha", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	h.record(project.Record{Name: "beta", ID: "01BETA"})
	h.write(filepath.Join(h.gridHome, "projects", "beta", "tasks", "T-03-hidden", "task.md"), "---\nid: T-03\nproject: beta\n---\n")
	h.write(filepath.Join(h.gridHome, "projects", "beta", "tasks", "T-03-hidden", "jobs", "J-01", "inputs.json"), `{"sources":[{"pointer":"hidden.md"}]}`)
	for _, tc := range []struct{ role, member string }{
		{"guest", "gus"}, {"guest", "nobody"},
		{"grid-guest", "gus"}, {"grid-guest", "nobody"},
	} {
		t.Run(tc.role+"/"+tc.member, func(t *testing.T) {
			member := tc.member
			h.env.Getenv = func(k string) string {
				return map[string]string{"GRID_ROLE": tc.role, "GRID_MEMBER": member}[k]
			}
			if code := h.run(cwd, "project", "list", "--json"); code != 0 {
				t.Fatal(h.errb)
			}
			var entries []struct{ Name string }
			if err := json.Unmarshal(h.out.Bytes(), &entries); err != nil {
				t.Fatal(err)
			}
			want := 0
			if member == "gus" {
				want = 1
			}
			if len(entries) != want || len(entries) == 1 && entries[0].Name != "alpha" {
				t.Errorf("project list = %v, want %d invited projects", entries, want)
			}
			if code := h.run(cwd, "where", "--project", "beta", "--json"); code != 1 {
				t.Errorf("hidden project resolved: exit=%d", code)
			}
			if member == "gus" {
				if code := h.run(cwd, "task", "show", "T-02", "--json"); code != 0 {
					t.Errorf("invited task: exit=%d stderr=%s", code, h.errb)
				}
			}
			for _, jsonMode := range []bool{false, true} {
				for _, args := range [][]string{
					{"job", "trace", "hidden.md", "--project", "beta"},
					{"job", "trace", "hidden.md", "--project", "01BETA"},
					{"task", "list", "--project", "beta"},
					{"task", "show", "T-03"},
				} {
					missing := append([]string(nil), args...)
					missing[len(missing)-1] = "unknown"
					if args[1] == "show" {
						missing[len(missing)-1] = "T-99"
					}
					if jsonMode {
						args = append(args, "--json")
						missing = append(missing, "--json")
					}
					code := h.run(cwd, args...)
					out, stderr := h.out.String(), h.errb.String()
					missingCode := h.run(cwd, missing...)
					if code != 1 || code != missingCode || out != h.out.String() || stderr != h.errb.String() {
						t.Errorf("%v differs from unknown: exit=%d/%d stdout=%q/%q stderr=%q/%q", args, code, missingCode, out, h.out, stderr, h.errb)
					}
				}
			}
		})
	}
}
