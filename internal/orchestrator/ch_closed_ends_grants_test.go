package orchestrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// chGrantTakeovers returns the `file` of every line of feed/lease/takeovers.jsonl of task i,
// the shape taCancelTakeovers reads.
func chGrantTakeovers(t *testing.T, h *streamHome, i int) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.store(i).Dir(), "lease", "takeovers.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		var v struct {
			File string `json:"file"`
		}
		if err := json.Unmarshal(line, &v); err != nil {
			t.Fatalf("takeovers.jsonl line %q: %v", line, err)
		}
		files = append(files, v.File)
	}
	return files
}

// chGrantJobLines is the takeovers lines that end a Job grant: the lines of the task lease (a person's
// release, the stream's release, named by the fence the task lease now holds) are left out.
func chGrantJobLines(t *testing.T, h *streamHome, i int) []string {
	t.Helper()
	taskEnd := fmt.Sprintf(".%d.jsonl", h.tenure(i).Fence)
	var out []string
	for _, f := range chGrantTakeovers(t, h, i) {
		if strings.HasPrefix(f, "owner@") || strings.HasSuffix(f, taskEnd) {
			continue
		}
		out = append(out, f)
	}
	return out
}

func chGrantRead(t *testing.T, h *streamHome, i int, id string) lease.Tenure {
	t.Helper()
	g, err := (lease.TaskStore{Feed: h.store(i)}).Read(id)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// chGrantOpen is the open half of pgClosedRunning: J-1 posted, one open pass, so J-1 is assigned and granted.
func chGrantOpen(t *testing.T) (*streamHome, *Stream, lease.Tenure) {
	t.Helper()
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	h.pass(s)
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Assigned {
		t.Fatalf("J-1 is %s, want assigned", got)
	}
	g := chGrantRead(t, h, 0, "J-1")
	if g.Holder == nil {
		t.Fatal("the open pass granted J-1 to nobody")
	}
	return h, s, g
}

// chGrantEnded fails unless the grant of id names no holder, keeps fence and has one takeovers line for g.
func chGrantEnded(t *testing.T, h *streamHome, id string, g lease.Tenure) {
	t.Helper()
	got := chGrantRead(t, h, 0, id)
	if got.Holder != nil {
		t.Fatalf("grant of %s still names %s (fence %d), want no holder", id, *got.Holder, got.Fence)
	}
	if got.Fence != g.Fence {
		t.Fatalf("grant of %s fence is %d, want %d kept", id, got.Fence, g.Fence)
	}
	want := fmt.Sprintf("%s.%d.jsonl", *g.Holder, g.Fence)
	if files := chGrantJobLines(t, h, 0); len(files) != 1 || files[0] != want {
		t.Fatalf("takeovers.jsonl files are %v, want exactly [%s]", files, want)
	}
}

// Row 1: the closed pass ends the grant of the running Job it cancels.
func TestChGrantClosedEndsRunningGrant(t *testing.T) {
	h, s, g := chGrantOpen(t)
	if files := chGrantJobLines(t, h, 0); len(files) != 0 {
		t.Fatalf("takeovers.jsonl holds %v before the close, want no Job line", files)
	}
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s)
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	chGrantEnded(t, h, "J-1", g)
}

// Row 2: later passes add no takeovers line and no Feed record.
func TestChGrantClosedLaterPassesAddNoTakeover(t *testing.T) {
	h, s, g := chGrantOpen(t)
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s)
	chGrantEnded(t, h, "J-1", g)
	before := len(pgClosedRecords(t, h, 0))
	h.pass(s)
	h.pass(h.stream(4242, nil))
	chGrantEnded(t, h, "J-1", g)
	if after := len(pgClosedRecords(t, h, 0)); after != before {
		t.Errorf("later passes wrote %d records", after-before)
	}
}

// Row 3: a Job cancelled before it was ever granted has no lease file, no takeovers line and no skip.
func TestChGrantClosedNeverGrantedSkipped(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	pgClosedSetState(t, h, 0, "closed")
	s := h.stream(4242, nil)
	h.pass(s)
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if _, err := (lease.TaskStore{Feed: h.store(0)}).Read("J-1"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a Job cancelled before its grant has a lease file: %v", err)
	}
	if files := chGrantJobLines(t, h, 0); len(files) != 0 {
		t.Fatalf("takeovers.jsonl holds Job lines %v, want none", files)
	}
	if len(s.skipped) != 0 {
		t.Fatalf("the pass recorded a skip: %v", s.skipped)
	}
	if h.diag.Len() != 0 {
		t.Fatalf("the pass wrote a diagnostic: %s", h.diag.String())
	}
}

// Row 4: the worker Job's grant ends, the open curator Job stays assigned and keeps its lease file.
// This path reaches Tick, which also ends a cancelled Job's grant, so it is green on base too.
func TestChGrantClosedKeepsCuratorLease(t *testing.T) {
	h, s, g := chGrantOpen(t)
	pgClosedHolderPost(t, h, 0, "J-2", func(b map[string]any) { b["role"] = "curator" })
	pgClosedSetState(t, h, 0, "closed")
	h.pass(s)
	if got := pgClosedStatus(t, h, 0, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if got := pgClosedStatus(t, h, 0, "J-2"); got != contract.Assigned {
		t.Fatalf("the curator Job J-2 is %s, want assigned", got)
	}
	chGrantEnded(t, h, "J-1", g)
	curator := chGrantRead(t, h, 0, "J-2")
	if curator.Holder == nil {
		t.Fatal("the curator Job J-2 has no holder")
	}
	raw := filepath.Join(h.store(0).Dir(), "lease", "J-2.json")
	before, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	h.pass(s)
	after, err := os.ReadFile(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("the second pass changed the curator lease file:\n%s\n%s", before, after)
	}
	if got := pgClosedStatus(t, h, 0, "J-2"); got != contract.Assigned {
		t.Errorf("after the second pass J-2 is %s, want assigned", got)
	}
	chGrantEnded(t, h, "J-1", g)
}
