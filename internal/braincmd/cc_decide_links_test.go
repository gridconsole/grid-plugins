package braincmd

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

const (
	ccLkJob   = `{"to":"job:T-01/J-7","rel":"decided-by","summary":"The curator Job that filed this note","why":"Shows which Job judged and filed the proposal"}`
	ccLkOther = `{"to":"brain:project/other","rel":"related","summary":"A note on the same checks","why":"Read it next to this note for the full checks routine"}`
	ccLkC     = `{"to":"brain:project/third","rel":"related","summary":"A third note on checks","why":"Explains how the checks are run in CI"}`
)

func ccLkDecide(env Env, args ...string) (int, string) {
	var out, errb bytes.Buffer
	env.Stdout = &out
	env.Stderr = &errb
	return Run(append([]string{"decide", "T-01"}, args...), env), errb.String()
}

func ccLkRead(t *testing.T, path string) brain.Note {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	n, err := brain.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func ccLkLinks(t *testing.T, raws ...string) []brain.Link {
	t.Helper()
	var l linkFlags
	for _, r := range raws {
		if err := l.Set(r); err != nil {
			t.Fatal(err)
		}
	}
	return l
}

func ccLkNotePath(env Env) string {
	return filepath.Join(env.GridHome, "projects", "sample", "knowledge", "conventions", "lesson.md")
}

func TestCcLkAcceptAddsLinks(t *testing.T) {
	env, _ := decideFixture(t)
	if code, stderr := ccLkDecide(env, "lesson", "accept", "--link", ccLkJob, "--link", ccLkOther, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	n := ccLkRead(t, ccLkNotePath(env))
	want := ccLkLinks(t, ccLkJob, ccLkOther)
	if !reflect.DeepEqual(n.Links, want) {
		t.Fatalf("links %+v, want %+v", n.Links, want)
	}
	if n.Links[0].Summary == "" || n.Links[0].Why == "" || n.Links[1].Why == "" {
		t.Fatalf("a link lost its summary or why: %+v", n.Links)
	}
	lines := strings.Split(strings.TrimRight(n.Body, "\n"), "\n")
	if last := lines[len(lines)-1]; !strings.HasPrefix(last, "Source: [T-01](") {
		t.Fatalf("body does not end with the Source line: %q", last)
	}
}

func TestCcLkRefusesBadLink(t *testing.T) {
	cases := []struct{ name, link, want string }{
		{"empty why", `{"to":"job:T-01/J-7","rel":"decided-by","summary":"The curator Job","why":""}`, "link why must explain the benefit in 8 to 160 characters"},
		{"short why", `{"to":"job:T-01/J-7","rel":"decided-by","summary":"The curator Job","why":"short"}`, "link why must explain the benefit in 8 to 160 characters"},
		{"see also", `{"to":"job:T-01/J-7","rel":"decided-by","summary":"The curator Job","why":"see also"}`, "link why must explain the benefit in 8 to 160 characters"},
		{"task target", `{"to":"task:T-01","rel":"decided-by","summary":"The task","why":"Shows which task asked for this"}`, "link target must be brain:<level>/<id>, job:<task>/<job> or person:<id>, optionally with #anchor"},
		{"unknown rel", `{"to":"job:T-01/J-7","rel":"loves","summary":"The curator Job","why":"Shows which Job filed this"}`, "unknown link relation"},
		{"unknown field", `{"to":"job:T-01/J-7","rel":"decided-by","summary":"The curator Job","why":"Shows which Job filed this","extra":"x"}`, "unknown field"},
	}
	for _, c := range cases {
		for _, action := range []string{"accept", "merge-into"} {
			t.Run(c.name+"/"+action, func(t *testing.T) {
				env, dir := decideFixture(t)
				before := snapshotFiles(t, env.GridHome)
				args := []string{"lesson", action}
				if action == "merge-into" {
					args = append(args, "brain:project/target")
				}
				code, stderr := ccLkDecide(env, append(args, "--link", ccLkJob, "--link", c.link)...)
				if code != 2 {
					t.Fatalf("exit %d, want 2: %s", code, stderr)
				}
				if !strings.Contains(stderr, c.want) {
					t.Fatalf("stderr %q lacks %q", stderr, c.want)
				}
				if after := snapshotFiles(t, env.GridHome); !reflect.DeepEqual(before, after) {
					t.Fatal("the Grid home changed")
				}
				if n := ccLkRead(t, filepath.Join(dir, "brain-proposals", "lesson.md")); n.Status != "proposed" {
					t.Fatalf("proposal status %q", n.Status)
				}
				if code := Run([]string{"keep", "--task", "T-01", "--json"}, Env{GridHome: env.GridHome, Cwd: env.Cwd, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}); code != 0 {
					t.Fatalf("keep exit %d", code)
				}
			})
		}
	}
}

func TestCcLkMergeKeepsBoth(t *testing.T) {
	env, dir := decideFixture(t)
	own := ccLkLinks(t, `{"to":"brain:project/origin","rel":"related","summary":"Where the target came from","why":"Gives the first version of the target"}`)[0]
	mine := ccLkLinks(t, `{"to":"person:sam","rel":"related","summary":"Who asked for the checks","why":"Sam owns the checks and can say more"}`)[0]
	s := brain.Store{GridHome: env.GridHome, Project: "sample"}
	if _, err := s.Propose(brain.Note{Name: "target", Topic: "conventions", Description: "When testing", Summary: "Original", Source: "T-00", Level: "project", Body: "Original body.", Links: []brain.Link{own}}, brain.Origin{Kind: "session", ID: "example"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Accept("project", "target"); err != nil {
		t.Fatal(err)
	}
	p := ccLkRead(t, filepath.Join(dir, "brain-proposals", "lesson.md"))
	p.Links = []brain.Link{mine}
	b, err := brain.Encode(p)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "brain-proposals", "lesson.md"), b, 0600); err != nil {
		t.Fatal(err)
	}
	if code, stderr := ccLkDecide(env, "lesson", "merge-into", "brain:project/target", "--link", ccLkC, "--link", ccLkC, "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	n := ccLkRead(t, filepath.Join(env.GridHome, "projects", "sample", "knowledge", "conventions", "target.md"))
	want := append([]brain.Link{own, mine}, ccLkLinks(t, ccLkC)...)
	if !reflect.DeepEqual(n.Links, want) {
		t.Fatalf("links %+v, want %+v", n.Links, want)
	}
}

func TestCcLkRejectIgnoresLinks(t *testing.T) {
	rejected := func(extra ...string) (brain.Note, []string) {
		env, dir := decideFixture(t)
		if code, stderr := ccLkDecide(env, append([]string{"lesson", "reject", "--note", "Already covered"}, extra...)...); code != 0 {
			t.Fatalf("exit %d: %s", code, stderr)
		}
		var paths []string
		for p := range snapshotFiles(t, env.GridHome) {
			rel, err := filepath.Rel(env.GridHome, p)
			if err != nil {
				t.Fatal(err)
			}
			paths = append(paths, rel)
		}
		sort.Strings(paths)
		n := ccLkRead(t, filepath.Join(dir, "brain-proposals", "lesson.md"))
		n.Updated = ""
		return n, paths
	}
	plain, plainPaths := rejected()
	linked, linkedPaths := rejected("--link", ccLkJob)
	if plain.Status != "rejected" {
		t.Fatalf("status %q", plain.Status)
	}
	a, _ := brain.Encode(plain)
	b, _ := brain.Encode(linked)
	if !bytes.Equal(a, b) {
		t.Fatalf("rejected note differs:\n%s\n%s", a, b)
	}
	if !reflect.DeepEqual(plainPaths, linkedPaths) {
		t.Fatalf("files differ: %v vs %v", plainPaths, linkedPaths)
	}
}
