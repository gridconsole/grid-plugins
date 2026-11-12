package task

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// tiOutcomeRead returns the parsed front matter of task.md and the lines of its
// ## Outcome section, from the heading to the next heading or the end.
func tiOutcomeRead(t *testing.T, taskDir string) (map[string]any, []string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(taskDir, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := frontmatter.Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	var section []string
	in := false
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "## ") {
			in = line == "## Outcome"
			continue
		}
		if in {
			section = append(section, line)
		}
	}
	for len(section) > 0 && strings.TrimSpace(section[len(section)-1]) == "" {
		section = section[:len(section)-1]
	}
	return doc.Data, section
}

// tiOutcomeHas reports whether the section holds line exactly.
func tiOutcomeHas(section []string, line string) bool {
	for _, got := range section {
		if got == line {
			return true
		}
	}
	return false
}

// The Proof list names each plan item by its title, with its id, its level and
// the number of files in its evidence; the section still starts with the summary
// and holds ### Proof, and the outcome: key is the summary alone.
func TestTiOutcomeTitles(t *testing.T) {
	s, task := deliverFixture(t, "")
	if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Done.\nSecond line.", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	front, section := tiOutcomeRead(t, task.Path)
	want := "- Check result (i-1): check, 1 file in proof/i-1/"
	if !tiOutcomeHas(section, want) {
		t.Fatalf("Outcome lacks %q:\n%s", want, strings.Join(section, "\n"))
	}
	if len(section) < 3 || section[0] != "" || section[1] != "Done." || section[2] != "Second line." {
		t.Fatalf("section does not start with the summary:\n%q", section)
	}
	if !tiOutcomeHas(section, "### Proof") {
		t.Fatalf("Outcome lacks ### Proof:\n%s", strings.Join(section, "\n"))
	}
	if front["outcome"] != "Done." {
		t.Fatalf("outcome: front matter %v, want the summary", front["outcome"])
	}
}

// An item with an empty title keeps its id alone; one file is "1 file", more are
// "<n> files".
func TestTiOutcomeCounts(t *testing.T) {
	s, task := writeFixture(t)
	// A title of backticks only is empty once the plan reader strips them.
	write(t, filepath.Join(task.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  jobs: []\n- [x] 2 ``\n  jobs: []\n")
	approveFixturePlan(t, s, task)
	store := proof.Store{TaskDir: task.Path}
	for _, in := range []proof.Input{
		{Item: "1", Text: "one", Kind: "check", By: "grid"},
		{Item: "2", Text: "one", Kind: "check", By: "grid"},
		{Item: "2", Text: "two", Kind: "log", By: "grid"},
	} {
		if _, err := store.Add(in); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Done.", By: "grid"}); err != nil {
		t.Fatal(err)
	}
	_, section := tiOutcomeRead(t, task.Path)
	for _, want := range []string{
		"- Check result (i-1): check, 1 file in proof/i-1/",
		"- i-2: check, 2 files in proof/i-2/",
	} {
		if !tiOutcomeHas(section, want) {
			t.Fatalf("Outcome lacks %q:\n%s", want, strings.Join(section, "\n"))
		}
	}
}

// In a git repo the delivery records the full head in task.md and ends the
// Outcome with the branch and the first 12 of the head; outside a repo it writes
// neither.
func TestTiOutcomeHead(t *testing.T) {
	t.Run("in a repo", func(t *testing.T) {
		s, task := deliverFixture(t, "")
		repo := t.TempDir()
		testgit.Init(t, repo, "", true)
		testgit.Run(t, repo, "checkout", "-q", "-b", "grid/T-01")
		write(t, filepath.Join(repo, "f.txt"), "x\n")
		testgit.Run(t, repo, "add", "f.txt")
		testgit.Run(t, repo, "commit", "-q", "-m", "second")
		head := testgit.Run(t, repo, "rev-parse", "HEAD")
		if _, err := s.Deliver(DeliverOptions{Cwd: repo, Task: task.ID, Outcome: "Done.", By: "grid"}); err != nil {
			t.Fatal(err)
		}
		front, section := tiOutcomeRead(t, task.Path)
		if front["head"] != head {
			t.Fatalf("head: %v, want %s", front["head"], head)
		}
		wantLast := "Delivered from grid/T-01 at " + head[:12] + "."
		if len(section) == 0 || section[len(section)-1] != wantLast {
			t.Fatalf("Outcome does not end with %q:\n%s", wantLast, strings.Join(section, "\n"))
		}
		if !tiOutcomeHas(section, "- Check result (i-1): check, 1 file in proof/i-1/") {
			t.Fatalf("the Proof list moved:\n%s", strings.Join(section, "\n"))
		}
	})
	t.Run("outside a repo", func(t *testing.T) {
		s, task := deliverFixture(t, "")
		t.Setenv("GIT_CEILING_DIRECTORIES", gridhome.Real(filepath.Dir(t.TempDir())))
		if _, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Done.", By: "grid"}); err != nil {
			t.Fatal(err)
		}
		front, section := tiOutcomeRead(t, task.Path)
		if _, ok := front["head"]; ok {
			t.Fatalf("head written outside a repo: %v", front)
		}
		for _, line := range section {
			if strings.HasPrefix(line, "Delivered from") {
				t.Fatalf("Delivered line written outside a repo: %q", line)
			}
		}
	})
}
