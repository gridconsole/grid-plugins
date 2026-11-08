package land

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func fixture(t *testing.T) (Store, string, string) {
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
	write(t, filepath.Join(dir, "task.md"), "---\nid: T-01\ntitle: Example\nproject: sample\nstate: waiting\ncreated: 2026-11-08T00:00:00Z\n---\n")
	write(t, filepath.Join(dir, "intent.md"), "Example\n")
	write(t, filepath.Join(dir, "plan.md"), "# Plan\n\n- [ ] 1 Land the change\n  jobs: []\n")
	write(t, filepath.Join(home, "projects", "sample", "knowledge", "deploy", "lands.md"), "---\nlands:\n  target: local\n  branch: main\n  command: printf landed > outcome\n  needs-approve: true\n---\n")
	return Store{Resolver: where.Resolver{GridHome: home, Machine: "machine-example"}}, dir, repo
}
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

// approve records the plan approval the way a person does, through grid approve's writer.
func approve(t *testing.T, s Store) {
	t.Helper()
	if _, err := s.Approve(task.WriteOptions{ID: "T-01"}, ""); err != nil {
		t.Fatal(err)
	}
}

// landJSON is the record the first L27 branch trusted; any agent could write it.
const landJSON = `{"plan_hash":"%s","approved":true,"tier":"person-team"}`

func TestApprovalBoundary(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, s Store, dir string)
		allow bool
	}{
		{"missing", func(*testing.T, Store, string) {}, false},
		{"approved", func(t *testing.T, s Store, _ string) { approve(t, s) }, true},
		{"stale", func(t *testing.T, s Store, dir string) {
			approve(t, s)
			write(t, filepath.Join(dir, "plan.md"), "# Plan\n\n- [ ] 1 Land another change\n  jobs: []\n")
		}, false},
		// Regression: an agent-written approvals/land.json with the current hash never lands.
		{"forged-land-json", func(t *testing.T, s Store, dir string) {
			text, err := plan.Store{TaskDir: dir}.Read()
			if err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(dir, "plan.hash"), plan.Hash(text)+"\n")
			write(t, filepath.Join(dir, "approvals", "land.json"), fmt.Sprintf(landJSON, plan.Hash(text)))
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, dir, repo := fixture(t)
			tc.setup(t, s, dir)
			before, _ := os.ReadFile(filepath.Join(dir, "history.md"))
			r, err := s.Run(context.Background(), repo, "T-01", false)
			if (err == nil) != tc.allow {
				t.Fatalf("result %+v, error %v", r, err)
			}
			b, readErr := os.ReadFile(filepath.Join(repo, "outcome"))
			history, _ := os.ReadFile(filepath.Join(dir, "history.md"))
			if !tc.allow {
				if readErr == nil {
					t.Fatal("command ran without approval")
				}
				if string(history) != string(before) {
					t.Fatal("refusal wrote history")
				}
				return
			}
			if readErr != nil || string(b) != "landed" || r.Status != "landed" {
				t.Fatalf("result %+v, output %s, error %v", r, b, readErr)
			}
			if !strings.Contains(string(history), "land started") || !strings.Contains(string(history), "land landed") {
				t.Fatalf("history %s", history)
			}
		})
	}
}

func TestDryRunGoldenAndNoWrites(t *testing.T) {
	s, dir, repo := fixture(t)
	before := files(t, s.Resolver.GridHome)
	r, err := s.Run(context.Background(), repo, "T-01", true)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	golden, err := os.ReadFile("testdata/dry-run.json")
	if err != nil {
		t.Fatal(err)
	}
	if string(b)+"\n" != string(golden) {
		t.Fatalf("got %s", b)
	}
	if before != files(t, s.Resolver.GridHome) {
		t.Fatal("dry run wrote files")
	}
	if _, err := os.Stat(filepath.Join(dir, "history.md")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote history")
	}
}
func files(t *testing.T, root string) string {
	t.Helper()
	var out strings.Builder
	err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		out.WriteString(rel + "\n")
		if !e.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			out.Write(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestFailureTimeoutAndEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name, command, status string
		timeout               time.Duration
	}{
		{"failure", "printf failure >&2; exit 7", "failed", time.Second},
		{"timeout", "sleep 10", "timed-out", 30 * time.Millisecond},
		{"environment", `printf '%s' "$GRID_LAND_BRANCH" > outcome`, "landed", time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, dir, repo := fixture(t)
			s.Timeout = tc.timeout
			// Quoting through JSON is also valid YAML string syntax.
			command, _ := json.Marshal(tc.command)
			write(t, filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md"), "---\nlands:\n  target: local\n  branch: main\n  command: "+string(command)+"\n  needs-approve: true\n---\n")
			approve(t, s)
			start := time.Now()
			r, err := s.Run(context.Background(), repo, "T-01", false)
			if r.Status != tc.status || (err == nil) != (tc.status == "landed") {
				t.Fatalf("result %+v: %v", r, err)
			}
			if time.Since(start) > 3*time.Second {
				t.Fatal("timeout did not bound execution")
			}
			b, _ := os.ReadFile(filepath.Join(dir, "history.md"))
			if !strings.Contains(string(b), "land "+tc.status) {
				t.Fatalf("result not recorded: %s", b)
			}
			if tc.name == "failure" && (r.Execution.ExitCode != 7 || r.Execution.Stderr != "failure") {
				t.Fatalf("lost failure: %+v", r.Execution)
			}
			if tc.name == "environment" {
				b, _ := os.ReadFile(filepath.Join(repo, "outcome"))
				if string(b) != "main" {
					t.Fatalf("environment %s", b)
				}
			}
		})
	}
}

func TestInvalidDeployAndHash(t *testing.T) {
	for _, kind := range []string{"missing", "unknown-key", "missing-field", "wrong-type", "two-rules", "no-plan", "no-bypass"} {
		t.Run(kind, func(t *testing.T) {
			s, dir, repo := fixture(t)
			topic := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
			b, _ := os.ReadFile(topic)
			switch kind {
			case "missing":
				if err := os.Remove(topic); err != nil {
					t.Fatal(err)
				}
			case "unknown-key":
				write(t, topic, strings.Replace(string(b), "  target:", "  extra: true\n  target:", 1))
			case "missing-field":
				write(t, topic, strings.Replace(string(b), "  branch: main\n", "", 1))
			case "wrong-type":
				write(t, topic, strings.Replace(string(b), "needs-approve: true", "needs-approve: yes", 1))
			case "two-rules":
				write(t, filepath.Join(filepath.Dir(topic), "other.md"), string(b))
			case "no-plan":
				if err := os.Remove(filepath.Join(dir, "plan.md")); err != nil {
					t.Fatal(err)
				}
			case "no-bypass":
				write(t, topic, strings.Replace(string(b), "needs-approve: true", "needs-approve: false", 1))
			}
			_, err := s.Run(context.Background(), repo, "T-01", kind != "no-bypass")
			if err == nil {
				t.Fatal("unsafe land accepted")
			}
		})
	}
}

func TestLandRefusesSymlinkedApproval(t *testing.T) {
	s, dir, repo := fixture(t)
	approve(t, s)
	elsewhere := filepath.Join(s.Resolver.GridHome, "approval.json")
	if err := os.Rename(filepath.Join(dir, "approval.json"), elsewhere); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(elsewhere, filepath.Join(dir, "approval.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := s.Run(context.Background(), repo, "T-01", false); err == nil {
		t.Fatal("followed approval symlink")
	}
	if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
		t.Fatal("command ran")
	}
}

func setWorkflow(t *testing.T, s Store, dir, gate string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "task.md"), strings.Replace(string(b), "state: waiting", "workflow: release\nstate: waiting", 1))
	write(t, filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "workflows", "release.md"), "---\nland-approval: "+gate+"\n---\n")
}

func TestWorkflowApproval(t *testing.T) {
	for _, tc := range []struct {
		gate, approval string
		allow          bool
	}{
		{"plan", "plan", true}, {"plan", "ship", false},
		{"ship", "ship", true}, {"ship", "plan", false},
		{"none", "", true}, {"ship", "", false}, {"typo", "plan", false},
	} {
		t.Run(tc.gate+"/"+tc.approval, func(t *testing.T) {
			s, dir, repo := fixture(t)
			setWorkflow(t, s, dir, tc.gate)
			if tc.approval == "ship" {
				b, _ := os.ReadFile(filepath.Join(dir, "task.md"))
				write(t, filepath.Join(dir, "task.md"), strings.Replace(string(b), "state: waiting", "gate: ship\nstate: waiting", 1))
			}
			if tc.approval != "" {
				approve(t, s)
			}
			_, err := s.Run(context.Background(), repo, "T-01", false)
			if (err == nil) != tc.allow {
				t.Fatalf("allow %v, error %v", tc.allow, err)
			}
			_, err = os.Stat(filepath.Join(repo, "outcome"))
			if (err == nil) != tc.allow {
				t.Fatalf("command execution: %v", err)
			}
		})
	}
}

func TestApprovalBindsLandsRule(t *testing.T) {
	for _, pair := range [][2]string{{"target: local", "target: production"}, {"branch: main", "branch: release"}, {"printf landed > outcome", "printf changed > outcome"}, {"needs-approve: true", "needs-approve: false"}, {"needs-approve: true", "needs-approve: true\n  env: [LAND_TOKEN]"}} {
		t.Run(pair[0], func(t *testing.T) {
			s, _, repo := fixture(t)
			approve(t, s)
			path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
			b, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(b), pair[0], pair[1], 1))
			_, err := s.Run(context.Background(), repo, "T-01", false)
			if !errors.Is(err, ErrRejected) {
				t.Fatalf("expected final rejection, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
				t.Fatal("changed command ran")
			}
		})
	}
}

// Both arms have a real plan, its stored hash, and the forged record. The only
// difference is the authoritative receipt, so accepting land.json cannot pass.
func TestForgedApprovalRequiresAuthoritativeReceipt(t *testing.T) {
	s, dir, repo := fixture(t)
	text, err := plan.Store{TaskDir: dir}.Read()
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "plan.hash"), plan.Hash(text)+"\n")
	write(t, filepath.Join(dir, "approvals", "land.json"), fmt.Sprintf(landJSON, plan.Hash(text)))
	_, err = s.Run(context.Background(), repo, "T-01", false)
	if !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "not been approved") {
		t.Fatalf("expected missing authoritative receipt, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
		t.Fatal("forged approval ran command")
	}
	approve(t, s)
	if _, err := s.Run(context.Background(), repo, "T-01", false); err != nil {
		t.Fatalf("valid control: %v", err)
	}
}

func TestWorkflowDefaultAndInvalidDocuments(t *testing.T) {
	for _, tc := range []struct {
		name, document string
		allow          bool
	}{
		{"default", "---\nname: release\n---\n", true},
		{"wrong type", "---\nland-approval: false\n---\n", false},
		{"empty", "---\nland-approval: ''\n---\n", false},
		{"duplicate", "---\nland-approval: plan\nland-approval: none\n---\n", false},
		{"missing", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, dir, repo := fixture(t)
			setWorkflow(t, s, dir, "plan")
			path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "workflows", "release.md")
			if tc.document == "" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, path, tc.document)
			}
			approve(t, s)
			_, err := s.Run(context.Background(), repo, "T-01", false)
			if (err == nil) != tc.allow {
				t.Fatalf("allow %v: %v", tc.allow, err)
			}
		})
	}
}

func TestUnboundApprovalCannotLand(t *testing.T) {
	s, _, repo := fixture(t)
	// A pre-L61 approval has a correct plan hash but no lands binding.
	if _, err := s.tasks().Approve(task.WriteOptions{ID: "T-01"}, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Run(context.Background(), repo, "T-01", false); !errors.Is(err, ErrRejected) {
		t.Fatalf("unbound receipt: %v", err)
	}
}

func TestHistoryWithoutApprovalAndSymlinkRefusal(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(fmt.Sprint(symlink), func(t *testing.T) {
			s, dir, repo := fixture(t)
			setWorkflow(t, s, dir, "none")
			if symlink {
				other := filepath.Join(repo, "other-history")
				write(t, other, "original\n")
				if err := os.Symlink(other, filepath.Join(dir, "history.md")); err != nil {
					t.Skip(err)
				}
			}
			_, err := s.Run(context.Background(), repo, "T-01", false)
			if symlink {
				if err == nil {
					t.Fatal("symlinked history accepted")
				}
				if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
					t.Fatal("command ran before history failure")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(filepath.Join(dir, "history.md"))
			if err != nil || !strings.Contains(string(b), "land started") || !strings.Contains(string(b), "land landed") {
				t.Fatalf("history %s: %v", b, err)
			}
		})
	}
}
