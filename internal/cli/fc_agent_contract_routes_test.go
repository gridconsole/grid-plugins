package cli

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

type fcAgentSub struct {
	Name  string   `json:"name"`
	Args  []string `json:"args"`
	Flags []struct {
		Name string `json:"name"`
	} `json:"flags"`
}

// fcAgentSubs returns the subcommands the contract holds under "agent", by name.
func fcAgentSubs(t *testing.T) map[string]fcAgentSub {
	t.Helper()
	var doc struct {
		Commands []struct {
			Name        string       `json:"name"`
			Subcommands []fcAgentSub `json:"subcommands"`
		} `json:"commands"`
	}
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatalf("contract/commands.json: %v", err)
	}
	for _, c := range doc.Commands {
		if c.Name != "agent" {
			continue
		}
		subs := map[string]fcAgentSub{}
		for _, s := range c.Subcommands {
			subs[s.Name] = s
		}
		return subs
	}
	t.Fatal("contract/commands.json has no agent command")
	return nil
}

func fcAgentFlags(s fcAgentSub) []string {
	var names []string
	for _, f := range s.Flags {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func fcAgentHas(flags []string, want ...string) bool {
	have := map[string]bool{}
	for _, f := range flags {
		have[f] = true
	}
	for _, w := range want {
		if !have[w] {
			return false
		}
	}
	return true
}

func TestFcAgentRoutesInContract(t *testing.T) {
	subs := fcAgentSubs(t)
	var got []string
	for name := range subs {
		got = append(got, name)
	}
	sort.Strings(got)
	want := []string{"budget apply", "budget show", "budget undo", "check", "list", "set", "sync", "unset"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("agent subcommands in contract = %v, want %v", got, want)
	}
	apply := subs["budget apply"]
	if !fcAgentHas(fcAgentFlags(apply), "--dry-run", "--with", "--expect") {
		t.Errorf("agent budget apply flags = %v, want --dry-run, --with, --expect", fcAgentFlags(apply))
	}
	if !reflect.DeepEqual(apply.Args, []string{"<economic|balanced|high>"}) {
		t.Errorf("agent budget apply args = %v, want one <economic|balanced|high>", apply.Args)
	}
	if sync := subs["sync"]; !fcAgentHas(fcAgentFlags(sync), "--templates", "--dir") {
		t.Errorf("agent sync flags = %v, want --templates and --dir", fcAgentFlags(sync))
	}
	for _, name := range []string{"budget show", "budget undo"} {
		if len(subs[name].Args) != 0 {
			t.Errorf("agent %s args = %v, want none", name, subs[name].Args)
		}
	}
}

// TestFcAgentAppReadIsNamed: the app (internal/slices/agentreads/http.go:75) calls
// `grid agent budget show --json`, so the contract has to name that route.
func TestFcAgentAppReadIsNamed(t *testing.T) {
	show, ok := fcAgentSubs(t)["budget show"]
	if !ok {
		t.Fatal(`contract/commands.json lacks "agent budget show", which the app calls (internal/slices/agentreads/http.go:75)`)
	}
	if !fcAgentHas(fcAgentFlags(show), "--json") {
		t.Errorf("agent budget show flags = %v, want --json", fcAgentFlags(show))
	}
}

func TestFcAgentBudgetExits(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{{"agent", "budget", "nope"}, {"agent", "budget", "apply"}} {
		if code := h.run(h.tmp, args...); code != 2 {
			t.Errorf("grid %s: exit %d, want 2 (stderr %q)", strings.Join(args, " "), code, h.errb.String())
		}
	}
	if code := h.run(h.tmp, "agent", "budget", "--help"); code != 0 {
		t.Fatalf("grid agent budget --help: exit %d, want 0: %s", code, h.errb.String())
	}
	out := h.out.String()
	for _, want := range []string{"Usage: grid agent budget show [--json]", "grid agent budget apply <economic|balanced|high>", "grid agent budget undo [--json]"} {
		if !strings.Contains(out, want) {
			t.Errorf("agent budget --help output lacks %q:\n%s", want, out)
		}
	}
}
