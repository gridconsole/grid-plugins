package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
)

func TestJobGolden(t *testing.T) {
	for _, tc := range []struct {
		name, verb, items string
		code              int
	}{
		{"job-cycles-empty-json", "cycles", `[]`, 0},
		{"job-cycles-clear-json", "cycles", `[{"id":"J-2","after":["J-1","gate:Q-7"]},{"id":"J-1"}]`, 0},
		{"job-cycles-cycle-json", "cycles", `[{"id":"J-2","after":[{"job":"J-1","kind":"failed","why":"Fallback if the first Job fails"}]},{"id":"J-1","after":["J-2"]}]`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.write(filepath.Join(h.tmp, "items.json"), tc.items)
			args := []string{"job", tc.verb, "items.json", "--json"}
			if code := h.run(h.tmp, args...); code != tc.code {
				t.Fatalf("exit %d, want %d: %s %s", code, tc.code, h.out, h.errb)
			}
			if h.errb.Len() != 0 {
				t.Fatalf("stderr: %s", h.errb)
			}
			golden.Assert(t, tc.name, h.out.String())
		})
	}
}

func TestJobInputErrors(t *testing.T) {
	for _, tc := range []struct{ name, verb, input, want string }{
		{"malformed", "cycles", `[`, "decode"},
		{"not list", "cycles", `{}`, "decode"},
		{"null", "cycles", `null`, "JSON list"},
		{"two lists", "cycles", `[] []`, "multiple JSON values"},
		{"trailing junk", "cycles", `[] garbage`, "decode"},
		{"missing target", "cycles", `[{"id":"J-1","after":["J-2"]}]`, "does not exist"},
		{"duplicate", "cycles", `[{"id":"J-1"},{"id":"J-1"}]`, "duplicate"},
		{"unknown edge key", "cycles", `[{"id":"J-1","after":[{"job":"J-1","why":"A useful reason","kinds":"done"}]}]`, "unknown field"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.write(filepath.Join(h.tmp, "items.json"), tc.input)
			if code := h.run(h.tmp, "job", tc.verb, "items.json", "--json"); code != 1 || !strings.Contains(h.errb.String(), tc.want) || h.out.Len() != 0 {
				t.Fatalf("exit %d, stdout %s, stderr %s", code, h.out, h.errb)
			}
		})
	}
}

func TestJobGraphUsageAndPlain(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.tmp, "items.json"), `[{"id":"J-1","after":["J-1"]}]`)
	for _, args := range [][]string{{"job", "cycles"}, {"job", "cycles", "items.json", "extra"}, {"job", "cycles", "items.json", "--unknown"}, {"job", "cycles", "items.json", "--policy", "policy.json"}} {
		if code := h.run(h.tmp, args...); code != 2 {
			t.Fatalf("%v: exit %d: %s %s", args, code, h.out, h.errb)
		}
	}
	if code := h.run(h.tmp, "job", "cycles", "--help"); code != 0 {
		t.Fatalf("cycles --help: exit %d", code)
	}
	if code := h.run(h.tmp, "job", "cycles", "items.json"); code != 1 || h.out.String() != "Job cycle: J-1 -> J-1\n" {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.tmp, "job", "cycles", "--json", "items.json"); code != 1 || !strings.Contains(h.out.String(), `"cycle":["J-1","J-1"]`) {
		t.Fatalf("leading flag: %d %s", code, h.out)
	}
	h.write(filepath.Join(h.tmp, "items.json"), `[{"id":"J-1"},{"id":"J-2","after":["J-1"]}]`)
	if code := h.run(h.tmp, "job", "cycles", "items.json"); code != 0 || h.out.String() != "No Job cycles.\n" {
		t.Fatalf("cycles: %d %s", code, h.out)
	}
	if code := h.run(h.tmp, "job", "cycles", "missing.json"); code != 1 || !strings.Contains(h.errb.String(), "read missing.json") {
		t.Fatalf("missing input: %d %s", code, h.errb)
	}
	// Explicit -- protects a filename starting with a dash.
	if err := os.Rename(filepath.Join(h.tmp, "items.json"), filepath.Join(h.tmp, "-items.json")); err != nil {
		t.Fatal(err)
	}
	if code := h.run(h.tmp, "job", "cycles", "--json", "--", "-items.json"); code != 0 {
		t.Fatalf("dash filename: %d %s", code, h.errb)
	}
}

func TestJobUsageListsGraphVerbs(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.tmp, "job", "--help"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), "grid job cycles") {
		t.Fatalf("usage lacks %q: %s", "grid job cycles", h.out)
	}
}
