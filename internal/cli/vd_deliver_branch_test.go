package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// vdBrRepo makes a repo under the harness with a commit on main and a second on
// grid/T-01, which is checked out. It returns the folder and main's sha.
func vdBrRepo(t *testing.T, h *harness) (string, string) {
	t.Helper()
	dir := filepath.Join(h.tmp, "worktree")
	h.mkdir(dir)
	testgit.Init(t, dir, "", true)
	main := testgit.Run(t, dir, "rev-parse", "HEAD")
	testgit.Run(t, dir, "checkout", "-q", "-b", "grid/T-01")
	h.write(filepath.Join(dir, "f.txt"), "x\n")
	testgit.Run(t, dir, "add", "f.txt")
	testgit.Run(t, dir, "commit", "-q", "-m", "second")
	return dir, main
}

// vdBrFront returns the parsed front matter of the task's task.md.
func vdBrFront(t *testing.T, taskDir string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(taskDir, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := frontmatter.Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(b), "\nbranch:"); n > 1 {
		t.Fatalf("%d branch keys:\n%s", n, b)
	}
	return doc.Data
}

func TestVdBrDeliverRecordsGitBranchAndBase(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	repo, main := vdBrRepo(t, h)
	if code := h.run(repo, "deliver", "--task", "T-01", "--outcome", "X"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); got != "T-01 delivered; waiting for the person\n" {
		t.Fatalf("stdout: %q", got)
	}
	data := vdBrFront(t, dir)
	if data["branch"] != "grid/T-01" || data["base"] != main {
		t.Fatalf("branch/base: %v / %v, want grid/T-01 / %s", data["branch"], data["base"], main)
	}
}

func TestVdBrDeliverFlagsWin(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	repo, _ := vdBrRepo(t, h)
	if code := h.run(repo, "deliver", "--task", "T-01", "--outcome", "X", "--branch", "b2", "--base", "abc123"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	data := vdBrFront(t, dir)
	if data["branch"] != "b2" || data["base"] != "abc123" {
		t.Fatalf("branch/base: %v / %v", data["branch"], data["base"])
	}
}

func TestVdBrDeliverOutsideRepoWritesNeither(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	t.Setenv("GIT_CEILING_DIRECTORIES", h.root)
	if code := h.run(h.tmp, "deliver", "--task", "T-01", "--outcome", "X"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); got != "T-01 delivered; waiting for the person\n" {
		t.Fatalf("stdout: %q", got)
	}
	data := vdBrFront(t, dir)
	if _, ok := data["branch"]; ok {
		t.Fatalf("branch written: %v", data)
	}
	if _, ok := data["base"]; ok {
		t.Fatalf("base written: %v", data)
	}
}

func TestVdBrSecondDeliveryKeepsOneBranchKey(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	repo, main := vdBrRepo(t, h)
	if code := h.run(repo, "deliver", "--task", "T-01", "--outcome", "X"); code != 0 {
		t.Fatalf("first exit %d: %s", code, h.errb)
	}
	// The person sends the task back; delivery may then run again.
	if code := h.run(dir, "task", "status", "T-01", "running"); code != 0 {
		t.Fatalf("status exit %d: %s", code, h.errb)
	}
	if code := h.run(repo, "deliver", "--task", "T-01", "--outcome", "Y", "--branch", "b3"); code != 0 {
		t.Fatalf("second exit %d: %s", code, h.errb)
	}
	data := vdBrFront(t, dir)
	if data["branch"] != "b3" || data["base"] != main {
		t.Fatalf("branch/base: %v / %v", data["branch"], data["base"])
	}
}
