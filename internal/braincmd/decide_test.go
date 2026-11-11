package braincmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func decideFixture(t *testing.T) (Env, string) {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(gridhome.TasksDir(home, "sample"), "T-01-example")
	if err := project.Write(home, project.Record{Name: "sample", ID: "project-example"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "brain-proposals"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string]string{"task.md": "---\nid: T-01\ntitle: Example\nproject: sample\nstate: open\ncreated: 2026-11-08T00:00:00Z\n---\n", "intent.md": "Example\n"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	n := brain.Note{Name: "lesson", Topic: "conventions", Type: "reference", Description: "When checking changes", Summary: "Run the checks", Source: "T-01", Level: "project", Status: "proposed", Origin: brain.Origin{Kind: "session", ID: "example"}, Body: "Run checks before landing.\n"}
	b, err := brain.Encode(n)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "brain-proposals", "lesson.md"), b, 0600); err != nil {
		t.Fatal(err)
	}
	return Env{GridHome: home, Cwd: dir}, dir
}

func TestTaskDecisions(t *testing.T) {
	for _, action := range []string{"accept", "reject", "merge-into"} {
		t.Run(action, func(t *testing.T) {
			env, dir := decideFixture(t)
			var out, stderr bytes.Buffer
			env.Stdout = &out
			env.Stderr = &stderr
			s := brain.Store{GridHome: env.GridHome, Project: "sample"}
			if action == "merge-into" {
				_, err := s.Propose(brain.Note{Name: "target", Topic: "conventions", Description: "When testing", Summary: "Original", Source: "T-00", Level: "project", Body: "Original body."}, brain.Origin{Kind: "session", ID: "example"})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.Accept("project", "target"); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"decide", "T-01", "lesson", action, "brain:project/target", "--note", "Reviewed", "--json"}
			var firstIndex []byte
			for i := 0; i < 2; i++ {
				out.Reset()
				if code := Run(args, env); code != 0 {
					t.Fatalf("code %d: %s", code, &stderr)
				}
				if action != "reject" {
					index, err := os.ReadFile(filepath.Join(env.GridHome, "projects", "sample", "knowledge", "backlinks.json"))
					if err != nil {
						t.Fatal(err)
					}
					if i == 0 {
						firstIndex = index
					} else if !bytes.Equal(firstIndex, index) {
						t.Fatal("retry rewrote the index generation")
					}
				}
			}
			var result brain.TaskDecision
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			result.At = "TIME"
			b, _ := json.Marshal(result)
			golden, err := os.ReadFile(filepath.Join("testdata", "decide-"+action+".json"))
			if err != nil {
				t.Fatal(err)
			}
			if string(b)+"\n" != string(golden) {
				t.Fatalf("got %s", b)
			}
			history, err := os.ReadFile(filepath.Join(dir, "history.md"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Count(string(history), "brain decided") != 1 {
				t.Fatalf("history duplicated: %s", history)
			}
			entry, err := s.Show("brain:project/target")
			if action == "reject" {
				if err == nil {
					t.Fatal("rejection wrote a note")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if strings.Count(entry.Note.Body, "Run checks before landing.") != 1 || !strings.Contains(entry.Note.Body, "[T-01]") {
					t.Fatalf("bad linked note: %s", entry.Note.Body)
				}
			}
			args[3] = "reject"
			if action == "reject" {
				args[3] = "accept"
			}
			if Run(args, env) == 0 {
				t.Fatal("conflicting decision accepted")
			}
		})
	}
}

func TestDecideRefusesUnsafeProposals(t *testing.T) {
	for _, kind := range []string{"external", "config", "traversal"} {
		t.Run(kind, func(t *testing.T) {
			env, dir := decideFixture(t)
			env.Stdout = &bytes.Buffer{}
			env.Stderr = &bytes.Buffer{}
			path := filepath.Join(dir, "brain-proposals", "lesson.md")
			b, _ := os.ReadFile(path)
			n, _ := brain.Parse(b)
			if kind == "external" {
				n.Origin.Kind = "import"
			}
			if kind == "config" {
				n.Topic = "deploy"
			}
			b, _ = brain.Encode(n)
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Fatal(err)
			}
			name := "lesson"
			if kind == "traversal" {
				name = "../lesson"
			}
			if Run([]string{"decide", "T-01", name, "accept", "target"}, env) == 0 {
				t.Fatal("unsafe proposal accepted")
			}
		})
	}
}

func TestDecideRecoversAfterHistoryFailure(t *testing.T) {
	env, dir := decideFixture(t)
	env.Stdout = &bytes.Buffer{}
	env.Stderr = &bytes.Buffer{}
	// A directory in place of history prevents completion after publication.
	if err := os.Mkdir(filepath.Join(dir, "history.md"), 0700); err != nil {
		t.Fatal(err)
	}
	args := []string{"decide", "T-01", "lesson", "accept", "target"}
	if Run(args, env) == 0 {
		t.Fatal("expected history failure")
	}
	if err := os.Remove(filepath.Join(dir, "history.md")); err != nil {
		t.Fatal(err)
	}
	if Run(args, env) != 0 {
		t.Fatalf("recovery failed: %s", env.Stderr)
	}
	s := brain.Store{GridHome: env.GridHome, Project: "sample"}
	e, err := s.Show("brain:project/target")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(e.Note.Body, "Run checks before landing.") != 1 {
		t.Fatal("recovery duplicated content")
	}
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil || strings.Count(string(b), "brain decided") != 1 {
		t.Fatalf("history %s: %v", b, err)
	}
}

// snapshotFiles maps every file below dir to its bytes, locks aside.
func snapshotFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(p, ".lock") {
			return err
		}
		b, err := os.ReadFile(p)
		out[p] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

const decideMark = "Run checks before landing."

// A proposal the check refuses is refused by decide as it is by accept: exit 3
// with the check_failed JSON, and no receipt, entry, version, audit line,
// status change or history line.
func TestDecideRefusedByCheckWritesNothing(t *testing.T) {
	for _, action := range []string{"accept", "merge-into"} {
		t.Run(action, func(t *testing.T) {
			failMark(t, decideMark)
			env, dir := decideFixture(t)
			var out, stderr bytes.Buffer
			env.Stdout = &out
			env.Stderr = &stderr
			s := brain.Store{GridHome: env.GridHome, Project: "sample"}
			if action == "merge-into" {
				if _, err := s.Propose(brain.Note{Name: "target", Topic: "conventions", Description: "When testing", Summary: "Original", Source: "T-00", Level: "project", Body: "Original body."}, brain.Origin{Kind: "session", ID: "example"}); err != nil {
					t.Fatal(err)
				}
				if _, err := s.Accept("project", "target"); err != nil {
					t.Fatal(err)
				}
			}
			before := snapshotFiles(t, env.GridHome)
			args := []string{"decide", "T-01", "lesson", action, "brain:project/target", "--json"}
			if code := Run(args, env); code != 3 {
				t.Fatalf("json refusal: %d %s %s", code, &out, &stderr)
			}
			if out.Len() != 0 {
				t.Fatalf("stdout should be empty: %s", &out)
			}
			var got struct {
				Error    string `json:"error"`
				Code     string `json:"code"`
				Findings []struct {
					Code string `json:"code"`
					Text string `json:"text"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(stderr.Bytes(), &got); err != nil || got.Error != "check_failed" || got.Code != "check_failed" || len(got.Findings) != 1 || got.Findings[0].Code != "test_finding" || got.Findings[0].Text != decideMark {
				t.Fatalf("json: %v %+v %s", err, got, &stderr)
			}
			if _, err := os.Stat(filepath.Join(dir, "brain-proposals", ".decisions", "lesson.json")); err == nil {
				t.Fatal("the refused decision left a receipt")
			}
			after := snapshotFiles(t, env.GridHome)
			for p, b := range before {
				if a, ok := after[p]; !ok || a != b {
					t.Fatalf("%s changed or vanished", p)
				}
			}
			for p := range after {
				if _, ok := before[p]; !ok {
					t.Fatalf("%s was created", p)
				}
			}

			out.Reset()
			stderr.Reset()
			if code := Run(args[:len(args)-1], env); code != 3 || out.Len() != 0 || !strings.Contains(stderr.String(), "test_finding") {
				t.Fatalf("plain refusal: %d %s %s", code, &out, &stderr)
			}

			// Reject of the same proposal is never gated.
			out.Reset()
			stderr.Reset()
			if code := Run([]string{"decide", "T-01", "lesson", "reject", "brain:project/target"}, env); code != 0 {
				t.Fatalf("reject: %d %s", code, &stderr)
			}
		})
	}
}

// The same decision with the check passing publishes as before.
func TestDecidePassingCheckStillPublishes(t *testing.T) {
	failMark(t, "NOT IN THIS PROPOSAL")
	env, _ := decideFixture(t)
	var out, stderr bytes.Buffer
	env.Stdout = &out
	env.Stderr = &stderr
	if code := Run([]string{"decide", "T-01", "lesson", "accept", "brain:project/target", "--json"}, env); code != 0 {
		t.Fatalf("decide: %d %s", code, &stderr)
	}
	s := brain.Store{GridHome: env.GridHome, Project: "sample"}
	if e, err := s.Show("brain:project/target"); err != nil || e.Note.Status != "accepted" || !strings.Contains(e.Note.Body, decideMark) {
		t.Fatalf("entry: %+v %v", e.Note.Status, err)
	}
}
