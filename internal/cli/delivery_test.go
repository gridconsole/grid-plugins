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
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/job/frontmatter"
	"github.com/gridconsole/grid-plugins/internal/plan"
	"github.com/gridconsole/grid-plugins/internal/proof"
	"github.com/gridconsole/grid-plugins/internal/task"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

func deliveryFixture(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	created, err := (task.Store{Resolver: resolver(h.env)}).New(task.NewOptions{Cwd: h.home, Project: "personal", Words: "Deliver result"})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(created.Path, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: check\n  jobs: []\n")
	approveDeliveryPlan(t, h, created)
	return h, created.Path
}

// approveDeliveryPlan leaves the receipt a person's approval of plan.md leaves,
// so the task can be delivered. A test that later rewrites plan.md unapproves it.
func approveDeliveryPlan(t *testing.T, h *harness, created task.Task) {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(created.Path, "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(task.Event{Task: created.ID, Who: contract.Author{ID: "local", Role: "owner"}, When: "2026-01-02T03:04:05.000Z", What: "approve", FromState: created.State, ToState: created.State, PlanHash: plan.Hash(string(text)), PlanDigest: plan.Digest(string(text))})
	if err != nil {
		t.Fatal(err)
	}
	h.write(filepath.Join(created.Path, "approval.json"), string(data)+"\n")
}

// acceptPlanInHistory releases the plan as a task migrated from the old plugin
// is: a "plan accepted" history line, with no approval.json.
func acceptPlanInHistory(t *testing.T, h *harness, dir string) {
	t.Helper()
	if err := os.Remove(filepath.Join(dir, "approval.json")); err != nil {
		t.Fatal(err)
	}
	history, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	h.write(filepath.Join(dir, "history.md"), string(history)+taskfile.HistoryLine("2026-01-02T03:04", "plan accepted", "000000", "owner")+"\n")
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

// homeProposal is a full home-level note, as accept reads it.
func homeProposal(name, source string) string {
	return "---\nname: " + name + "\ntopic: testing\ntype: reference\ndescription: A fact\nsummary: A fact\nupdated: 2026-01-01T00:00:00Z\nsource: " + source +
		"\nsuperseded_by: \"\"\nlinks: []\nlevel: home\nstatus: proposed\nreason: Reuse this check\norigin:\n  kind: job\n  id: " + source + "\n  tier: agent\n---\nA fact.\n"
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
			h.write(filepath.Join(h.gridHome, "brain-proposals", "fact.md"), homeProposal("fact", "T-01/J-01"))
		}, []string{"deliver", "Result ready"}, "Deliver blocked: 1 brain proposals wait", 1},
		{"home-malformed", func(h *harness, d string) {
			h.write(filepath.Join(h.gridHome, "brain-proposals", "fact.md"), "---\nstatus: proposed\ntopic: testing\n---\n")
		}, []string{"deliver", "Result ready"}, "proposal fact.md", 1},
		{"malformed-proposal", func(h *harness, d string) {
			h.write(filepath.Join(d, "brain-proposals", "fact.md"), "---\nstatus: [\n---\n")
		}, []string{"deliver", "Result ready"}, "proposal fact.md", 1},
		{"wrong-level", func(h *harness, d string) {
			h.write(filepath.Join(d, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: screenshot\n  jobs: []\n")
			addDeliveryProof(t, h, d)
		}, []string{"deliver", "Result ready"}, "lacks screenshot proof", 1},
		{"unknown-level", func(h *harness, d string) {
			h.write(filepath.Join(d, "plan.md"), "# Plan 1\n- [x] 1 Check result\n  proof: magic\n  jobs: []\n")
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
	if !preview.DryRun || len(preview.Actions) != 4 {
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
	if doc.Data["state"] != "waiting" || doc.Data["gate"] != nil || doc.Data["step"] != "ship" || doc.Data["outcome"] != "The report is ready." {
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
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(history), "\n"), "\n")
	var receipt task.Event
	if err = json.Unmarshal([]byte(strings.TrimPrefix(lines[len(lines)-1], "- ")), &receipt); err != nil {
		t.Fatal(err, lines)
	}
	if receipt.What != "deliver" || receipt.Note != "The report is ready." || receipt.Who.ID != "grid" || receipt.FromState != "queued" || receipt.ToState != "waiting" || strings.Contains(string(history), "approve") {
		t.Fatalf("%+v", receipt)
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
	// A task already at the ship gate cannot move to waiting again; the person
	// sends it back, and delivery then replaces the section without inventing an approval.
	if code := h.run(dir, "deliver", "Updated outcome"); code != 1 || !strings.Contains(h.errb.String(), `current state "waiting"`) {
		t.Fatal(code, h.errb)
	}
	if code := h.run(dir, "task", "status", "T-01", "running"); code != 0 {
		t.Fatal(h.errb)
	}
	if code := h.run(dir, "deliver", "Updated outcome"); code != 0 {
		t.Fatal(h.errb)
	}
	b, err = os.ReadFile(taskPath)
	if err != nil || strings.Count(string(b), "## Outcome\n") != 1 || !strings.Contains(string(b), "Updated outcome") {
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
		{[]string{"history", "add", "two\nlines"}, 2, "one nonempty line"},
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
	writer := os.Getenv("GRID_HISTORY_CLI_WRITER")
	if writer == "status" {
		for i := 0; i < 30; i++ {
			state := []string{"running", "waiting"}[i%2]
			if code := Run([]string{"task", "status", "T-01", state}, env); code != 0 {
				t.Fatal(code, errout.String())
			}
		}
		return
	}
	for i := 0; i < 20; i++ {
		line := fmt.Sprintf("writer%s-%d", writer, i)
		if code := Run([]string{"history", "add", line}, env); code != 0 {
			t.Fatal(code, errout.String())
		}
	}
}

// Two `history add` processes and a `task status` process write one task at
// once. All three go through the task writer's lock, so no line is lost.
func TestConcurrentHistoryCommands(t *testing.T) {
	h, dir := deliveryFixture(t)
	before, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	base := strings.Count(string(before), "\n")
	writers := []string{"0", "1", "status"}
	cmds := make([]*exec.Cmd, len(writers))
	errs := make([]bytes.Buffer, len(writers))
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestHistoryCommandProcess$")
		cmds[i].Env = append(os.Environ(), "GRID_HISTORY_CLI_HELPER=1", "GRID_HISTORY_CLI_TASK="+dir, "GRID_HISTORY_CLI_HOME="+h.gridHome, "GRID_HISTORY_CLI_WRITER="+writers[i])
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
	if got := strings.Count(string(b), "\n"); got != base+40+30 {
		t.Fatalf("history has %d lines, want %d: %s", got, base+70, b)
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 20; j++ {
			if strings.Count(string(b), fmt.Sprintf(`"note":"writer%d-%d"}`+"\n", i, j)) != 1 {
				t.Fatalf("missing or repeated %d-%d", i, j)
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

func TestUnrelatedHomeProposalDoesNotBlockDelivery(t *testing.T) {
	h, dir := deliveryFixture(t)
	addDeliveryProof(t, h, dir)
	h.write(filepath.Join(h.gridHome, "brain-proposals", "elsewhere.md"), homeProposal("elsewhere", "T-99/J-01"))
	if code := h.run(h.home, "brain", "keep", "--task", "T-01"); code != 0 || !strings.Contains(h.out.String(), "elsewhere") {
		t.Fatalf("keep must still list it: %s %s", h.out, h.errb)
	}
	if code := h.run(dir, "deliver", "Ready"); code != 0 {
		t.Fatal(h.errb)
	}
}

// Delivery raises no ship gate (S6c3), so approve after it records the plan
// approval in approval.json, with no gate, and writes no ship receipt.
func TestApproveAfterDeliveryRecordsPlanApproval(t *testing.T) {
	h, dir := deliveryFixture(t)
	acceptPlanInHistory(t, h, dir)
	addDeliveryProof(t, h, dir)
	if code := h.run(dir, "deliver", "Ready"); code != 0 {
		t.Fatal(h.errb)
	}
	h.leased(dir)
	if code := h.run(dir, "approve", "T-01", "--json"); code != 0 {
		t.Fatal(h.errb)
	}
	var event task.Event
	if err := json.Unmarshal(h.out.Bytes(), &event); err != nil || event.What != "approve" || event.Gate != "" || event.PlanHash == "" {
		t.Fatalf("%+v %v", event, err)
	}
	// The approval is the plan Job result, not a file (verdict 12).
	if _, err := os.Stat(filepath.Join(dir, "approval.json")); !os.IsNotExist(err) {
		t.Fatal("approve wrote approval.json", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "approval-ship.json")); !os.IsNotExist(err) {
		t.Fatal("approve wrote a ship receipt", err)
	}
	if !task.Approved(dir) {
		t.Fatal("the plan approval must count")
	}
}
