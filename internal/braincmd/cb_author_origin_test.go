package braincmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/task"
)

var cbAuSessionID = regexp.MustCompile(`^[0-9a-f]{24}$`)

// cbAuNewRows is what the audit log gained between two newest-first reads.
func cbAuNewRows(t *testing.T, before, after []brain.Change) []brain.Change {
	t.Helper()
	if len(after) <= len(before) {
		t.Fatalf("the audit log gained nothing: %d rows before, %d after", len(before), len(after))
	}
	return after[:len(after)-len(before)]
}

// cbAuTree reads every file under the Grid home, so a value can be searched
// for in everything a command wrote.
func cbAuTree(t *testing.T, home string) map[string][]byte {
	t.Helper()
	tree := map[string][]byte{}
	err := filepath.WalkDir(home, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		rel, _ := filepath.Rel(home, path)
		tree[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// A proposal made in a Job names that Job, and the audit line written when the
// task's curator accepts it carries the same author.
func TestCbAuProposeInJob(t *testing.T) {
	env, dir := decideFixture(t)
	var out, errb bytes.Buffer
	env.Stdout, env.Stderr, env.Stdin = &out, &errb, strings.NewReader("")
	env.Task, env.Job, env.Session = "T-01", "J-3", "sess-1"
	want := brain.Origin{Kind: "job", ID: "T-01/J-3"}

	code := Run([]string{"propose", "--task", "T-01", "--name", "fromjob", "--topic", "conventions",
		"--description", "When a Job learns something.", "--summary", "A Job proposed this.", "--source", "T-01",
		"--body", "Run the checks.\n", "--json"}, env)
	if code != 0 {
		t.Fatalf("propose: exit %d: %s", code, &errb)
	}
	b, err := os.ReadFile(filepath.Join(dir, "brain-proposals", "fromjob.md"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := brain.Parse(b)
	if err != nil || n.Origin != want {
		t.Fatalf("proposal origin: %+v, %v\n%s", n.Origin, err, b)
	}

	out.Reset()
	errb.Reset()
	if code := Run([]string{"decide", "T-01", "fromjob", "accept", "--json"}, env); code != 0 {
		t.Fatalf("decide: exit %d: %s", code, &errb)
	}
	changes, err := (brain.Store{GridHome: env.GridHome, Project: "sample"}).Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var accepted []brain.Change
	for _, c := range changes {
		if c.Status == "accepted" && c.Name == "fromjob" {
			accepted = append(accepted, c)
		}
	}
	if len(accepted) != 1 || accepted[0].OriginKind != "job" || accepted[0].OriginID != "T-01/J-3" {
		t.Fatalf("audit rows for fromjob: %+v\nall: %+v", accepted, changes)
	}
}

// Revert and supersede name the person the host runs the plugin for, as drop,
// restore, write and rename do; without one they name the session.
func TestCbAuRevertAndSupersedePerson(t *testing.T) {
	for _, c := range []struct {
		member   string
		wantKind string
		wantID   string // empty means a 24-hex session id
	}{
		{"bo", "person", "bo"},
		{"", "session", ""},
		{"no good!", "session", ""},
	} {
		check := func(t *testing.T, rows []brain.Change) {
			t.Helper()
			for _, row := range rows {
				id := row.OriginID
				ok := id == c.wantID
				if c.wantID == "" {
					ok = cbAuSessionID.MatchString(id)
				}
				if row.OriginKind != c.wantKind || !ok {
					t.Errorf("member %q: %s row %s has origin %s:%s, want %s:%s", c.member, row.Status, row.Name, row.OriginKind, id, c.wantKind, c.wantID)
				}
			}
		}
		t.Run("revert/"+c.member, func(t *testing.T) {
			h := newHarness(t)
			h.env.Role, h.env.Member = "owner", c.member
			h.threeVersions(t)
			before := h.changes(t).Changes
			if code := h.run("revert", "decisions/checks", "--to", "1", "--level", "home"); code != 0 {
				t.Fatalf("revert: exit %d: %s", code, &h.err)
			}
			rows := cbAuNewRows(t, before, h.changes(t).Changes)
			if rows[0].Status != "reverted" {
				t.Fatalf("new rows: %+v", rows)
			}
			check(t, rows)
		})
		t.Run("supersede/"+c.member, func(t *testing.T) {
			h := newHarness(t)
			h.env.Role, h.env.Member = "owner", c.member
			h.svNotes(t, "deploy", "deploy-v2")
			before := h.changes(t).Changes
			if code := h.run("supersede", "deploy", "--by", "deploy-v2", "--reason", svReason); code != 0 {
				t.Fatalf("supersede: exit %d: %s", code, &h.err)
			}
			rows := cbAuNewRows(t, before, h.changes(t).Changes)
			statuses := map[string]bool{}
			for _, row := range rows {
				statuses[row.Status] = true
			}
			if !statuses["superseded"] {
				t.Fatalf("new rows: %+v", rows)
			}
			check(t, rows)
		})
	}
}

// A Job or session value that cannot be a plain name is never written; the
// next source is used, and a random session id is the last one.
func TestCbAuFallback(t *testing.T) {
	resolves := func(id string, err error) func(string) (task.Task, error) {
		return func(string) (task.Task, error) { return task.Task{ID: id}, err }
	}
	hex := brain.Origin{Kind: "session"} // ID empty: any 24-hex id
	for _, c := range []struct {
		name     string
		set      func(*Env)
		extra    []string
		want     brain.Origin
		notWrite []string
	}{
		{"job empty, no session", func(e *Env) { e.Job = "" }, nil, hex, nil},
		{"job empty, session", func(e *Env) { e.Job, e.Session = "", "sess-1" }, nil, brain.Origin{Kind: "session", ID: "sess-1"}, nil},
		{"job with a slash", func(e *Env) { e.Job, e.Session = "zz/9", "sess-1" }, nil, brain.Origin{Kind: "session", ID: "sess-1"}, []string{"zz/9"}},
		{"job with a newline", func(e *Env) { e.Job, e.Session = "zz-9\n", "sess-1" }, nil, brain.Origin{Kind: "session", ID: "sess-1"}, []string{"zz-9"}},
		{"job with a newline, no session", func(e *Env) { e.Job = "zz-9\n" }, nil, hex, []string{"zz-9"}},
		{"task does not resolve, session", func(e *Env) { e.Task, e.Session = "T-99", "sess-1" }, nil, brain.Origin{Kind: "session", ID: "sess-1"}, nil},
		{"task does not resolve, no session", func(e *Env) { e.Task = "T-99" }, nil, hex, nil},
		{"task empty", func(e *Env) { e.Task = "" }, nil, hex, nil},
		{"resolver fails", func(e *Env) { e.ResolveTask, e.Session = resolves("T-01", errors.New("boom")), "sess-1" }, nil, brain.Origin{Kind: "session", ID: "sess-1"}, nil},
		{"resolved task id is not a name", func(e *Env) { e.ResolveTask, e.Session = resolves("zz/1", nil), "sess-1" }, nil, brain.Origin{Kind: "session", ID: "sess-1"}, []string{"zz/1"}},
		{"resolved task id is empty", func(e *Env) { e.ResolveTask = resolves("", nil) }, nil, hex, nil},
		{"session with a space", func(e *Env) { e.Job, e.Session = "", "zz 9" }, nil, hex, []string{"zz 9"}},
		{"session with a newline", func(e *Env) { e.Job, e.Session = "", "zz9\n" }, nil, hex, []string{"zz9"}},
		{"nothing valid", func(e *Env) { e.Job, e.Session = "zz/9", "zz 8" }, nil, hex, []string{"zz/9", "zz 8"}},
		{"valid job wins over a bad session", func(e *Env) { e.Session = "zz 9" }, nil, brain.Origin{Kind: "job", ID: "T-01/J-3"}, []string{"zz 9"}},
		{"valid job wins over a session", func(e *Env) { e.Session = "sess-1" }, nil, brain.Origin{Kind: "job", ID: "T-01/J-3"}, nil},
		{"external keeps the import kind", func(e *Env) {}, []string{"--external"}, brain.Origin{Kind: "import", ID: "T-01/J-3"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			env, _ := decideFixture(t)
			var out, errb bytes.Buffer
			env.Stdout, env.Stderr, env.Stdin = &out, &errb, strings.NewReader("")
			env.Task, env.Job = "T-01", "J-3"
			c.set(&env)
			args := append([]string{"propose", "--level", "home", "--name", "checks", "--topic", "decisions",
				"--description", "Before changing checks.", "--summary", "Run the checks.", "--source", "T-01",
				"--body", "# Checks\nRun make test.\n", "--json"}, c.extra...)
			if code := Run(args, env); code != 0 {
				t.Fatalf("propose: exit %d: %s", code, &errb)
			}
			var entry brain.Entry
			if err := json.Unmarshal(out.Bytes(), &entry); err != nil {
				t.Fatal(err)
			}
			disk, err := os.ReadFile(filepath.Join(env.GridHome, filepath.FromSlash(entry.Path)))
			if err != nil {
				t.Fatal(err)
			}
			stored, err := brain.Parse(disk)
			if err != nil {
				t.Fatal(err)
			}
			same := func(got brain.Origin) bool {
				if c.want.ID == "" {
					return got.Kind == c.want.Kind && cbAuSessionID.MatchString(got.ID)
				}
				return got == c.want
			}
			if !same(entry.Note.Origin) || !same(stored.Origin) {
				t.Fatalf("origin: printed %+v, stored %+v, want %+v", entry.Note.Origin, stored.Origin, c.want)
			}
			// The author reaches the audit line of the accepted note unchanged.
			// An imported proposal is quarantined and cannot be accepted.
			if len(c.extra) == 0 {
				out.Reset()
				if code := Run([]string{"accept", "checks", "--level", "home"}, env); code != 0 {
					t.Fatalf("accept: exit %d: %s", code, &errb)
				}
				changes, err := (brain.Store{GridHome: env.GridHome}).Changes("home", time.Time{}, 0)
				if err != nil || len(changes) != 1 {
					t.Fatalf("changes: %+v, %v", changes, err)
				}
				if got := (brain.Origin{Kind: changes[0].OriginKind, ID: changes[0].OriginID}); !same(got) {
					t.Fatalf("audit origin: %+v, want %+v", got, c.want)
				}
			}
			// An invalid value is in no file the command wrote.
			var paths []string
			tree := cbAuTree(t, env.GridHome)
			for path := range tree {
				paths = append(paths, path)
			}
			sort.Strings(paths)
			for _, bad := range c.notWrite {
				for _, path := range paths {
					if bytes.Contains(tree[path], []byte(bad)) {
						t.Errorf("%s contains the invalid value %q", path, bad)
					}
				}
			}
		})
	}
}

// Every author braincmd stamps comes from one of two functions: hyDrOrigin
// (the person, else the session) and cbAuProposeOrigin (the Job, else the
// session); brain.SessionOrigin is their last fallback and nobody else's. The
// package also reads no environment variable: every one enters through Env.
func TestCbAuSetWalk(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("no sources: %v", err)
	}
	allowed := map[string]bool{"hyDrOrigin": true, "cbAuProposeOrigin": true}
	seen := map[string]bool{}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		scan := func(owner string, node ast.Node) {
			ast.Inspect(node, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok {
					return true
				}
				where := fset.Position(sel.Pos())
				switch {
				case pkg.Name == "brain" && sel.Sel.Name == "SessionOrigin":
					seen[owner] = true
					if !allowed[owner] {
						t.Errorf("%s: brain.SessionOrigin used in %q; use hyDrOrigin or cbAuProposeOrigin", where, owner)
					}
				case pkg.Name == "os" && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv" || sel.Sel.Name == "Environ"):
					t.Errorf("%s: os.%s in %q; read the environment in internal/cli and pass it in Env", where, sel.Sel.Name, owner)
				}
				return true
			})
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				scan(fn.Name.Name, fn)
			} else {
				scan("(package level)", decl)
			}
		}
	}
	for owner := range allowed {
		if !seen[owner] {
			t.Errorf("%s no longer calls brain.SessionOrigin; the walk would pass without checking anything", owner)
		}
	}
}
