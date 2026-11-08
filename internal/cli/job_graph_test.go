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
		name, verb, items, policy string
		code                      int
	}{
		{"job-cycles-empty-json", "cycles", `[]`, "", 0},
		{"job-cycles-clear-json", "cycles", `[{"id":"J-2","after":["J-1","gate:Q-7"]},{"id":"J-1"}]`, "", 0},
		{"job-cycles-cycle-json", "cycles", `[{"id":"J-2","after":[{"job":"J-1","kind":"failed","why":"Fallback if the first Job fails"}]},{"id":"J-1","after":["J-2"]}]`, "", 1},
		{"job-floor-empty-json", "floor", `[]`, "", 0},
		{"job-floor-levels-json", "floor", `[
			{"id":"J-6","review":"L0","risk":["security"],"review_reason":"An approved exception for this small change","verify":{"command":"make test","min_tests":1}},
			{"id":"J-5","review":"L2","write":["infra/config.yaml"],"verify":{"command":"make test","min_tests":1}},
			{"id":"J-4","review":"L3","write":["app/pages/view.tsx"],"verify":{"command":"npm test","min_tests":1}},
			{"id":"J-3","review":"L2","write":["src/auth/**"],"verify":{"command":"go test ./...","min_tests":3}},
			{"id":"J-2","review":"L1","write":["src/service.go"],"verify":{"command":"go test ./...","min_tests":2}},
			{"id":"J-1","kind":"non-code","review":"L0","write":["docs/**"],"verify":{"kind":"reviewer-check"}}
		]`, `{"environment_paths":["infra/**"],"risk_paths":["src/auth/**"]}`, 0},
		{"job-floor-refused-json", "floor", `[
			{"id":"J-2","review":"L1","write":["src/service.go"],"verify":{"command":"go test ./...","min_tests":0}},
			{"id":"J-1","review":"L0","risk":["cross-platform"],"verify":{"command":"go build ./...","min_tests":1}}
		]`, "", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.write(filepath.Join(h.tmp, "items.json"), tc.items)
			args := []string{"job", tc.verb, "items.json", "--json"}
			if tc.policy != "" {
				h.write(filepath.Join(h.tmp, "policy.json"), tc.policy)
				args = append(args, "--policy", "policy.json")
			}
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
		{"unknown key", "floor", `[{"id":"J-1","risks":["security"]}]`, "unknown field"},
		{"unknown edge key", "cycles", `[{"id":"J-1","after":[{"job":"J-1","why":"A useful reason","kinds":"done"}]}]`, "unknown field"},
		{"invalid risk", "floor", `[{"id":"J-1","risk":["unknown"]}]`, "unknown risk"},
		{"invalid scope", "floor", `[{"id":"J-1","write":["../src"]}]`, "invalid scope"},
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
	for _, args := range [][]string{{"job", "cycles"}, {"job", "floor"}, {"job", "cycles", "items.json", "extra"}, {"job", "cycles", "items.json", "--unknown"}, {"job", "floor", "items.json", "--policy"}, {"job", "cycles", "items.json", "--policy", "policy.json"}} {
		if code := h.run(h.tmp, args...); code != 2 {
			t.Fatalf("%v: exit %d: %s %s", args, code, h.out, h.errb)
		}
	}
	for _, args := range [][]string{{"job", "cycles", "--help"}, {"job", "floor", "--help"}} {
		if code := h.run(h.tmp, args...); code != 0 {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
	if code := h.run(h.tmp, "job", "cycles", "items.json"); code != 1 || h.out.String() != "Job cycle: J-1 -> J-1\n" {
		t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.tmp, "job", "cycles", "--json", "items.json"); code != 1 || !strings.Contains(h.out.String(), `"cycle":["J-1","J-1"]`) {
		t.Fatalf("leading flag: %d %s", code, h.out)
	}
	h.write(filepath.Join(h.tmp, "items.json"), `[{"id":"J-1","review":"L2","irreversible":true,"verify":{"command":"make test","min_tests":1}},{"id":"J-2","review":"L1","visible":"big","verify":{"command":"make test","min_tests":1}}]`)
	if code := h.run(h.tmp, "job", "floor", "items.json"); code != 0 || !strings.Contains(h.out.String(), "gated by classify") || !strings.Contains(h.out.String(), "checker capture at L3") {
		t.Fatalf("floor: %d %s %s", code, h.out, h.errb)
	}
	if code := h.run(h.tmp, "job", "cycles", "items.json"); code != 0 || h.out.String() != "No Job cycles.\n" {
		t.Fatalf("cycles: %d %s", code, h.out)
	}
	if code := h.run(h.tmp, "job", "floor", "missing.json"); code != 1 || !strings.Contains(h.errb.String(), "read missing.json") {
		t.Fatalf("missing input: %d %s", code, h.errb)
	}
	if code := h.run(h.tmp, "job", "floor", "items.json", "--policy", "missing.json"); code != 1 {
		t.Fatalf("missing policy: %d", code)
	}
	h.write(filepath.Join(h.tmp, "policy.json"), `{"risk_path":["src/**"]}`)
	if code := h.run(h.tmp, "job", "floor", "items.json", "--policy=policy.json"); code != 1 || !strings.Contains(h.errb.String(), "unknown field") {
		t.Fatalf("invalid policy: %d %s", code, h.errb)
	}
	h.write(filepath.Join(h.tmp, "policy.json"), `null`)
	if code := h.run(h.tmp, "job", "floor", "items.json", "--policy", "policy.json"); code != 1 || !strings.Contains(h.errb.String(), "JSON object") {
		t.Fatalf("null policy: %d %s", code, h.errb)
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
	for _, want := range []string{"grid job cycles", "grid job floor"} {
		if !strings.Contains(h.out.String(), want) {
			t.Fatalf("usage lacks %q: %s", want, h.out)
		}
	}
}

func TestJobFloorPolicyRepoAndCase(t *testing.T) {
	h := newHarness(t)
	h.write(filepath.Join(h.tmp, "policy.json"), `{"repo":"main","risk_paths":["src/auth/**"]}`)
	for _, write := range []string{"main:src/auth/x.go", "Src/Auth/x.go"} {
		h.write(filepath.Join(h.tmp, "items.json"), `[{"id":"J-1","review":"L1","write":["`+write+`"],"verify":{"command":"make test","min_tests":1}}]`)
		h.out.Reset()
		if code := h.run(h.tmp, "job", "floor", "items.json", "--policy", "policy.json"); code != 1 || !strings.Contains(h.out.String(), "floor L2") || !strings.Contains(h.out.String(), "below floor L2") {
			t.Fatalf("%s: exit %d: %s %s", write, code, h.out, h.errb)
		}
	}
}
