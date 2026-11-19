package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridconfig"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

var curatorShowRe = regexp.MustCompile("`(grid brain show [^`]+)`")

// The curator Job names each project and home proposal with a grid brain show
// line. Run exactly that line through the brain command: it must read the
// proposal, on a plain store and on a vault store.
func TestCuratorJobShowCommandReadsHomeAndProjectProposals(t *testing.T) {
	for _, vault := range []bool{false, true} {
		name := "plain"
		if vault {
			name = "vault"
		}
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			h.record(project.Record{Name: "alpha", ID: "01ALPHA"})
			s := task.Store{Resolver: resolver(h.env)}
			created, err := s.New(task.NewOptions{Cwd: h.home, Project: "alpha", Words: "Curate", Size: "small", Workflow: "Check and ship", Who: "alex", Role: "member"})
			if err != nil {
				t.Fatal(err)
			}
			if vault {
				v := filepath.Join(t.TempDir(), "vault")
				if err := os.MkdirAll(v, 0o755); err != nil {
					t.Fatal(err)
				}
				cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: h.gridHome, Brain: v, Member: "Sam"}}}
				if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, h.home), cfg); err != nil {
					t.Fatal(err)
				}
			}
			store := brain.Store{GridHome: h.gridHome, Project: "alpha"}.WithConfig(h.home, nil)
			topics := map[string]string{"home": "decisions", "project": "conventions"}
			for _, level := range []string{"home", "project"} {
				n := brain.Note{Name: level + "-fact", Topic: topics[level], Description: "When checking", Summary: "A " + level + " fact", Source: "job:" + created.ID + "/J-1", Level: level, Body: "Body of the " + level + " fact.\n"}
				if _, err := store.Propose(n, brain.Origin{Kind: "job", ID: created.ID + "/J-1"}); err != nil {
					t.Fatal(err)
				}
			}
			waiting, err := store.Waiting(created.Path)
			if err != nil {
				t.Fatal(err)
			}
			var ids []string
			for _, w := range waiting {
				ids = append(ids, proposalPointer(w))
			}
			if len(ids) != 2 {
				t.Fatalf("waiting pointers %v, want a home and a project proposal", ids)
			}
			body := curatorJobBody("J-1", created.ID, ids)
			acc, _ := body["acceptance"].([]any)
			text, _ := acc[0].(map[string]any)["text"].(string)
			cmds := curatorShowRe.FindAllStringSubmatch(text, -1)
			if len(cmds) != 2 {
				t.Fatalf("job names %d show commands, want 2: %s", len(cmds), text)
			}
			for _, c := range cmds {
				args := strings.Fields(c[1])[2:] // drop "grid brain"
				h.out.Reset()
				h.errb.Reset()
				env := h.env
				env.Cwd = created.Path
				if code := runBrain(args, env); code != 0 {
					t.Errorf("%q exit %d: %s", c[1], code, h.errb)
					continue
				}
				if !strings.Contains(h.out.String(), "fact") {
					t.Errorf("%q printed no proposal: %s", c[1], h.out)
				}
			}
		})
	}
}
