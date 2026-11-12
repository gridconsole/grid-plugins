package orchestrator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/lease"
)

// taCancelRig is the rig TestGkReplanNotForControlCancel uses, one tick and one
// ack further: J-1 is assigned to the worker, which has acknowledged it, so its
// grant lease/J-1.json names the worker and a child would be running.
func taCancelRig(t *testing.T) (*fixture, lease.Tenure) {
	t.Helper()
	f := setup(t)
	f.job(t, "J-1", nil)
	f.tick(t)
	f.ack(t)
	return f, taCancelGrant(t, f)
}

func taCancelGrant(t *testing.T, f *fixture) lease.Tenure {
	t.Helper()
	g, err := (lease.TaskStore{Feed: f.s.Feed}).Read("J-1")
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// taCancelPost posts the owner's cancel the way gk_replan_after_cancel_test.go does.
func taCancelPost(t *testing.T, f *fixture) {
	t.Helper()
	f.post(t, feed.Input{Type: "control", Body: map[string]any{"target": "J-1", "reason": "owner cancelled", "status": contract.Cancelled, "close_reason": "owner cancelled"}}, f.s.Writer)
}

// taCancelTakeovers returns the `file` of every line of feed/lease/takeovers.jsonl.
func taCancelTakeovers(t *testing.T, f *fixture) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(f.s.Feed.Dir(), "lease", "takeovers.jsonl"))
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

func taCancelWant(t *testing.T, f *fixture, g lease.Tenure, ended bool) {
	t.Helper()
	got := taCancelGrant(t, f)
	files := taCancelTakeovers(t, f)
	if g.Holder == nil {
		t.Fatal("the rig granted J-1 to nobody")
	}
	name := fmt.Sprintf("%s.%d.jsonl", *g.Holder, g.Fence)
	if !ended {
		if got.Holder == nil || *got.Holder != *g.Holder || got.Fence != g.Fence {
			t.Fatalf("grant of J-1 is %+v, want it still held by %s at fence %d", got, *g.Holder, g.Fence)
		}
		if len(files) != 0 {
			t.Fatalf("takeovers.jsonl holds %v, want no line", files)
		}
		return
	}
	if got.Holder != nil {
		t.Fatalf("grant of J-1 still names %s (fence %d), want no holder", *got.Holder, got.Fence)
	}
	if got.Fence != g.Fence {
		t.Fatalf("grant of J-1 fence is %d, want %d kept", got.Fence, g.Fence)
	}
	if len(files) != 1 || files[0] != name {
		t.Fatalf("takeovers.jsonl files are %v, want exactly [%s]", files, name)
	}
}

func TestTaCancelEndsGrant(t *testing.T) {
	f, g := taCancelRig(t)
	taCancelWant(t, f, g, false)
	taCancelPost(t, f)
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	taCancelWant(t, f, g, true)
}

func TestTaCancelRunningKeepsGrant(t *testing.T) {
	f, g := taCancelRig(t)
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got == contract.Cancelled {
		t.Fatalf("J-1 is %s, want a running Job", got)
	}
	taCancelWant(t, f, g, false)
}

func TestTaCancelOnce(t *testing.T) {
	f, g := taCancelRig(t)
	taCancelPost(t, f)
	f.tick(t)
	taCancelWant(t, f, g, true)
	f.tick(t)
	f.tick(t)
	taCancelWant(t, f, g, true)
}

// A Job that was cancelled before it was ever granted has no lease file, and the
// pass leaves it alone.
func TestTaCancelNeverGranted(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	taCancelPost(t, f)
	f.tick(t)
	f.tick(t)
	if got := gkStatus(t, f, "J-1"); got != contract.Cancelled {
		t.Fatalf("J-1 is %s, want cancelled", got)
	}
	if _, err := (lease.TaskStore{Feed: f.s.Feed}).Read("J-1"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("a Job cancelled before its grant has a lease file: %v", err)
	}
	if files := taCancelTakeovers(t, f); len(files) != 0 {
		t.Fatalf("takeovers.jsonl holds %v, want no line", files)
	}
}
