package cli

import (
	"slices"
	"sort"
	"strings"
	"testing"
)

// hsIpWalk compares table with cmds: missing are commands with no entry, extra
// are entries that name no command.
func hsIpWalk(table map[string]hsIpVerb, cmds []command) (missing, extra []string) {
	names := map[string]bool{}
	for _, c := range cmds {
		names[c.name] = true
		if _, ok := table[c.name]; !ok {
			missing = append(missing, c.name)
		}
	}
	for name := range table {
		if !names[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return missing, extra
}

// hsIpCopy is a shallow copy of table that a sub-test may change.
func hsIpCopy(table map[string]hsIpVerb) map[string]hsIpVerb {
	out := make(map[string]hsIpVerb, len(table))
	for k, v := range table {
		out[k] = v
	}
	return out
}

func TestHsIpWalksEveryCommand(t *testing.T) {
	cmds := commands()
	missing, extra := hsIpWalk(hsIpTable, cmds)
	for _, name := range missing {
		t.Errorf("command %q has no in-process answer in hsIpTable", name)
	}
	for _, name := range extra {
		t.Errorf("hsIpTable names %q, which commands() does not hold", name)
	}
	t.Logf("walked %d commands, %d table entries", len(cmds), len(hsIpTable))

	t.Run("deleted entry is reported", func(t *testing.T) {
		for _, c := range cmds {
			table := hsIpCopy(hsIpTable)
			delete(table, c.name)
			missing, extra := hsIpWalk(table, cmds)
			if !slices.Equal(missing, []string{c.name}) || len(extra) != 0 {
				t.Errorf("without %q: missing %v extra %v, want missing [%s]", c.name, missing, extra, c.name)
			}
		}
	})
	t.Run("stray entry is reported", func(t *testing.T) {
		table := hsIpCopy(hsIpTable)
		table["no-such-verb"] = hsIpVerb{}
		missing, extra := hsIpWalk(table, cmds)
		if len(missing) != 0 || !slices.Equal(extra, []string{"no-such-verb"}) {
			t.Errorf("missing %v extra %v, want extra [no-such-verb]", missing, extra)
		}
	})
	t.Run("an argv of a deleted verb fails closed", func(t *testing.T) {
		table := hsIpCopy(hsIpTable)
		delete(table, "version")
		if ok, why := hsIpAnswer(table, []string{"version"}); ok || why == "" {
			t.Errorf("version with no entry = %v %q, want false with a why", ok, why)
		}
	})
}

// hsIpBinaryOnly are the argv the brief names as process-tied.
var hsIpBinaryOnly = [][]string{
	{"worker", "next"},
	{"job", "next"},
	{"feed", "tick"},
	{"job", "verify"},
	{"eval", "run"},
	{"env", "up"},
	{"land"},
	{"agent", "sync"},
	{"task", "attach", "x"},
	{"lease", "take", "task", "--hold"},
	{"task", "new", "x"},
	{"mcp"},
	{"hook"},
	{"open"},
	{"routine", "run", "x"},
	{"account", "sign-in"},
}

// hsIpSafe are argv that only read and write the Grid home.
var hsIpSafe = [][]string{
	{"version"},
	{"where", "--json"},
	{"feed", "state", "--json", "--task", "T-01"},
	{"feed", "read", "--task", "T-01"},
	{"person", "check", "--json"},
	{"question", "list"},
	{"approve", "--help"},
	{"task", "--help"},
}

// hsIpUnknown are argv the table cannot answer.
var hsIpUnknown = [][]string{
	{},
	{"nonsense"},
	{"task", "nonsense"},
}

func TestHsIpBinaryOnly(t *testing.T) {
	for _, args := range hsIpBinaryOnly {
		ok, why := InProcess(args)
		if ok || why == "" {
			t.Errorf("InProcess(%q) = %v %q, want false with a why", args, ok, why)
		}
	}
	// The same calls behind global flags, and with --hold spelled as the flag package accepts it.
	for _, args := range [][]string{
		{"--member", "m", "worker", "next"},
		{"--grid-home=/g", "--role", "member", "land"},
		{"lease", "take", "task", "--task", "T-01", "-hold"},
		{"lease", "take", "task", "--hold=true"},
		{"account", "note", "--then", "true"},
	} {
		if ok, why := InProcess(args); ok || why == "" {
			t.Errorf("InProcess(%q) = %v %q, want false with a why", args, ok, why)
		}
	}
}

func TestHsIpSafe(t *testing.T) {
	for _, args := range append(slices.Clone(hsIpSafe),
		[]string{"--member", "m", "feed", "state", "--json"},
		[]string{"lease", "take", "task", "--task", "T-01"},
		[]string{"help"},
	) {
		if ok, why := InProcess(args); !ok {
			t.Errorf("InProcess(%q) = false %q, want true", args, why)
		}
	}
}

func TestHsIpFailClosed(t *testing.T) {
	for _, args := range append(slices.Clone(hsIpUnknown),
		[]string{"task"},
		[]string{"--member"},
		[]string{"--member", "m"},
		[]string{"--json", "version"},
	) {
		ok, why := InProcess(args)
		if ok || strings.TrimSpace(why) == "" {
			t.Errorf("InProcess(%q) = %v %q, want false with a why", args, ok, why)
		}
	}
}
