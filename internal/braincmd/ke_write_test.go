package braincmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

type keWrFix struct {
	t        *testing.T
	env      Env
	files    string
	out, err bytes.Buffer
}

func keWrNew(t *testing.T) *keWrFix {
	t.Helper()
	home := t.TempDir()
	f := &keWrFix{t: t, files: t.TempDir()}
	f.env = Env{Home: home, Cwd: home, GridHome: filepath.Join(home, ".grid")}
	return f
}

// run is runWrite with the Resolver set from GridHome and Home, as Run does.
func (f *keWrFix) run(role, member, stdin string, args ...string) int {
	f.out.Reset()
	f.err.Reset()
	env := f.env
	env.Role, env.Member = role, member
	env.Stdin = strings.NewReader(stdin)
	env.Stdout, env.Stderr = &f.out, &f.err
	env.Resolver.GridHome = env.GridHome
	env.Resolver.Home = env.Home
	env.Resolver.TempRoots = env.TempRoots
	return runWrite(args, env)
}

// path stores bytes as a file outside the Grid home and returns its path.
func (f *keWrFix) path(name string, data []byte) string {
	f.t.Helper()
	p := filepath.Join(f.files, name+".md")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		f.t.Fatal(err)
	}
	return p
}

func keWrBytes(topic, name, body string, mods ...func(*brain.Note)) []byte {
	n := brain.Note{Name: name, Topic: topic, Type: "reference", Description: "Read before changing the " + name + ".",
		Summary: "What the " + name + " says.", Source: "person:ana", Level: "home", Status: "accepted",
		Origin: brain.Origin{Kind: "session", ID: "s0"}, Body: body}
	for _, m := range mods {
		m(&n)
	}
	b, err := brain.Encode(n)
	if err != nil {
		panic(err)
	}
	return b
}

func keWrWorkflow(n *brain.Note) { n.Type = "workflow" }

func keWrSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		if d.IsDir() {
			out[rel] = "dir"
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		out[rel] = fmt.Sprintf("file %q", b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func keWrSame(t *testing.T, before, after map[string]string) {
	t.Helper()
	for k, v := range before {
		if got, ok := after[k]; !ok {
			t.Errorf("%s was removed", k)
		} else if got != v {
			t.Errorf("%s changed:\n before %s\n after  %s", k, v, got)
		}
	}
	for k := range after {
		if _, ok := before[k]; !ok {
			t.Errorf("%s was added", k)
		}
	}
}

func keWrLog(t *testing.T, gridHome string) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(gridHome, "knowledge", "log.md"))
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "- ") {
			lines = append(lines, l)
		}
	}
	return lines
}

const (
	keWrRoleLine     = "grid brain write: caller role is not allowed to write brain entries\n"
	keWrWorkflowLine = "grid brain write: caller role is not allowed to write workflow notes\n"
)

func TestKeWrCommand(t *testing.T) {
	f := keWrNew(t)
	x1 := keWrBytes("decisions", "x", "# X\n\nVersion one.\n")
	x2 := keWrBytes("decisions", "x", "# X\n\nVersion two.\n")
	rev1 := brain.NoteRev(x1)
	flow1 := keWrBytes("decisions", "flow", "# Flow\n", keWrWorkflow)
	flow2 := keWrBytes("decisions", "flow", "# Flow two\n", keWrWorkflow)
	const home = "--level=home"

	ok := func(t *testing.T, want string, role, member string, args ...string) {
		t.Helper()
		if code := f.run(role, member, "", args...); code != 0 || f.out.String() != want || f.err.Len() != 0 {
			t.Fatalf("%v as %q: exit %d, stdout %q (want %q), stderr %q", args, role, code, f.out.String(), want, f.err.String())
		}
	}
	ok(t, "created decisions/x v1 rev "+rev1+"\n", "owner", "ana", "decisions/x", "--expect", "none", "--file", f.path("x1", x1), home)
	ok(t, "created decisions/flow v1 rev "+brain.NoteRev(flow1)+"\n", "owner", "ana", "decisions/flow", "--expect", "none", "--file", f.path("flow1", flow1), home)
	ok(t, "edited decisions/flow v2 rev "+brain.NoteRev(flow2)+"\n", "owner", "ana", "decisions/flow", "--expect", brain.NoteRev(flow1), "--file", f.path("flow2", flow2), home)
	flowRev := brain.NoteRev(flow2)
	// A note placed by hand has no stored version.
	hand := keWrBytes("decisions", "y", "# Y\n")
	if err := os.WriteFile(filepath.Join(f.env.GridHome, "knowledge", "decisions", "y.md"), hand, 0o600); err != nil {
		t.Fatal(err)
	}
	// "notes" is a custom knowledge topic at home level.
	if err := os.MkdirAll(filepath.Join(f.env.GridHome, "topics", "notes"), 0o700); err != nil {
		t.Fatal(err)
	}
	def := "---\nname: notes\nkind: knowledge\nlevel: home\ndescription: A custom topic.\n---\n# notes\n"
	if err := os.WriteFile(filepath.Join(f.env.GridHome, "topics", "notes", "TOPIC.md"), []byte(def), 0o600); err != nil {
		t.Fatal(err)
	}

	// refuse runs one call that must change nothing: same exit, same stderr, same
	// stdout, the home byte-identical and no new log.md line.
	refuse := func(name string, code int, stderr, stdout, role, member string, args ...string) {
		t.Run(name, func(t *testing.T) {
			before, lines := keWrSnapshot(t, f.env.GridHome), len(keWrLog(t, f.env.GridHome))
			got := f.run(role, member, "", args...)
			if got != code {
				t.Errorf("exit %d, want %d (stderr %q, stdout %q)", got, code, f.err.String(), f.out.String())
			}
			if !strings.Contains(f.err.String(), stderr) || (stderr != "" && stderr[0] == '{' && f.err.String() != stderr) {
				t.Errorf("stderr %q, want %q", f.err.String(), stderr)
			}
			if f.out.String() != stdout {
				t.Errorf("stdout %q, want %q", f.out.String(), stdout)
			}
			keWrSame(t, before, keWrSnapshot(t, f.env.GridHome))
			if n := len(keWrLog(t, f.env.GridHome)); n != lines {
				t.Errorf("log.md has %d lines, was %d", n, lines)
			}
		})
	}
	xNew := f.path("xnew", x2)
	conflict := `{"error":"conflict","rev":"` + rev1 + `"}` + "\n"
	notVersioned := `{"error":"not_versioned"}` + "\n"

	// Usage errors come first, for any role.
	refuse("absolute path", 2, "expected <topic>/<name>", "", "owner", "ana", "/abs/x", "--expect", "none", "--file", xNew, home)
	refuse("three parts", 2, "expected <topic>/<name>", "", "owner", "ana", "a/b/c", "--expect", "none", "--file", xNew, home)
	refuse("no slash", 2, "expected <topic>/<name>", "", "owner", "ana", "x", "--expect", "none", "--file", xNew, home)
	refuse("no file flag", 2, "--file is required", "", "owner", "ana", "decisions/x", "--expect", rev1, home)
	refuse("bad expect", 2, "--expect must be", "", "owner", "ana", "decisions/x", "--expect", "ABC", "--file", xNew, home)
	refuse("by flag", 2, "flag provided but not defined: -by", "", "owner", "ana", "decisions/x", "--expect", rev1, "--by", "ana", "--file", xNew, home)
	refuse("level and project", 2, "--level home cannot be combined with --project", "", "owner", "ana", "decisions/x", "--expect", rev1, "--file", xNew, home, "--project", "p")
	refuse("a note that is not valid", 2, "status must be", "", "owner", "ana", "decisions/p", "--expect", "none",
		"--file", f.path("proposed", keWrBytes("decisions", "p", "# P\n", func(n *brain.Note) { n.Status = "proposed" })), home)
	refuse("a missing file", 2, "no such file", "", "owner", "ana", "decisions/p", "--expect", "none", "--file", filepath.Join(f.files, "absent.md"), home)

	// The role is checked before the input is read: guest and worker, new and replace.
	for _, role := range []string{"guest", "worker"} {
		refuse(role+" new note", 3, keWrRoleLine, "", role, "ana", "decisions/n", "--expect", "none", "--file", xNew, home)
		refuse(role+" replace", 3, keWrRoleLine, "", role, "ana", "decisions/x", "--expect", rev1, "--file", xNew, home)
		refuse(role+" workflow note", 3, keWrRoleLine, "", role, "ana", "decisions/n", "--expect", "none", "--file", f.path("wf-"+role, keWrBytes("decisions", "n", "# N\n", keWrWorkflow)), home)
		refuse(role+" unreadable file", 3, keWrRoleLine, "", role, "ana", "decisions/n", "--expect", "none", "--file", filepath.Join(f.files, "absent.md"), home)
	}
	person := `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to write brain entries","code":"person_required","message":"grid brain write: caller role is not allowed to write brain entries"}` + "\n"
	refuse("guest json", 3, keWrRoleLine, person, "guest", "ana", "decisions/n", "--expect", "none", "--file", xNew, home, "--json")

	// A member may write notes, not workflow notes: new text or the note on disk.
	wfPerson := `{"error":"person_required","reason":"role_not_allowed","hint":"caller role is not allowed to write workflow notes","code":"person_required","message":"grid brain write: caller role is not allowed to write workflow notes"}` + "\n"
	refuse("member new workflow text", 3, keWrWorkflowLine, "", "member", "bo", "decisions/n", "--expect", "none", "--file", f.path("wf-member", keWrBytes("decisions", "n", "# N\n", keWrWorkflow)), home)
	refuse("member new workflow text json", 3, keWrWorkflowLine, wfPerson, "member", "bo", "decisions/n", "--expect", "none", "--file", f.path("wf-member", keWrBytes("decisions", "n", "# N\n", keWrWorkflow)), home, "--json")
	refuse("member replaces a workflow note", 3, keWrWorkflowLine, "", "member", "bo", "decisions/flow", "--expect", flowRev, "--file", f.path("plain-flow", keWrBytes("decisions", "flow", "# Plain\n")), home)
	refuse("member replaces a workflow note json", 3, keWrWorkflowLine, wfPerson, "member", "bo", "decisions/flow", "--expect", flowRev, "--file", f.path("plain-flow", keWrBytes("decisions", "flow", "# Plain\n")), home, "--json")

	// The store's refusals: exits 6, 7 and 1, with the JSON object on stderr.
	refuse("stale rev", 6, conflict, "", "owner", "ana", "decisions/x", "--expect", "0000000000000000", "--file", xNew, home)
	refuse("stale rev json", 6, conflict, "", "owner", "ana", "decisions/x", "--expect", "0000000000000000", "--file", xNew, home, "--json")
	refuse("none on an existing note", 6, conflict, "", "owner", "ana", "decisions/x", "--expect", "none", "--file", xNew, home)
	refuse("a rev on a missing note", 6, `{"error":"conflict","rev":"none"}`+"\n", "", "owner", "ana", "decisions/gone", "--expect", rev1, "--file", f.path("gone", keWrBytes("decisions", "gone", "# G\n")), home)
	refuse("machines", 7, notVersioned, "", "owner", "ana", "machines/x", "--expect", "none", "--file", f.path("machines", keWrBytes("machines", "x", "# M\n")), home)
	refuse("machines json", 7, notVersioned, "", "owner", "ana", "machines/x", "--expect", "none", "--file", f.path("machines", keWrBytes("machines", "x", "# M\n")), home, "--json")
	refuse("a note placed by hand", 7, notVersioned, "", "owner", "ana", "decisions/y", "--expect", brain.NoteRev(hand), "--file", f.path("y2", keWrBytes("decisions", "y", "# Y2\n")), home)
	refuse("an unregistered project", 1, "brain: ", "", "owner", "ana", "decisions/x", "--expect", rev1, "--file", xNew, "--project", "no-such-project")
	// The store is resolved before the input is read: an unregistered project is
	// exit 1 whatever --file holds, as the mount's unknown-project walk needs.
	refuse("an unregistered project with a missing file", 1, "brain: ", "", "owner", "ana", "decisions/x", "--expect", "none", "--file", filepath.Join(f.files, "absent.md"), "--project", "no-such-project")
	refuse("an unregistered project with empty stdin", 1, "brain: ", "", "owner", "ana", "decisions/x", "--expect", "none", "--file", "-", "--project", "no-such-project")
	refuse("an unregistered project with a file that is not a note", 1, "brain: ", "", "owner", "ana", "decisions/x", "--expect", "none", "--file", f.path("not-a-note", []byte("plain text\n")), "--project", "no-such-project")
	refuse("name held by another topic", 9, `{"error":"refused","reason":"note name \"x\" already belongs to topic decisions"}`+"\n", "", "owner", "ana", "notes/x", "--expect", "none", "--file", f.path("held", keWrBytes("notes", "x", "# H\n")), home)

	// The roles that write: the empty role, owner and member each replace x in turn.
	curRev := rev1
	for i, role := range []string{"", "owner", "member"} {
		next := keWrBytes("decisions", "x", fmt.Sprintf("# X\n\nRound %d.\n", i))
		ok(t, fmt.Sprintf("edited decisions/x v%d rev %s\n", i+2, brain.NoteRev(next)), role, "ana", "decisions/x", "--expect", curRev, "--file", f.path("round", next), home)
		curRev = brain.NoteRev(next)
	}
	// The empty role and the owner replace a workflow note, as the owner did above.
	ok(t, "edited decisions/flow v3 rev "+brain.NoteRev(flow1)+"\n", "", "", "decisions/flow", "--expect", flowRev, "--file", f.path("flow1", flow1), home)

	// Who is named: the person the host runs the plugin for, else the session.
	who := func(name, member string) (string, []byte) {
		data := keWrBytes("decisions", name, "# "+name+"\n")
		ok(t, "created decisions/"+name+" v1 rev "+brain.NoteRev(data)+"\n", "member", member, "decisions/"+name, "--expect", "none", "--file", f.path(name, data), home, "--reason", " first   draft ")
		var line string
		for _, l := range keWrLog(t, f.env.GridHome) {
			if strings.Contains(l, "| created | "+name+" |") {
				line = l
			}
		}
		disk, err := os.ReadFile(filepath.Join(f.env.GridHome, "knowledge", "decisions", name+".md"))
		if err != nil || !bytes.Equal(disk, data) {
			t.Fatalf("%s on disk: %q, %v", name, disk, err)
		}
		return line, disk
	}
	line, disk := who("byperson", "bo")
	if !regexp.MustCompile(`\| created \| byperson \| person:bo \| first draft \| decisions \| v1 \| sha256:[0-9a-f]{16}$`).MatchString(line) {
		t.Fatalf("log line for a member: %q", line)
	}
	if n, err := brain.Parse(disk); err != nil || n.Origin != (brain.Origin{Kind: "session", ID: "s0"}) || strings.Contains(string(disk), "bo") {
		t.Fatalf("front matter names a person: %+v %v", n.Origin, err)
	}
	for _, member := range []string{"", "no good!", "../x"} {
		line, _ := who("bysession"+fmt.Sprint(len(member)), member)
		if !regexp.MustCompile(`\| created \| bysession\d+ \| session:[0-9a-f]{24} \|`).MatchString(line) || strings.Contains(line, "person:") {
			t.Fatalf("log line for member %q: %q", member, line)
		}
	}

	// --file - reads stdin; --json prints the result.
	data := keWrBytes("decisions", "viastdin", "# Stdin\n")
	if code := f.run("member", "bo", string(data), "decisions/viastdin", "--expect", "none", "--file", "-", home, "--json"); code != 0 || f.err.Len() != 0 {
		t.Fatalf("stdin write: exit %d, stderr %q", code, f.err.String())
	}
	dec := json.NewDecoder(&f.out)
	dec.DisallowUnknownFields()
	var res brain.WriteResult
	if err := dec.Decode(&res); err != nil || res != (brain.WriteResult{Status: "created", Version: 1, Rev: brain.NoteRev(data)}) {
		t.Fatalf("json result: %+v %v", res, err)
	}
	// Under the lock a later replace by another writer is a conflict with the rev.
	if code := f.run("owner", "ana", "", "decisions/viastdin", "--expect", "none", "--file", f.path("again", data), home, "--json"); code != 6 ||
		f.err.String() != `{"error":"conflict","rev":"`+brain.NoteRev(data)+`"}`+"\n" {
		t.Fatalf("second create: exit %d, stderr %q", code, f.err.String())
	}
}
