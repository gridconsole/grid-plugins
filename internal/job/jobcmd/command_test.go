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
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func fixture(t *testing.T) (Env, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	h := jobtest.New(t, filepath.Join("..", "..", "..", "testdata"))
	out, errs := &bytes.Buffer{}, &bytes.Buffer{}
	return Env{Resolver: ownerResolver(h.GridHome), Cwd: filepath.Dir(h.Task), GridHome: h.GridHome, Machine: jobtest.Machine, Stdout: out, Stderr: errs}, out, errs
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
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
	if !r.OK || r.ID != "J-1" || r.Path == "" || r.Inputs == "" {
		t.Fatalf("%+v", r)
	}
	first, e2 := os.ReadFile(r.Path)
	if e2 != nil {
		t.Fatal(e2)
	}
	call(t, e, out, errs, 0, "render", "T-01", "J-1", "--json")
	got := call(t, e, out, errs, 0, "lint", r.Path, "--json")
	if !got.OK || len(got.Findings) != 0 {
		t.Fatalf("%+v", got)
	}
	show := call(t, e, out, errs, 0, "show", "T-01", "J-1", "--json")
	if show.Job == nil || show.Brief != string(first) {
		t.Fatal("show does not match saved brief")
	}
	call(t, e, out, errs, 0, "show", r.Path)
	if out.String() != string(first) {
		t.Fatal("plain show differs")
	}
	e.Cwd = filepath.Join(e.Cwd, "T-01")
	call(t, e, out, errs, 0, "show", "--json")
	call(t, e, out, errs, 0, "show", "J-1", "--json")
	call(t, e, out, errs, 0, "lint", r.Path)
	if !strings.Contains(out.String(), "ok: true") {
		t.Fatal("missing plain status")
	}
}
func TestRenderRefusesInvalidAndChangedJobs(t *testing.T) {
	e, out, errs := fixture(t)
	plan := filepath.Join(e.Cwd, "T-01", "plan.md")
	original, err := os.ReadFile(plan)
	if err != nil {
		t.Fatal(err)
	}
	bad := strings.Replace(string(original), "on_blocked: ask-and-continue", "on_blocked: wait", 1)
	if err = os.WriteFile(plan, []byte(bad), 0644); err != nil {
		t.Fatal(err)
	}
	h := jobtest.Home{GridHome: e.GridHome, Task: filepath.Join(e.Cwd, "T-01")}
	h.Approve(t)
	r := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
	if len(r.Findings) == 0 {
		t.Fatal("missing findings")
	}
	if _, err = os.Stat(filepath.Join(e.Cwd, "T-01", "jobs")); !os.IsNotExist(err) {
		t.Fatal("invalid render created artifacts")
	}
	if err = os.WriteFile(plan, original, 0644); err != nil {
		t.Fatal(err)
	}
	h.Approve(t)
	r = call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
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
	rejected := call(t, e, out, errs, 1, "render", "T-01", "1", "--json")
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
	for _, args := range [][]string{{"render"}, {"render", "T-01", "1", "--budget", "0"}, {"lint"}, {"show", "a", "b", "c"}, {"unknown"}, {"lint", "a", "--unknown"}} {
		call(t, e, out, errs, 2, append(args, "--json")...)
	}
	call(t, e, out, errs, 1, "render", "T-01", "999", "--json")
	call(t, e, out, errs, 1, "show", "T-01", "J-999", "--json")
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
			if code := Run([]string{"render", "T-01", "1", "--json"}, local); code != 0 {
				t.Errorf("exit %d: %s %s", code, out, errs)
			}
		}()
	}
	wg.Wait()
	b, err := os.ReadFile(filepath.Join(e.Cwd, "T-01", "jobs", "J-1", "inputs.json"))
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
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
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
	r := call(t, e, out, errs, 0, "render", "T-01", "1", "--json")
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
		path := filepath.Join(e.Cwd, "T-01", "task.md")
		metadata, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(metadata), "owner: member\n", line, 1)), 0o644); err != nil {
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

// A task made with grid task new is the person's own: job render quotes the
// request as the owner's tier, and the contract pointer intent.md#request resolves.
func TestTaskNewThenRenderTrustsTheRequest(t *testing.T) {
	plan, err := os.ReadFile(filepath.Join("..", "..", "..", "testdata", "job", "task", "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's brain note and research file are not in a fresh Grid home.
	text := string(plan)
	for old, now := range map[string]string{
		"      links:\n        - {to: brain:project/export-format, rel: uses, why: \"Keep the documented column order stable for readers\"}\n": "      links: []\n",
		"keep: [brain:project/export-format]": "keep: []",
		"read: [task:research.md#format]":     "read: []",
	} {
		if !strings.Contains(text, old) {
			t.Fatalf("fixture plan lost %q", old)
		}
		text = strings.Replace(text, old, now, 1)
	}
	for _, tc := range []struct {
		name, who, role string
		want            contract.TrustTier
	}{
		{"machine owner", "", "", contract.PersonTeam},
		{"member", "alex", "", contract.PersonTeam},
		{"guest", "sam", "guest", contract.PersonGuest},
		{"an agent role", "builder-1", "builder", contract.External},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			home := filepath.Join(root, "home")
			if err := os.MkdirAll(home, 0o755); err != nil {
				t.Fatal(err)
			}
			store := task.Store{Resolver: where.Resolver{GridHome: filepath.Join(home, ".grid"), Home: home, Ceiling: root}}
			made, err := store.New(task.NewOptions{Cwd: home, Project: where.Personal, Words: "I want the export format documented.", Who: tc.who, Role: tc.role})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(made.Path, "plan.md"), []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := store.Change(task.WriteOptions{Cwd: made.Path, ID: made.ID, Action: "status", State: "waiting"}); err != nil {
				t.Fatal(err)
			}
			(jobtest.Home{GridHome: store.Resolver.GridHome, Task: made.Path}).Approve(t)
			out, errs := &bytes.Buffer{}, &bytes.Buffer{}
			e := Env{Resolver: ownerResolver(filepath.Join(home, ".grid")), Cwd: home, GridHome: filepath.Join(home, ".grid"), Machine: jobtest.Machine, Stdout: out, Stderr: errs}
			r := call(t, e, out, errs, 0, "render", made.Path, "1", "--json")
			file, err := os.ReadFile(r.Path)
			if err != nil {
				t.Fatal(err)
			}
			brief := string(file)
			saved, err := os.ReadFile(r.Inputs)
			if err != nil {
				t.Fatal(err)
			}
			var in struct {
				Sources []struct {
					Pointer string             `json:"pointer"`
					Tier    contract.TrustTier `json:"tier"`
				} `json:"sources"`
			}
			if err := json.Unmarshal(saved, &in); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, src := range in.Sources {
				if src.Pointer == "intent.md#request" {
					found = true
					if src.Tier != tc.want {
						t.Errorf("intent.md#request is %q, want %q", src.Tier, tc.want)
					}
				}
			}
			if !found {
				t.Fatal("intent.md#request did not resolve")
			}
			label := "### Owner [" + string(tc.want) + "] (intent.md#request)"
			if !strings.Contains(brief, label) {
				t.Fatalf("brief lacks %q:\n%s", label, brief)
			}
			if tc.want == contract.External && !strings.Contains(brief, "<data tier=\"external\"") {
				t.Fatalf("an external request must sit inside the data fence:\n%s", brief)
			}
			if tc.want != contract.External && (strings.Contains(brief, "<data tier=\"person-team\"") || !strings.Contains(brief, "I want the export format documented.")) {
				t.Fatalf("the owner's own words are quoted, not fenced:\n%s", brief)
			}
		})
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
	plan := filepath.Join(e.Cwd, "T-01", "plan.md")
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
	(jobtest.Home{GridHome: e.GridHome, Task: filepath.Join(e.Cwd, "T-01")}).Approve(t)
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
