package land_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/cli"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/land"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

// r7Write writes content at path, making its folders.
func r7Write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// r7Fixture is a delivered task T-01 (state waiting) in project sample, whose
// knowledge/deploy folder holds deploy, path by path; nil leaves the folder out.
func r7Fixture(t *testing.T, deploy map[string]string) (land.Store, string, string) {
	t.Helper()
	home := t.TempDir()
	repo := filepath.Join(home, "repo")
	if err := os.Mkdir(repo, 0700); err != nil {
		t.Fatal(err)
	}
	if err := project.Write(home, project.Record{Name: "sample", ID: "example", Folders: []project.Folder{{Machine: "machine-example", Path: repo}}}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(gridhome.TasksDir(home, "sample"), "T-01-example")
	r7Write(t, filepath.Join(dir, "task.md"), "---\nid: T-01\ntitle: Example\nproject: sample\nstate: waiting\ncreated: 2026-11-08T00:00:00Z\n---\n")
	r7Write(t, filepath.Join(dir, "intent.md"), "Example\n")
	r7Write(t, filepath.Join(dir, "plan.md"), "# Plan\n\n- [ ] 1 Land the change\n  jobs: []\n")
	for name, text := range deploy {
		r7Write(t, filepath.Join(home, "projects", "sample", "knowledge", "deploy", name), text)
	}
	return land.Store{Resolver: where.Resolver{GridHome: home, Machine: "machine-example"}}, dir, home
}

func r7History(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

// A ship answer on a project with no deploy topic records that there is nothing
// to land and closes the task on the answer, with no file system error text.
func TestR7ShipAnswerNoDeployTopicClosesTask(t *testing.T) {
	for _, tc := range []struct {
		name   string
		deploy map[string]string
	}{
		{"no knowledge/deploy folder", nil},
		{"knowledge/deploy holds only TOPIC.md", map[string]string{"TOPIC.md": "---\ntitle: Deploy\n---\nDelivery targets and steps\n"}},
		{"knowledge/deploy entry with no lands block", map[string]string{"TOPIC.md": "Deploy\n", "notes.md": "---\ntitle: Notes\n---\nNo lands here.\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, dir, _ := r7Fixture(t, tc.deploy)
			_, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6")
			history := r7History(t, dir)
			if err != nil || !ran {
				t.Fatalf("AfterShip = ran %t, %v; want ran and no error; history:\n%s", ran, err, history)
			}
			if want := " · ship answered · J-6 approved; no deploy topic for project sample, nothing to land · "; strings.Count(history, want) != 1 {
				t.Fatalf("history does not hold exactly one %q line:\n%s", want, history)
			}
			if !strings.Contains(history, "approved after J-6; no deploy topic") {
				t.Fatalf("history has no close note \"approved after J-6; no deploy topic\":\n%s", history)
			}
			if strings.Contains(history, "openat") || strings.Contains(history, "land failed") {
				t.Fatalf("history holds an openat or land failed line:\n%s", history)
			}
			if rec, err := task.Read(dir); err != nil || rec.State != "closed" {
				t.Fatalf("task state %q %v after the answer, want closed", rec.State, err)
			}
			// A second call for the same Job does nothing.
			if _, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6"); err != nil || ran {
				t.Fatalf("second AfterShip = ran %t, %v; want not ran", ran, err)
			}
		})
	}
}

// A deploy entry whose front matter is malformed is still a land failure, and the
// task stays waiting: a broken topic never passes as "no deploy topic".
func TestR7ShipAnswerMalformedDeployTopicFails(t *testing.T) {
	s, dir, _ := r7Fixture(t, map[string]string{"lands.md": "---\nlands:\n  target: [unclosed\n---\n"})
	_, ran, err := s.AfterShip(context.Background(), "", "T-01", "J-6")
	history := r7History(t, dir)
	if err == nil || !ran || strings.Contains(err.Error(), "has no deploy topic") {
		t.Fatalf("AfterShip = ran %t, %v; want a land error for the malformed entry", ran, err)
	}
	if !strings.Contains(history, " · land failed · ") || strings.Contains(history, "nothing to land") {
		t.Fatalf("history does not record the failed land:\n%s", history)
	}
	if rec, err := task.Read(dir); err != nil || rec.State != "waiting" {
		t.Fatalf("task state %q %v after a malformed deploy topic, want waiting", rec.State, err)
	}
}

// grid land by hand on a project with no deploy topic exits non-zero and names
// the missing topic, not the openat text.
func TestR7GridLandNoDeployTopicSaysSo(t *testing.T) {
	for _, tc := range []struct {
		name   string
		deploy map[string]string
	}{
		{"no knowledge/deploy folder", nil},
		{"knowledge/deploy holds only TOPIC.md", map[string]string{"TOPIC.md": "Deploy\n"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, home := r7Fixture(t, tc.deploy)
			var stdout, stderr bytes.Buffer
			env := cli.Env{GridHome: home, Machine: "machine-example", Cwd: gridhome.Real(home), Stdout: &stdout, Stderr: &stderr}
			code := cli.Run([]string{"land", "T-01"}, env)
			if code == 0 {
				t.Fatalf("grid land exit 0, want non-zero; stdout %s", stdout.String())
			}
			if want := "project sample has no deploy topic (knowledge/deploy/<name>.md with a lands: block)"; !strings.Contains(stderr.String(), want) {
				t.Fatalf("grid land stderr %q, want it to name %q", stderr.String(), want)
			}
			if strings.Contains(stderr.String(), "openat") {
				t.Fatalf("grid land stderr holds openat text: %q", stderr.String())
			}
		})
	}
}

// Approve keeps its behaviour: a missing deploy folder is ordinary task approval
// with no lands hash; a folder with no lands entry is still refused.
func TestR7ApproveUnchangedWithoutDeployTopic(t *testing.T) {
	s, _, _ := r7Fixture(t, nil)
	event, err := s.Approve(task.WriteOptions{ID: "T-01"}, "")
	if err != nil || event.LandsHash != "" {
		t.Fatalf("Approve with no deploy folder = %+v, %v; want ordinary approval with no lands hash", event, err)
	}
	s, _, _ = r7Fixture(t, map[string]string{"TOPIC.md": "Deploy\n"})
	if _, err := s.Approve(task.WriteOptions{ID: "T-01"}, ""); err == nil {
		t.Fatal("Approve with a deploy folder and no lands entry passed; it was refused before this lane")
	}
}
