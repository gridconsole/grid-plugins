package braincmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
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
	n := brain.Note{Name: "lesson", Topic: "conventions", Type: "reference", Description: "When checking changes", Summary: "Run the checks", Source: "T-01", Level: "project", Status: "proposed", Origin: brain.Origin{Kind: "session", ID: "example", Tier: contract.Agent}, Body: "Run checks before landing.\n"}
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
				_, err := s.Propose(brain.Note{Name: "target", Topic: "conventions", Description: "When testing", Summary: "Original", Source: "T-00", Level: "project", Body: "Original body."}, brain.Origin{Kind: "session", ID: "example", Tier: contract.Agent})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = s.Accept("project", "target"); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"decide", "T-01", "lesson", action, "brain:project/target", "--note", "Reviewed", "--json"}
			for i := 0; i < 2; i++ {
				out.Reset()
				if code := Run(args, env); code != 0 {
					t.Fatalf("code %d: %s", code, &stderr)
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
				n.Origin.Tier = contract.External
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
