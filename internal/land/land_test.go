package land

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/project"
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
	write(t, filepath.Join(dir, "task.md"), "---\nid: T-01\ntitle: Example\nproject: sample\nstate: open\ncreated: 2026-11-08T00:00:00Z\n---\n")
	write(t, filepath.Join(dir, "intent.md"), "Example\n")
	write(t, filepath.Join(dir, "plan.hash"), "opaque: revision/one + two\n")
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
func approve(t *testing.T, dir string, a Approval) {
	t.Helper()
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "approvals", "land.json"), string(b))
}

func TestApprovalBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, hash      string
		tier            contract.TrustTier
		approved, allow bool
	}{
		{"missing", "", contract.PersonTeam, true, false},
		{"stale", "old", contract.PersonTeam, true, false},
		{"opaque-match", "opaque: revision/one + two", contract.PersonTeam, true, true},
		{"agent", "opaque: revision/one + two", contract.Agent, true, false},
		{"guest", "opaque: revision/one + two", contract.PersonGuest, true, false},
		{"denied", "opaque: revision/one + two", contract.PersonTeam, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, dir, repo := fixture(t)
			if tc.hash != "" {
				approve(t, dir, Approval{tc.hash, tc.approved, tc.tier})
			}
			r, err := s.Run(context.Background(), repo, "T-01", false)
			if (err == nil) != tc.allow {
				t.Fatalf("result %+v, error %v", r, err)
			}
			b, readErr := os.ReadFile(filepath.Join(repo, "outcome"))
			if !tc.allow {
				if readErr == nil {
					t.Fatal("command ran without approval")
				}
				if _, err := os.Stat(filepath.Join(dir, "history.md")); !os.IsNotExist(err) {
					t.Fatal("refusal wrote history")
				}
				return
			}
			if readErr != nil || string(b) != "landed" || r.Status != "landed" {
				t.Fatalf("result %+v, output %s, error %v", r, b, readErr)
			}
			history, err := os.ReadFile(filepath.Join(dir, "history.md"))
			if err != nil || !strings.Contains(string(history), "land landed") {
				t.Fatalf("history %s, %v", history, err)
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
			approve(t, dir, Approval{"opaque: revision/one + two", true, contract.PersonTeam})
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
	for _, kind := range []string{"missing", "unknown-key", "missing-field", "wrong-type", "two-rules", "empty-hash", "multiline-hash", "no-bypass"} {
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
			case "empty-hash":
				write(t, filepath.Join(dir, "plan.hash"), "\n")
			case "multiline-hash":
				write(t, filepath.Join(dir, "plan.hash"), "first\nsecond\n")
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
	target := filepath.Join(s.Resolver.GridHome, "approval.json")
	write(t, target, `{"plan_hash":"opaque: revision/one + two","approved":true,"tier":"person-team"}`)
	approvalDir := filepath.Join(dir, "approvals")
	if err := os.Mkdir(approvalDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(approvalDir, "land.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := s.Run(context.Background(), repo, "T-01", false); err == nil {
		t.Fatal("followed approval symlink")
	}
	if _, err := os.Stat(filepath.Join(repo, "outcome")); !os.IsNotExist(err) {
		t.Fatal("command ran")
	}
}
