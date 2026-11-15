package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

// Round 23, lane P-r23-cli-writes (U-chk-plugin-rev-cli rows 9 and 12).

// r23UnleasedCancels is every unleased cancel request of the task.
func r23UnleasedCancels(t *testing.T, a *agentTask) []feed.Record {
	t.Helper()
	var out []feed.Record
	for _, c := range r20ClControls(t, a) {
		if c.Lease == nil && c.Body["status"] == "cancelled" {
			out = append(out, c)
		}
	}
	return out
}

// Row 12: a person's `grid job cancel J-1` typed twice saves one request. The
// second run exits 0, prints the id of the first and writes nothing.
func TestR23RepeatedPersonCancelSavesOnce(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	if code := r20ClCancel(a); code != 0 {
		t.Fatalf("the first cancel: exit %d %s", code, a.h.errb)
	}
	first := strings.TrimSpace(a.h.out.String())
	if first == "" {
		t.Fatalf("the first cancel printed no id: %q", a.h.out)
	}
	before := r20ClRecords(t, a)
	if code := r20ClCancel(a); code != 0 {
		t.Fatalf("the repeated cancel: exit %d %s", code, a.h.errb)
	}
	if second := strings.TrimSpace(a.h.out.String()); second != first {
		t.Fatalf("the repeated cancel printed %q, want the first request's id %q", second, first)
	}
	if got := r20ClRecords(t, a); got != before {
		t.Fatalf("%d records before and %d after the repeated cancel", before, got)
	}
	if n := len(r23UnleasedCancels(t, a)); n != 1 {
		t.Fatalf("want one unleased cancel request, got %d", n)
	}
}

// Row 12, --json: the repeat prints the first request, still pending.
func TestR23RepeatedPersonCancelJSONNamesTheFirst(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	if code := r20ClCancel(a, "--json"); code != 0 {
		t.Fatalf("the first cancel: exit %d %s", code, a.h.errb)
	}
	first := a.lastControl(t).ID
	if code := r20ClCancel(a, "--json"); code != 0 {
		t.Fatalf("the repeated cancel: exit %d %s", code, a.h.errb)
	}
	if !strings.Contains(a.h.out.String(), `"id":"`+first+`"`) || !strings.Contains(a.h.out.String(), `"pending":true`) {
		t.Fatalf("the repeat does not print the first request %s as pending: %s", first, a.h.out)
	}
	if n := len(r23UnleasedCancels(t, a)); n != 1 {
		t.Fatalf("want one unleased cancel request, got %d", n)
	}
}

// Row 12 keeps the rule per person: a second person's cancel of the same Job is
// still saved (personCancels lets the earliest win and says why on the Feed).
func TestR23AnotherPersonsCancelIsStillSaved(t *testing.T) {
	a, _ := kbRoleAgent(t)
	r18PersonAtTerminal(t, "dana")
	if code := r20ClCancel(a); code != 0 {
		t.Fatalf("dana's cancel: exit %d %s", code, a.h.errb)
	}
	r18PersonAtTerminal(t, "erin")
	if code := r20ClCancel(a); code != 0 {
		t.Fatalf("erin's cancel: exit %d %s", code, a.h.errb)
	}
	if n := len(r23UnleasedCancels(t, a)); n != 2 {
		t.Fatalf("want one request per person (2), got %d", n)
	}
}

// Row 9: the ship person Job's brief.md is written atomically (internal/atomicfile),
// as the other task-folder writers write; os.WriteFile leaves a truncated brief on
// a crash and the next delivery keeps it. The walk reads deliver_person_job.go.
func TestR23ShipBriefIsWrittenAtomically(t *testing.T) {
	src := "deliver_person_job.go"
	f, err := parser.ParseFile(token.NewFileSet(), src, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	atomic := false
	ast.Inspect(f, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, ok := sel.X.(*ast.Ident); ok {
			switch x.Name + "." + sel.Sel.Name {
			case "os.WriteFile", "os.Create", "ioutil.WriteFile":
				t.Errorf("%s writes with %s.%s, which is not atomic", src, x.Name, sel.Sel.Name)
			case "atomicfile.Write":
				atomic = true
			}
		}
		return true
	})
	if !atomic {
		t.Errorf("%s does not write the brief with atomicfile.Write", src)
	}
}

// Row 9, behaviour: after a delivery the brief is whole and the folder holds no
// temp file of the write.
func TestR23ShipBriefLeavesNoTempFile(t *testing.T) {
	h, dir, id, store := keeperHarness(t, "tally-money-handling")
	if code := h.deliverRun(dir, "--outcome", "Tally rounds amounts to whole cents", "--task", id); code != 0 {
		t.Fatalf("deliver: %d %s", code, h.errb)
	}
	ship, _ := openApproveJob(t, store)
	folder := filepath.Join(dir, "jobs", ship.ID)
	b, err := os.ReadFile(filepath.Join(folder, "brief.md"))
	if err != nil || !strings.HasSuffix(string(b), "## Follow-up\n") {
		t.Fatalf("the brief is not whole: %v %q", err, b)
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "brief.md" {
			t.Errorf("unexpected file %s beside the brief", e.Name())
		}
	}
}
