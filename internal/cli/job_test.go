package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJobRegistered(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "job", "--help"); code != 0 || !strings.Contains(h.out.String(), "grid job render") {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	path := h.tmp + "/brief.md"
	h.write(path, "no frontmatter\n")
	if code := h.run(h.tmp, "job", "lint", path, "--json"); code != 1 {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	var out struct {
		Findings []struct {
			Rule string `json:"rule"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Findings) != 1 || out.Findings[0].Rule != "brief.frontmatter" {
		t.Fatalf("%+v", out)
	}
}
