package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// ikJobVerbs is every verb grid job answers: the orchestrator verbs of
// runOrchestratorJob and the verbs of internal/job/jobcmd.
var ikJobVerbs = []string{"next", "post", "ack", "result", "cancel", "tree", "blocked", "render", "lint", "show", "trace", "verify", "cycles"}

// TestIkJobUsageListsEveryVerb: grid job --help names every job verb on stdout,
// and a bare grid job exits 2 with the usage on stderr, as a bare grid feed does.
func TestIkJobUsageListsEveryVerb(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "job", "--help"); code != 0 || h.errb.Len() != 0 {
		t.Fatalf("job --help: exit %d, stderr %q", code, h.errb)
	}
	for _, verb := range ikJobVerbs {
		if !strings.Contains(h.out.String(), "grid job "+verb) {
			t.Errorf("job --help lacks %q:\n%s", "grid job "+verb, h.out)
		}
	}
	if code := h.run(h.home, "job"); code != 2 || h.out.Len() != 0 || !strings.Contains(h.errb.String(), "grid job next") {
		t.Fatalf("bare job: exit %d, want 2 with the usage on stderr\nstdout: %s\nstderr: %s", code, h.out, h.errb)
	}
	if code := h.run(h.home, "feed"); code != 2 {
		t.Fatalf("bare feed: exit %d, want 2", code)
	}
}

// TestIkJobUnknownVerbNamesEveryVerb: the unknown-verb refusal of grid job lists all the verbs.
func TestIkJobUnknownVerbNamesEveryVerb(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "job", "frobnicate"); code != 2 {
		t.Fatalf("job frobnicate: exit %d, want 2: %s%s", code, h.out, h.errb)
	}
	for _, verb := range ikJobVerbs {
		if !strings.Contains(h.errb.String(), verb) {
			t.Errorf("unknown-verb text lacks %q: %s", verb, h.errb)
		}
	}
}

// TestIkHelpSummariesNameBuiltVerbs: every verb the job, feed and plan summaries
// of grid help (and so contract/commands.json) advertise answers --help with exit 0.
func TestIkHelpSummariesNameBuiltVerbs(t *testing.T) {
	h := newHarness(t)
	for _, c := range commands() {
		if c.name != "job" && c.name != "feed" && c.name != "plan" {
			continue
		}
		for _, verb := range strings.Split(c.summary, ", ") {
			if code := h.run(h.home, c.name, verb, "--help"); code != 0 {
				t.Errorf("grid %s %s --help: exit %d, but the summary %q advertises it\n%s%s", c.name, verb, code, c.summary, h.out, h.errb)
			}
		}
	}
}

// TestIkHelpGoesToStdoutUnderItsOwnName: --help prints the usage of the command asked about, on stdout.
func TestIkHelpGoesToStdoutUnderItsOwnName(t *testing.T) {
	h := newHarness(t)
	cases := []struct {
		args []string
		want string
		not  string
	}{
		{[]string{"feed", "tick", "--help"}, "grid feed tick", "grid job tick"},
		{[]string{"job", "next", "--help"}, "grid job next", ""},
		{[]string{"job", "cancel", "--help"}, "grid job cancel", ""},
		{[]string{"approve", "--help"}, "Usage: grid approve", ""},
		{[]string{"plan", "check", "--help"}, "grid plan", ""},
		{[]string{"routine", "add", "--help"}, "grid routine add", ""},
	}
	for _, c := range cases {
		code := h.run(h.home, c.args...)
		if code != 0 || h.errb.Len() != 0 || !strings.Contains(h.out.String(), c.want) || (c.not != "" && strings.Contains(h.out.String(), c.not)) {
			t.Errorf("%v: exit %d, want 0 with %q on stdout and nothing on stderr\nstdout: %s\nstderr: %s", c.args, code, c.want, h.out, h.errb)
		}
	}
}

// TestIkHookInventoryHasItsFlags: contract/commands.json lists the flags hook level and hook levels take.
func TestIkHookInventoryHasItsFlags(t *testing.T) {
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]bool{"level": {"--project": true, "--json": true}, "levels": {"--json": true}}
	for _, c := range doc.Commands {
		if c.Name != "hook" {
			continue
		}
		for _, sub := range c.Subcommands {
			for flag := range want[sub.Name] {
				found := false
				for _, f := range sub.Flags {
					found = found || f.Name == flag
				}
				if !found {
					t.Errorf("hook %s: commands.json lists no %s flag", sub.Name, flag)
				}
			}
			if sub.Name == "levels" && !sub.JSON {
				t.Error("hook levels: commands.json has json false, but --json works")
			}
		}
	}
}

// TestIkBrainWriteDecisionNamesExpect: the brain write decision gives the flag the built verb takes.
func TestIkBrainWriteDecisionNamesExpect(t *testing.T) {
	data, err := os.ReadFile("../../docs/decisions/D-ie-brain-write-decision.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "--rev") {
		t.Error("D-ie-brain-write-decision.md names --rev; the built verb takes --expect (contract/README.md grid brain write)")
	}
	if !strings.Contains(string(data), "grid brain write <topic>/<name> --expect") {
		t.Error("D-ie-brain-write-decision.md does not give the verb line with --expect")
	}
}
