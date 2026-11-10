package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
)

// personCheckHarness clears every variable the check reads, then sets the given ones.
func personCheckHarness(t *testing.T, set map[string]string) *harness {
	t.Helper()
	h := newHarness(t)
	for _, k := range []string{"GRID_ROLE", "GRID_AGENT", "GRID_WORKER_ID", "GRID_WORKER"} {
		t.Setenv(k, "")
	}
	for k, v := range set {
		t.Setenv(k, v)
	}
	return h
}

// The app's caller (plugincli PersonCheck, `person check --json`) reads one JSON
// value from stdout on exit 0, and on exit 3 an object with error "person_required"
// and a non-empty reason. Anything else it takes as a failed check.
func TestPersonCheckAnswersPerson(t *testing.T) {
	for name, set := range map[string]map[string]string{
		"no role (local machine)": {},
		"cli":                     {"GRID_ROLE": "cli"},
		"owner":                   {"GRID_ROLE": "owner"},
		"member":                  {"GRID_ROLE": "member", "GRID_WHO": "alice@example.com"},
		"lead":                    {"GRID_ROLE": "lead"},
		"guest":                   {"GRID_ROLE": "guest", "GRID_WHO": "visitor@example.com"},
		"prefixed role":           {"GRID_ROLE": "grid-owner"},
	} {
		t.Run(name, func(t *testing.T) {
			h := personCheckHarness(t, set)
			if code := h.run(h.home, "person", "check", "--json"); code != 0 {
				t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
			}
			dec := json.NewDecoder(bytes.NewReader(h.out.Bytes()))
			var got map[string]any
			if err := dec.Decode(&got); err != nil || got["ok"] != true || len(got) != 1 {
				t.Fatalf("stdout %q: %v", h.out, err)
			}
			if _, err := dec.Token(); err != io.EOF {
				t.Fatalf("trailing output after the object: %q", h.out)
			}
			if h.errb.Len() != 0 {
				t.Fatalf("stderr %q", h.errb)
			}
		})
	}
}

func TestPersonCheckRefusesAgent(t *testing.T) {
	for name, tc := range map[string]struct {
		set    map[string]string
		reason string
	}{
		"worker role":   {map[string]string{"GRID_ROLE": "worker"}, "agent_role"},
		"unknown role":  {map[string]string{"GRID_ROLE": "reviewer"}, "agent_role"},
		"agent marker":  {map[string]string{"GRID_ROLE": "owner", "GRID_AGENT": "a1"}, "agent_worker"},
		"worker id":     {map[string]string{"GRID_WORKER_ID": "w1"}, "agent_worker"},
		"worker marker": {map[string]string{"GRID_ROLE": "member", "GRID_WORKER": "w1"}, "agent_worker"},
	} {
		t.Run(name, func(t *testing.T) {
			h := personCheckHarness(t, tc.set)
			if code := h.run(h.home, "person", "check", "--json"); code != 3 {
				t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
			}
			var body struct {
				Error  string `json:"error"`
				Reason string `json:"reason"`
			}
			if err := json.Unmarshal(h.out.Bytes(), &body); err != nil {
				t.Fatalf("stdout %q: %v", h.out, err)
			}
			if body.Error != "person_required" || body.Reason != tc.reason {
				t.Fatalf("body %+v, want person_required %s", body, tc.reason)
			}
			if !strings.Contains(h.errb.String(), tc.reason) {
				t.Fatalf("stderr %q lacks the reason", h.errb)
			}
		})
	}
}

// The app treats "unknown command ... person" as an older plugin; the new verb
// must not say that, and a bad argument stays a usage error, not a refusal.
func TestPersonCheckUsage(t *testing.T) {
	h := personCheckHarness(t, nil)
	if code := h.run(h.home, "person", "check", "--json", "extra"); code != 2 {
		t.Fatalf("extra argument: exit %d", code)
	}
	if code := h.run(h.home, "person", "check", "--bogus"); code != 2 {
		t.Fatalf("unknown flag: exit %d", code)
	}
	if code := h.run(h.home, "person", "check"); code != 0 || strings.TrimSpace(h.out.String()) != "person check: ok" {
		t.Fatalf("plain: exit %d %q", code, h.out)
	}
	if h.run(h.home, "person", "nope") != 2 || !strings.Contains(h.errb.String(), "unknown command") {
		t.Fatalf("an unknown person verb must keep its usage error: %q", h.errb)
	}
}
