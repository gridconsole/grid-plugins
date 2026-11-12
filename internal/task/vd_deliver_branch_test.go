package task

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// vdBrTree reads every file under root, as the dry-run tests compare them.
func vdBrTree(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		out[rel] = string(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// A dry run with a Branch writes nothing and lists the same actions as without.
func TestVdBrDryRunWritesNothing(t *testing.T) {
	s, task := deliverFixture(t, "")
	root := filepath.Dir(filepath.Dir(filepath.Dir(task.Path)))
	before := vdBrTree(t, root)
	plain, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Done.", By: "grid", DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	withBranch, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Done.", By: "grid", DryRun: true, Branch: "grid/T-01", Base: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if !withBranch.DryRun || len(withBranch.Actions) != len(plain.Actions) || !reflect.DeepEqual(withBranch.Actions, plain.Actions) {
		t.Fatalf("actions differ:\n%v\n%v", plain.Actions, withBranch.Actions)
	}
	if !reflect.DeepEqual(before, vdBrTree(t, root)) {
		t.Fatal("dry run wrote files")
	}
}

// A real delivery with a Branch and Base writes both keys, once, and adds no action.
func TestVdBrDeliverWritesBranchAndBase(t *testing.T) {
	s, task := deliverFixture(t, "branch: old\n")
	d, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Done.", By: "grid", Branch: "grid/T-01", Base: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Actions) != 3 {
		t.Fatalf("actions: %v", d.Actions)
	}
	b, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	front := strings.SplitN(string(b), "---\n", 3)[1]
	if strings.Count(front, "branch:") != 1 || !strings.Contains(front, "branch: grid/T-01\n") || !strings.Contains(front, "base: abc123\n") {
		t.Fatalf("front matter:\n%s", front)
	}
}
