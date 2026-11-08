package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/artifact"
	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func deliveryFixture(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	created, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: h.home, Project: "personal", Words: "Deliver result"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(created.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: check\n")
	return h, created.Path
}

func treeBytes(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
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

func addDeliveryProof(t *testing.T, h *harness, dir string) {
	t.Helper()
	if code := h.run(dir, "proof", "add", "1", "--text", "All checks pass", "--kind", "check", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
}

func TestDeliveryCommandRefusals(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*harness, string)
		args  []string
		want  string
		code  int
	}{
		{"missing-outcome", nil, []string{"deliver"}, "supply --outcome", 2},
		{"missing-proof", nil, []string{"deliver", "--outcome", "Result ready"}, "i-1 lacks check proof", 1},
		{"pending", func(h *harness, d string) {
			h.write(filepath.Join(d, "brain-proposals", "fact.md"), "---\nstatus: proposed\ntopic: testing\nreason: Reuse this check\n---\n")
		}, []string{"deliver", "Result ready"}, "Deliver blocked: 1 brain proposals wait. Run: grid brain keep\n", 1},
		{"home-pending", func(h *harness, d string) {
			h.write(filepath.Join(h.gridHome, "brain-proposals", "fact.md"), "---\nstatus: proposed\ntopic: testing\n---\n")
		}, []string{"deliver", "Result ready"}, "brain proposals wait", 1},
		{"malformed-proposal", func(h *harness, d string) {
			h.write(filepath.Join(d, "brain-proposals", "fact.md"), "---\nstatus: [\n---\n")
		}, []string{"deliver", "Result ready"}, "proposal fact.md", 1},
		{"wrong-level", func(h *harness, d string) {
			h.write(filepath.Join(d, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: screenshot\n")
			addDeliveryProof(t, h, d)
		}, []string{"deliver", "Result ready"}, "lacks screenshot proof", 1},
		{"unknown-level", func(h *harness, d string) {
			h.write(filepath.Join(d, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: magic\n")
		}, []string{"deliver", "Result ready"}, "unknown proof level", 1},
		{"empty-plan", func(h *harness, d string) { h.write(filepath.Join(d, "plan.md"), "# Plan 1\n") }, []string{"deliver", "Result ready"}, "no items to prove", 1},
		{"missing-file", func(h *harness, d string) { addDeliveryProof(t, h, d) }, []string{"deliver", "Result ready", "absent.txt"}, "absent.txt", 1},
		{"dry-pending", func(h *harness, d string) {
			h.write(filepath.Join(d, "brain-proposals", "fact.md"), "---\nstatus: proposed\ntopic: testing\n---\n")
		}, []string{"deliver", "Result ready", "--dry-run"}, "brain proposals wait", 1},
		{"closed", func(h *harness, d string) {
			p := filepath.Join(d, "task.md")
			b, e := os.ReadFile(p)
			if e != nil {
				t.Fatal(e)
			}
			h.write(p, strings.Replace(string(b), "state: queued", "state: closed", 1))
		}, []string{"deliver", "Result ready"}, "cannot reopen", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, dir := deliveryFixture(t)
			if tc.setup != nil {
				tc.setup(h, dir)
			}
			before := treeBytes(t, h.gridHome)
			code := h.run(dir, tc.args...)
			if code != tc.code || !strings.Contains(h.errb.String(), tc.want) {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if strings.Count(h.errb.String(), "\n") != 1 {
				t.Fatalf("not one line: %q", h.errb.String())
			}
			if !reflect.DeepEqual(before, treeBytes(t, h.gridHome)) {
				t.Fatal("refusal wrote files")
			}
		})
	}
}

func TestDeliverSuccessAndDryRun(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	taskPath := filepath.Join(dir, "task.md")
	b, err := os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	h.write(taskPath, string(b)+"\n## Notes\n\nKeep these.\n\n## Outcome\n\nOld result.\n\n## Follow-up\n\nKeep this too.\n")
	source := filepath.Join(dir, "report.txt")
	h.write(source, "Report bytes\n")
	h.write(filepath.Join(dir, "brain-proposals", "done.md"), "---\nstatus: accepted\ntopic: testing\n---\n")
	before := treeBytes(t, h.gridHome)
	args := []string{"deliver", "--task", "T-01", "--outcome", "The report is ready.", "--file", source}
	if code := h.run(h.home, append(args, "--dry-run", "--json")...); code != 0 {
		t.Fatal(h.errb)
	}
	var preview task.Delivery
	if err = json.Unmarshal(h.out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if !preview.DryRun || len(preview.Actions) != 5 {
		t.Fatal(preview)
	}
	if !reflect.DeepEqual(before, treeBytes(t, h.gridHome)) {
		t.Fatal("dry run wrote files")
	}
	if code := h.run(h.home, append(args, "--json")...); code != 0 {
		t.Fatal(h.errb)
	}
	var result task.Delivery
	if err = json.Unmarshal(h.out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.DryRun || len(result.Artifacts) != 1 {
		t.Fatal(result)
	}
	b, err = os.ReadFile(taskPath)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := frontmatter.Parse(string(b))
	if err != nil {
		t.Fatal(err)
	}
	if doc.Data["state"] != "waiting" || doc.Data["gate"] != "ship" || doc.Data["step"] != "ship" || doc.Data["outcome"] != "The report is ready." {
		t.Fatal(doc.Data)
	}
	for _, text := range []string{"Keep these.", "Keep this too.", "### Deliverables", "### Proof", result.Artifacts[0].File} {
		if !strings.Contains(doc.Body, text) {
			t.Fatalf("missing %s: %s", text, doc.Body)
		}
	}
	if strings.Contains(doc.Body, "Old result") || strings.Count(doc.Body, "## Outcome\n") != 1 {
		t.Fatal(doc.Body)
	}
	history, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil || !strings.Contains(string(history), "delivered · The report is ready. · by grid\n") || strings.Contains(string(history), "approved") {
		t.Fatal(string(history), err)
	}
	records, err := (artifact.Store{TaskPath: dir}).List()
	if err != nil || len(records) != 1 {
		t.Fatal(records, err)
	}
	copied, err := os.ReadFile(filepath.Join(dir, "artifacts", records[0].File))
	if err != nil || string(copied) != "Report bytes\n" {
		t.Fatal(string(copied), err)
	}
	original, err := os.ReadFile(source)
	if err != nil || !bytes.Equal(original, copied) {
		t.Fatal("source changed", err)
	}
	if _, err = os.Stat(filepath.Join(dir, "proof", "index.json")); err != nil {
		t.Fatal(err)
	}
	// Re-delivery replaces the section without inventing an approval.
	if code := h.run(dir, "deliver", "Updated outcome"); code != 0 {
		t.Fatal(h.errb)
	}
	b, err = os.ReadFile(taskPath)
	if err != nil || strings.Count(string(b), "## Outcome\n") != 1 {
		t.Fatal(string(b), err)
	}
}

func TestProofCommandTable(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"proof", "add", "1", "--text", "Pass", "--json"}, 0, `"sha256"`},
		{[]string{"proof", "index", "--json"}, 0, `"items"`},
		{[]string{"proof", "add", "9", "--text", "Pass"}, 1, "unknown plan item"},
		{[]string{"proof", "add", "1", "--text", "Pass", "--kind", "unknown"}, 1, "kind must"},
		{[]string{"proof", "add", "1"}, 1, "one file"},
		{[]string{"proof", "add"}, 2, "operands"},
		{[]string{"proof", "index", "extra"}, 2, "operands"},
		{[]string{"proof", "remove"}, 2, "expected add or index"},
		{[]string{"proof", "--help"}, 0, "Usage"},
		{[]string{"proof", "index", "--invalid"}, 2, "not defined"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			h, dir := deliveryFixture(t)
			code := h.run(dir, tc.args...)
			if code != tc.code || !strings.Contains(h.out.String()+h.errb.String(), tc.want) {
				t.Fatalf("exit %d: %s%s", code, h.out, h.errb)
			}
		})
	}
}

func TestBrainKeepCommand(t *testing.T) {
	for _, status := range []string{"proposed", "accepted", "merged", "declined"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", status, asJSON), func(t *testing.T) {
				h, dir := deliveryFixture(t)
				h.write(filepath.Join(dir, "brain-proposals", "check.md"), "---\nstatus: "+status+"\ntopic: testing\nreason: Keep the useful check\n---\n")
				args := []string{"brain", "keep", "--task", "T-01"}
				if asJSON {
					args = append(args, "--json")
				}
				before := treeBytes(t, h.gridHome)
				if code := h.run(h.home, args...); code != 0 {
					t.Fatal(h.errb)
				}
				if asJSON {
					var rows []brain.WaitingProposal
					if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
						t.Fatal(err)
					}
					want := 0
					if status == "proposed" {
						want = 1
					}
					if len(rows) != want {
						t.Fatal(rows)
					}
				} else if status == "proposed" && !strings.Contains(h.out.String(), "testing: Keep the useful check") {
					t.Fatal(h.out)
				} else if status != "proposed" && h.out.Len() != 0 {
					t.Fatal(h.out)
				}
				if !reflect.DeepEqual(before, treeBytes(t, h.gridHome)) {
					t.Fatal("keep wrote files")
				}
			})
		}
	}
}

func TestHistoryCommandTable(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"history", "add", "Checked the result"}, 0, "History added"},
		{[]string{"history", "add", "Checked the result", "--task", "T-01", "--json"}, 0, `"line"`},
		{[]string{"history", "add", "two\nlines"}, 1, "one nonempty line"},
		{[]string{"history", "add", ""}, 1, "one nonempty line"},
		{[]string{"history", "add"}, 2, "expected one line"},
		{[]string{"history", "remove"}, 2, "expected add"},
		{[]string{"history", "--help"}, 0, "Usage"},
		{[]string{"history", "add", "line", "--task", "T-99"}, 1, "not found"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			h, dir := deliveryFixture(t)
			code := h.run(dir, tc.args...)
			if code != tc.code || !strings.Contains(h.out.String()+h.errb.String(), tc.want) {
				t.Fatalf("exit %d: %s%s", code, h.out, h.errb)
			}
		})
	}
}

func TestHistoryCommandProcess(t *testing.T) {
	if os.Getenv("GRID_HISTORY_CLI_HELPER") != "1" {
		return
	}
	var out, errout bytes.Buffer
	env := Env{Cwd: os.Getenv("GRID_HISTORY_CLI_TASK"), GridHome: os.Getenv("GRID_HISTORY_CLI_HOME"), Stdout: &out, Stderr: &errout}
	for i := 0; i < 20; i++ {
		line := fmt.Sprintf("writer%s-%d", os.Getenv("GRID_HISTORY_CLI_WRITER"), i)
		if code := Run([]string{"history", "add", line}, env); code != 0 {
			t.Fatal(code, errout.String())
		}
	}
}

func TestConcurrentHistoryCommands(t *testing.T) {
	h, dir := deliveryFixture(t)
	cmds := make([]*exec.Cmd, 2)
	var errs [2]bytes.Buffer
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestHistoryCommandProcess$")
		cmds[i].Env = append(os.Environ(), "GRID_HISTORY_CLI_HELPER=1", "GRID_HISTORY_CLI_TASK="+dir, "GRID_HISTORY_CLI_HOME="+h.gridHome, fmt.Sprintf("GRID_HISTORY_CLI_WRITER=%d", i))
		cmds[i].Stdout = &errs[i]
		cmds[i].Stderr = &errs[i]
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err, errs[i].String())
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "\n") != 40 {
		t.Fatal(string(b))
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 20; j++ {
			if strings.Count(string(b), fmt.Sprintf(" · writer%d-%d\n", i, j)) != 1 {
				t.Fatalf("missing %d-%d", i, j)
			}
		}
	}
}

func TestDeliverRejectsTamperedProof(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	var r proof.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(dir, filepath.FromSlash(r.File)), "changed")
	if code := h.run(dir, "deliver", "Ready"); code != 1 || !strings.Contains(h.errb.String(), "checksum") {
		t.Fatal(code, h.errb)
	}
}
