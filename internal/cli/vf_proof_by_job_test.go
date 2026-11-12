package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/proof"
)

// vfByRun adds proof under GRID_JOB=job and returns the saved record's author.
func vfByRun(t *testing.T, job string, args ...string) string {
	t.Helper()
	h, id, _ := proofTask(t)
	h.env.Getenv = func(key string) string {
		if key == "GRID_JOB" {
			return job
		}
		return ""
	}
	h.write(filepath.Join(h.home, "shot.png"), "png-bytes")
	full := append([]string{"proof", "add"}, args...)
	full = append(full, "--task", id, "--json")
	if code := h.run(h.home, full...); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var r proof.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatalf("json %q: %v", h.out.String(), err)
	}
	t.Logf("record %s", h.out.String())
	return r.By
}

func TestVfByJobWhenNoBy(t *testing.T) {
	if got := vfByRun(t, "J-2", "1", "--text", "x"); got != "J-2" {
		t.Errorf("by %q, want J-2", got)
	}
}

func TestVfByExplicitWins(t *testing.T) {
	if got := vfByRun(t, "J-2", "1", "--text", "x", "--by", "alex"); got != "alex" {
		t.Errorf("by %q, want alex", got)
	}
}

func TestVfByExplicitGridWins(t *testing.T) {
	if got := vfByRun(t, "J-2", "1", "--text", "x", "--by", "grid"); got != "grid" {
		t.Errorf("by %q, want grid", got)
	}
}

func TestVfByNoJobKeepsGrid(t *testing.T) {
	if got := vfByRun(t, "", "1", "--text", "x"); got != "grid" {
		t.Errorf("by %q, want grid", got)
	}
}

func TestVfByFileProof(t *testing.T) {
	if got := vfByRun(t, "J-3", "2", "shot.png", "--kind", "screenshot"); got != "J-3" {
		t.Errorf("by %q, want J-3", got)
	}
}
