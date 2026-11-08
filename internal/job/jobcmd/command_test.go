package jobcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

func fixture(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	h := jobtest.New(t, filepath.Join("..", "..", "..", "testdata"))
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	return Env{Cwd: filepath.Dir(h.Task), GridHome: h.GridHome, Machine: jobtest.Machine, Stdout: out, Stderr: errs}, out, errs
}
func call(t *testing.T, e Env, out, errs *bytes.Buffer, want int, args ...string) report {
	t.Helper()
	out.Reset()
	errs.Reset()
	got := Run(args, e)
	if got != want {
		t.Fatalf("%v: exit %d want %d; stdout %s stderr %s", args, got, want, out, errs)
	}
	var r report
	if strings.Contains(strings.Join(args, " "), "--json") {
		if err := json.Unmarshal(out.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
	}
	return r
}
func TestRenderLintShow(t *testing.T) {
	e, out, errs := fixture(t)
	r := call(t, e, out, errs, 0, "render", "task", "1", "--json")
	if !r.OK || r.ID != "J-1" || r.Path == "" || r.Inputs == "" {
		t.Fatalf("%+v", r)
	}
	first, e2 := os.ReadFile(r.Path)
	if e2 != nil {
		t.Fatal(e2)
	}
	call(t, e, out, errs, 0, "render", "task", "J-1", "--json")
	got := call(t, e, out, errs, 0, "lint", r.Path, "--json")
	if !got.OK || len(got.Findings) != 0 {
		t.Fatalf("%+v", got)
	}
	show := call(t, e, out, errs, 0, "show", "task", "J-1", "--json")
	if show.Job == nil || show.Brief != string(first) {
		t.Fatal("show does not match saved brief")
	}
	call(t, e, out, errs, 0, "show", r.Path)
	if out.String() != string(first) {
		t.Fatal("plain show differs")
	}
	e.Cwd = filepath.Join(e.Cwd, "task")
	call(t, e, out, errs, 0, "show", "--json")
	call(t, e, out, errs, 0, "show", "J-1", "--json")
	call(t, e, out, errs, 0, "lint", r.Path)
	if !strings.Contains(out.String(), "ok: true") {
		t.Fatal("missing plain status")
	}
}
func TestRenderRefusesInvalidAndChangedJobs(t *testing.T) {
	e, out, errs := fixture(t)
	plan := filepath.Join(e.Cwd, "task", "plan.md")
	original, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(original), "on_blocked: ask-and-continue", "on_blocked: wait", 1)
	if err = os.WriteFile(plan, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	r := call(t, e, out, errs, 1, "render", "task", "1", "--json")
	if len(r.Findings) == 0 {
		t.Fatal("missing findings")
	}
	if _, err = os.Stat(filepath.Join(e.Cwd, "task", "jobs")); !os.IsNotExist(err) {
		t.Fatal("invalid render created artifacts")
	}
	if err = os.WriteFile(plan, original, 0644); err != nil {
		t.Fatal(err)
	}
	r = call(t, e, out, errs, 0, "render", "task", "1", "--json")
	before, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(e.GridHome, "projects", jobtest.Project, "knowledge", "conventions", "export-format.md")
	f, err := os.OpenFile(note, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteString("A changed constraint.\n"); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	rejected := call(t, e, out, errs, 1, "render", "task", "1", "--json")
	if !strings.Contains(rejected.Error, "different saved inputs") {
		t.Fatalf("%+v", rejected)
	}
	after, err := os.ReadFile(r.Path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("changed existing bundle")
	}
}
func TestArgumentAndLintFailures(t *testing.T) {
	e, out, errs := fixture(t)
	for _, args := range [][]string{{"render"}, {"render", "task", "1", "--budget", "0"}, {"lint"}, {"show", "a", "b", "c"}, {"unknown"}, {"lint", "a", "--unknown"}} {
		call(t, e, out, errs, 2, append(args, "--json")...)
	}
	call(t, e, out, errs, 1, "render", "task", "999", "--json")
	call(t, e, out, errs, 1, "show", "task", "J-999", "--json")
	path := filepath.Join(e.Cwd, "broken.md")
	if err := os.WriteFile(path, []byte("broken\n"), 0644); err != nil {
		t.Fatal(err)
	}
	r := call(t, e, out, errs, 1, "lint", path, "--json")
	if len(r.Findings) != 1 || r.Findings[0].Rule != "brief.frontmatter" {
		t.Fatalf("%+v", r)
	}
}
func TestConcurrentIdenticalRender(t *testing.T) {
	e, _, _ := fixture(t)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			local := e
			out, errs := &bytes.Buffer{}, &bytes.Buffer{}
			local.Stdout, local.Stderr = out, errs
			if code := Run([]string{"render", "task", "1", "--json"}, local); code != 0 {
				t.Errorf("exit %d: %s %s", code, out, errs)
			}
		}()
	}
	wg.Wait()
	b, err := os.ReadFile(filepath.Join(e.Cwd, "task", "jobs", "J-1", "inputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(b) {
		t.Fatal("partial inputs published")
	}
}
func TestIndexGenerationRecorded(t *testing.T) {
	e, out, errs := fixture(t)
	store := brain.Store{GridHome: e.GridHome, Project: jobtest.Project}
	idx, err := store.RebuildIndex()
	if err != nil || idx.Generations["project"] == 0 {
		t.Fatalf("%+v %v", idx, err)
	}
	r := call(t, e, out, errs, 0, "render", "task", "1", "--json")
	b, err := os.ReadFile(r.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	var in struct {
		Generation string `json:"generation"`
	}
	if err = json.Unmarshal(b, &in); err != nil {
		t.Fatal(err)
	}
	if want := fmt.Sprintf(`{"project":%d}`, idx.Generations["project"]); in.Generation != want {
		t.Fatal(in.Generation)
	}
}

func render(t *testing.T, e Env, out, errs *bytes.Buffer) (brief, inputs string) {
	t.Helper()
	r := call(t, e, out, errs, 0, "render", "task", "1", "--json")
	b, err := os.ReadFile(r.Path)
	if err != nil {
		t.Fatal(err)
	}
	in, err := os.ReadFile(r.Inputs)
	if err != nil {
		t.Fatal(err)
	}
	return string(b), string(in)
}

func TestOwnerRoleDecidesTheTierOfTheIntent(t *testing.T) {
	for role, want := range map[string]contract.TrustTier{"member": contract.PersonTeam, "guest": contract.PersonGuest, "": contract.External} {
		e, out, errs := fixture(t)
		line := "owner: " + role + "\n"
		if role == "" {
			line = ""
		}
		if err := os.WriteFile(filepath.Join(e.Cwd, "task", "task.md"), []byte("---\nid: T-01\n"+line+"---\n# Export documentation\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		_, inputs := render(t, e, out, errs)
		var in struct {
			Sources []struct {
				Pointer string             `json:"pointer"`
				Tier    contract.TrustTier `json:"tier"`
			} `json:"sources"`
		}
		if err := json.Unmarshal([]byte(inputs), &in); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, src := range in.Sources {
			if src.Pointer == "intent.md#request" {
				found = true
				if src.Tier != want {
					t.Errorf("owner %q: intent is %q, want %q", role, src.Tier, want)
				}
			}
		}
		if !found {
			t.Errorf("owner %q: intent is not in the inputs", role)
		}
	}
}

func TestPersonalNoteStaysOutOfSavedFiles(t *testing.T) {
	e, out, errs := fixture(t)
	store := brain.Store{GridHome: e.GridHome, Project: jobtest.Project}
	n := brain.Note{Name: "my-habits", Topic: "decisions", Type: "decision", Description: "HOME-SUMMARY-MARKER", Summary: "HOME-SUMMARY-MARKER", Source: "fixture", Level: "home", Body: "HOME-BODY-MARKER\n"}
	if _, err := store.Propose(n, brain.Origin{Kind: "session", ID: "me", Tier: contract.PersonTeam}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Accept("home", "my-habits"); err != nil {
		t.Fatal(err)
	}
	plan := filepath.Join(e.Cwd, "task", "plan.md")
	b, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(b), "keep: [brain:project/export-format]", "keep: [brain:project/export-format, brain:home/my-habits]", 1)
	if edited == string(b) {
		t.Fatal("fixture keep line changed")
	}
	if err = os.WriteFile(plan, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	brief, inputs := render(t, e, out, errs)
	for _, text := range []string{brief, inputs} {
		if strings.Contains(text, "HOME-BODY-MARKER") || strings.Contains(text, "HOME-SUMMARY-MARKER") {
			t.Fatalf("personal note copied into a synced file:\n%s", text)
		}
		if !strings.Contains(text, "brain:home/my-habits") {
			t.Fatal("pointer to the personal note is missing")
		}
	}
}
