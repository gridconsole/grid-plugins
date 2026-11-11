package cli

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

// jvAuthorEnv answers every variable from vars and nothing from the process, so a
// builder shell that carries GRID_WORKER_ID cannot change a row.
func jvAuthorEnv(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

// jvAuthorAgentShell is the shell of an app worker's Job that carries no role.
func jvAuthorAgentShell() map[string]string {
	return map[string]string{"GRID_WORKER_ID": "dana@01M", "GRID_MEMBER": "dana"}
}

// jvAuthorLines reads the history lines of one task folder; a missing file has none.
func jvAuthorLines(t *testing.T, folder string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(folder, "history.md"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// jvAuthorEvent decodes one receipt line of history.md.
func jvAuthorEvent(t *testing.T, line string) task.Event {
	t.Helper()
	var event task.Event
	if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "- ")), &event); err != nil {
		t.Fatalf("history line %q: %v", line, err)
	}
	return event
}

func TestJvAuthorTable(t *testing.T) {
	for _, tc := range []struct {
		name         string
		vars         map[string]string
		wantID, role string
	}{
		{"worker with worker id", map[string]string{"GRID_ROLE": "worker", "GRID_WORKER_ID": "dana@01M"}, "dana@01M", "worker"},
		{"empty role with worker id", map[string]string{"GRID_WORKER_ID": "dana@01M"}, "dana@01M", "agent"},
		{"grid-worker prefix", map[string]string{"GRID_ROLE": "grid-worker", "GRID_WORKER_ID": "dana@01M"}, "dana@01M", "worker"},
		{"builder with agent", map[string]string{"GRID_ROLE": "builder", "GRID_AGENT": "b-1"}, "b-1", "builder"},
		{"worker without marks", map[string]string{"GRID_ROLE": "worker"}, "grid", "worker"},
		{"worker with actor", map[string]string{"GRID_ROLE": "worker", "GRID_ACTOR": "ann"}, "ann", "worker"},
		{"worker id before agent before worker", map[string]string{"GRID_ROLE": "worker", "GRID_AGENT": "a", "GRID_WORKER": "w", "GRID_WORKER_ID": "i"}, "i", "worker"},
		{"agent before worker", map[string]string{"GRID_ROLE": "worker", "GRID_AGENT": "a", "GRID_WORKER": "w"}, "a", "worker"},
		{"worker only", map[string]string{"GRID_ROLE": "worker", "GRID_WORKER": "w"}, "w", "worker"},
		{"member with worker mark", map[string]string{"GRID_ROLE": "member", "GRID_WORKER": "w-9"}, "w-9", "agent"},
		{"owner with worker mark", map[string]string{"GRID_ROLE": "owner", "GRID_AGENT": "a-2"}, "a-2", "agent"},
		{"cli with worker mark", map[string]string{"GRID_ROLE": "cli", "GRID_WORKER_ID": "dana@01M"}, "dana@01M", "agent"},
		{"guest", map[string]string{"GRID_ROLE": "guest"}, "local", "guest"},
		{"guest with worker id", map[string]string{"GRID_ROLE": "guest", "GRID_WORKER_ID": "dana@01M"}, "local", "guest"},
		{"guest with actor and worker id", map[string]string{"GRID_ROLE": "guest", "GRID_ACTOR": "ann", "GRID_WORKER_ID": "dana@01M"}, "ann", "guest"},
		{"lead", map[string]string{"GRID_ROLE": "lead"}, "local", "lead"},
		{"lead with actor", map[string]string{"GRID_ROLE": "lead", "GRID_ACTOR": "lee"}, "lee", "lead"},
		{"external with agent", map[string]string{"GRID_ROLE": "external", "GRID_AGENT": "x"}, "local", "external"},
		{"owner with actor and worker id", map[string]string{"GRID_ROLE": "owner", "GRID_ACTOR": "alex", "GRID_WORKER_ID": "dana@01M"}, "alex", "owner"},
		// The six rows of TestTaskWriteRecordsRole (task_write_test.go:90-97).
		{"no role", map[string]string{"GRID_MEMBER": "sam"}, "local", "owner"},
		{"owner", map[string]string{"GRID_ROLE": "owner", "GRID_MEMBER": "sam"}, "local", "owner"},
		{"cli", map[string]string{"GRID_ROLE": "cli", "GRID_MEMBER": "sam"}, "local", "owner"},
		{"member", map[string]string{"GRID_ROLE": "member", "GRID_MEMBER": "sam"}, "local", "member"},
		{"member with actor", map[string]string{"GRID_ROLE": "member", "GRID_ACTOR": "sam", "GRID_MEMBER": "sam"}, "sam", "member"},
		{"padded member with actor", map[string]string{"GRID_ROLE": " member ", "GRID_ACTOR": "sam", "GRID_MEMBER": "sam"}, "sam", "member"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.env.Getenv = jvAuthorEnv(tc.vars)
			got := writeOptions(h.env, "T-01", "status", "").Who
			if got.ID != tc.wantID || got.Role != tc.role {
				t.Fatalf("recorded %s/%s, want %s/%s", got.ID, got.Role, tc.wantID, tc.role)
			}
		})
	}
}

// jvAuthorWalk parses the non-test files of this package and lists every call of
// writeOptions, and every assignment to a .Who, .Who.ID or .Who.Role selector, each
// with its file and enclosing function. An assignment reads "guarded" when an
// enclosing if tests !agent, the answer writeAuthor gives.
func jvAuthorWalk(t *testing.T) (calls, assigns []string) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	isWho := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if sel.Sel.Name == "Who" {
			return true
		}
		inner, ok := sel.X.(*ast.SelectorExpr)
		return ok && (sel.Sel.Name == "ID" || sel.Sel.Name == "Role") && inner.Sel.Name == "Who"
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			var stack []ast.Node
			ast.Inspect(fd, func(n ast.Node) bool {
				if n == nil {
					stack = stack[:len(stack)-1]
					return true
				}
				stack = append(stack, n)
				switch n := n.(type) {
				case *ast.CallExpr:
					if id, ok := n.Fun.(*ast.Ident); ok && id.Name == "writeOptions" {
						calls = append(calls, name+":"+fd.Name.Name)
					}
				case *ast.AssignStmt:
					for _, lhs := range n.Lhs {
						if !isWho(lhs) {
							continue
						}
						guarded := false
						for _, up := range stack {
							if cond, ok := up.(*ast.IfStmt); ok {
								if not, ok := cond.Cond.(*ast.UnaryExpr); ok && not.Op == token.NOT {
									if id, ok := not.X.(*ast.Ident); ok && id.Name == "agent" {
										guarded = true
									}
								}
							}
						}
						assigns = append(assigns, fmt.Sprintf("%s:%s:%s guarded=%v", name, fd.Name.Name, types.ExprString(lhs), guarded))
					}
				}
				return true
			})
		}
	}
	sort.Strings(calls)
	sort.Strings(assigns)
	return calls, assigns
}

func TestJvAuthorEverySite(t *testing.T) {
	calls, assigns := jvAuthorWalk(t)
	wantCalls := []string{
		"history.go:runHistory", "land.go:runLand", "orchestrator.go:runPersonApproval", "task.go:runTaskAttach",
		"task.go:runTaskSummary", "task_lane.go:runTaskLane", "task_write.go:runApprove", "task_write.go:runTaskTrash", "task_write.go:runTaskWrite",
	}
	if strings.Join(calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("writeOptions call sites:\n%s\nwant:\n%s", strings.Join(calls, "\n"), strings.Join(wantCalls, "\n"))
	}
	// The author is overwritten only in the decline branch of runTaskWrite and in
	// runTaskTrash, and only when the author is not an agent. signal.go sets the
	// Who string of a signal action (intent.Action), which is not a task author.
	wantAssigns := []string{
		"signal.go:runSignal:act.Who guarded=false", "signal.go:runSignal:act.Who guarded=false",
		"task_write.go:runTaskTrash:o.Who.Role guarded=true",
		"task_write.go:runTaskWrite:o.Who.ID guarded=true", "task_write.go:runTaskWrite:o.Who.ID guarded=true",
	}
	if strings.Join(assigns, "\n") != strings.Join(wantAssigns, "\n") {
		t.Fatalf("assignments to Who:\n%s\nwant:\n%s", strings.Join(assigns, "\n"), strings.Join(wantAssigns, "\n"))
	}

	// End to end: an agent shell is recorded as the worker, in every write.
	t.Run("agent shell", func(t *testing.T) {
		h := newHarness(t)
		fixture := taskFixture(t, h)
		h.env.Getenv = jvAuthorEnv(jvAuthorAgentShell())
		for _, args := range [][]string{
			{"task", "status", fixture.ID, "running"},
			{"history", "add", "worked on it", "--task", fixture.ID},
			{"task", "close", fixture.ID},
		} {
			before := jvAuthorLines(t, fixture.Path)
			if code := h.run(h.home, args...); code != 0 {
				t.Fatalf("%v exit %d: %s", args, code, h.errb)
			}
			after := jvAuthorLines(t, fixture.Path)
			if len(after) != len(before)+1 {
				t.Fatalf("%v wrote %d history lines, want 1", args, len(after)-len(before))
			}
			if last := after[len(after)-1]; !strings.Contains(last, `"who":{"id":"dana@01M","role":"agent"}`) {
				t.Fatalf("%v recorded %s", args, last)
			}
		}
	})

	// A guest shell stays a guest whatever worker it names. A refused command
	// writes no line; a command that runs names role guest.
	for _, lease := range []string{"", "task"} {
		t.Run("guest shell lease="+lease, func(t *testing.T) {
			h := newHarness(t)
			h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
			fixture := taskFixtureProject(t, h, "example")
			vars := jvAuthorAgentShell()
			vars["GRID_ROLE"] = "guest"
			vars["GRID_LEASE"] = lease
			h.env.Getenv = jvAuthorEnv(vars)
			wrote := 0
			for _, args := range [][]string{
				{"task", "status", fixture.ID, "running"},
				{"history", "add", "worked on it", "--task", fixture.ID},
				{"task", "close", fixture.ID},
			} {
				before := jvAuthorLines(t, fixture.Path)
				code := h.run(h.home, args...)
				after := jvAuthorLines(t, fixture.Path)
				if code != 0 {
					t.Logf("%v refused with exit %d: %s", args, code, strings.TrimSpace(h.errb.String()))
					if len(after) != len(before) {
						t.Fatalf("%v exit %d but wrote %d lines", args, code, len(after)-len(before))
					}
					continue
				}
				if len(after) != len(before)+1 {
					t.Fatalf("%v wrote %d history lines, want 1", args, len(after)-len(before))
				}
				wrote++
				if who := jvAuthorEvent(t, after[len(after)-1]).Who; who.Role != "guest" || who.ID != "local" {
					t.Fatalf("%v recorded %+v, want local/guest", args, who)
				}
			}
			if lease == "" && wrote != 0 {
				t.Fatalf("a guest without a task lease wrote %d lines", wrote)
			}
			t.Logf("guest lease=%q: %d of 3 commands ran", lease, wrote)
		})
	}
}

func TestJvAuthorDeclineTrash(t *testing.T) {
	t.Run("agent shell", func(t *testing.T) {
		h := newHarness(t)
		fixture := taskFixture(t, h)
		h.env.Getenv = jvAuthorEnv(jvAuthorAgentShell())
		if code := h.run(h.home, "task", "decline", fixture.ID, "--reason", "x"); code != 0 {
			t.Fatalf("decline exit %d: %s", code, h.errb)
		}
		lines := jvAuthorLines(t, fixture.Path)
		if len(lines) != 1 || !strings.Contains(lines[0], `"who":{"id":"dana@01M","role":"agent"}`) {
			t.Fatalf("decline history = %q", lines)
		}
		if text := readTaskTestFile(t, filepath.Join(fixture.Path, "task.md")); !strings.Contains(text, "declined by dana@01M") {
			t.Fatalf("task.md lacks next: declined by dana@01M:\n%s", text)
		}

		// The delete policy reads the role, which stays owner for an empty role.
		other := taskFixture(t, h)
		if code := h.run(h.home, "task", "delete", other.ID, "--project", "personal"); code != 0 {
			t.Fatalf("delete exit %d: %s", code, h.errb)
		}
		var trashed []string
		err := filepath.WalkDir(filepath.Join(h.gridHome, "trash"), func(path string, d fs.DirEntry, err error) error {
			if err == nil && d.Name() == "history.md" {
				trashed = append(trashed, path)
			}
			return err
		})
		if err != nil || len(trashed) != 1 {
			t.Fatalf("trash history files = %v, %v", trashed, err)
		}
		lines = jvAuthorLines(t, filepath.Dir(trashed[0]))
		if len(lines) != 1 || !strings.Contains(lines[0], `"who":{"id":"dana@01M","role":"agent"}`) {
			t.Fatalf("delete history = %q", lines)
		}
	})

	t.Run("worker role refused as on main", func(t *testing.T) {
		h := newHarness(t)
		fixture := taskFixture(t, h)
		vars := jvAuthorAgentShell()
		vars["GRID_ROLE"] = "worker"
		h.env.Getenv = jvAuthorEnv(vars)
		if code := h.run(h.home, "task", "decline", fixture.ID, "--reason", "x"); code != 1 || !strings.Contains(h.errb.String(), "scope: role cannot decline tasks") {
			t.Fatalf("decline exit %d: %s", code, h.errb)
		}
		if code := h.run(h.home, "task", "delete", fixture.ID, "--project", "personal"); code != 3 || !strings.Contains(h.errb.String(), task.MoveRefusal) {
			t.Fatalf("delete exit %d: %s", code, h.errb)
		}
		if lines := jvAuthorLines(t, fixture.Path); len(lines) != 0 {
			t.Fatalf("a refused command wrote %q", lines)
		}
		if _, err := os.Stat(fixture.Path); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("person shells keep their text", func(t *testing.T) {
		h := newHarness(t)
		// A member declines in a shared project, as task_rename_decline_test.go does.
		h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
		fixture := taskFixtureProject(t, h, "example")
		h.env.Getenv = jvAuthorEnv(map[string]string{"GRID_ROLE": "member"})
		if code := h.run(h.home, "task", "decline", fixture.ID, "--project", "example", "--reason", "x"); code != 0 {
			t.Fatalf("decline exit %d: %s", code, h.errb)
		}
		if text := readTaskTestFile(t, filepath.Join(fixture.Path, "task.md")); !strings.Contains(text, "declined by you") {
			t.Fatalf("task.md lacks next: declined by you:\n%s", text)
		}
		if who := jvAuthorEvent(t, jvAuthorLines(t, fixture.Path)[0]).Who; who.ID != "local" || who.Role != "member" {
			t.Fatalf("decline recorded %+v", who)
		}
	})
}
