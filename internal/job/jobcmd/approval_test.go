package jobcmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func TestIntentPointersCannotBypassPlanApproval(t *testing.T) {
	for _, action := range []string{"render", "verify"} {
		t.Run(action, func(t *testing.T) {
			e, out, errs := fixture(t)
			dir := filepath.Join(e.Cwd, "T-01")
			path := filepath.Join(dir, "plan.md")
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Replace(string(b), "      intent:\n", "      intent:\n        item: intent.md#request\n        goal: intent.md#request\n", 1)
			write := func(text string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), 0600); err != nil {
					t.Fatal(err)
				}
			}
			write(text)
			h := jobtest.Home{GridHome: e.GridHome, Task: dir}
			h.Approve(t)
			write(strings.Replace(text, "Document the export format and column order.", "Document the revised export format and column order.", -1))
			args, code := []string{"render", "T-01", "1", "--json"}, 2
			if action == "verify" {
				args, code = []string{"verify", "J-1", "--task", "T-01", "--json"}, 1
			}
			if got := Run(args, e); got != code {
				t.Errorf("%s: exit %d, want %d", action, got, code)
			}
			var r report
			if err := json.Unmarshal(out.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			if r.OK || !strings.Contains(r.Error, "plan needs approval") {
				t.Fatalf("%s must refuse the edited plan: error %q", action, r.Error)
			}
			if _, err := os.Stat(filepath.Join(dir, "jobs")); !os.IsNotExist(err) {
				t.Fatal("approval refusal created Job artifacts")
			}
			h.Approve(t)
			r = call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
			brief, err := os.ReadFile(r.Path)
			if err != nil {
				t.Fatal(err)
			}
			if !r.OK || !strings.Contains(string(brief), "Document the revised export format and column order.") {
				t.Fatal("reapproved plan did not render its revised acceptance criteria")
			}
		})
	}
}
