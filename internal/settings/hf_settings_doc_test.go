package settings

import (
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"
)

// hfSdKnownOpen lists mismatches where the code, not the doc, is wrong. Each key
// is a mismatch string from the checks below; an entry that no longer
// mismatches fails the test. Empty today: every drift was in the doc.
var hfSdKnownOpen = map[string]string{}

var hfSdTick = regexp.MustCompile("`([^`]+)`")

func hfSdDoc(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GRID_HOME", home)
	b, err := os.ReadFile("../../docs/settings.md")
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(string(b), "\n")
}

// hfSdTable returns the data rows (cells trimmed) of the table whose header row
// starts with the given first and second cell.
func hfSdTable(t *testing.T, lines []string, h1, h2 string) [][]string {
	t.Helper()
	var rows [][]string
	in := false
	for _, l := range lines {
		if !strings.HasPrefix(l, "|") {
			if in {
				break
			}
			continue
		}
		cells := strings.Split(strings.Trim(strings.TrimSpace(l), "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if !in {
			if len(cells) >= 2 && cells[0] == h1 && cells[1] == h2 {
				in = true
			}
			continue
		}
		if strings.HasPrefix(cells[0], "---") {
			continue
		}
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		t.Fatalf("table %q | %q not found or empty", h1, h2)
	}
	return rows
}

func hfSdSetEq(want, got []string) []string {
	var out []string
	for _, w := range want {
		if !slices.Contains(got, w) {
			out = append(out, "missing in doc: "+w)
		}
	}
	for _, g := range got {
		if !slices.Contains(want, g) {
			out = append(out, "not in catalog: "+g)
		}
	}
	return out
}

// hfSdJudge fails on mismatches not in the known-open list and on known-open
// entries that no longer mismatch.
func hfSdJudge(t *testing.T, kind string, mism []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, m := range mism {
		key := kind + ": " + m
		seen[key] = true
		if _, ok := hfSdKnownOpen[key]; !ok {
			t.Errorf("%s", key)
		}
	}
	var stale []string
	for k := range hfSdKnownOpen {
		if strings.HasPrefix(k, kind+": ") && !seen[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	for _, k := range stale {
		t.Errorf("known-open entry no longer wrong, remove it: %s", k)
	}
}

func TestHfSdHooks(t *testing.T) {
	rows := hfSdTable(t, hfSdDoc(t), "Hook", "Default")
	var mism, got []string
	for _, r := range rows {
		if len(r) < 2 {
			t.Fatalf("short hook row %v", r)
		}
		for _, name := range strings.Split(r[0], ",") {
			name = strings.Trim(strings.TrimSpace(name), "`")
			if slices.Contains(retiredHooks, name) {
				mism = append(mism, "retired hook in table: "+name)
				continue
			}
			if slices.Contains(got, name) {
				mism = append(mism, "duplicate row: "+name)
			}
			got = append(got, name)
			if slices.Contains(hooks, name) && r[1] != defaultLevel(name) {
				mism = append(mism, "default of "+name+" is "+r[1]+", catalog "+defaultLevel(name))
			}
		}
	}
	mism = append(mism, hfSdSetEq(HookNames(), got)...)
	hfSdJudge(t, "hooks", mism)
}

func TestHfSdAgents(t *testing.T) {
	rows := hfSdTable(t, hfSdDoc(t), "Agent", "Default model")
	var mism, got []string
	for _, r := range rows {
		if len(r) < 3 {
			t.Fatalf("short agent row %v", r)
		}
		got = append(got, r[0])
		i := slices.IndexFunc(agents, func(a agentDefault) bool { return a.name == r[0] })
		if i < 0 {
			continue
		}
		if agents[i].model != r[1] {
			mism = append(mism, "model of "+r[0]+" is "+r[1]+", catalog "+agents[i].model)
		}
		if agents[i].effort != r[2] {
			mism = append(mism, "effort of "+r[0]+" is "+r[2]+", catalog "+agents[i].effort)
		}
	}
	mism = append(mism, hfSdSetEq(AgentNames(), got)...)
	hfSdJudge(t, "agents", mism)
}

func TestHfSdValues(t *testing.T) {
	rows := hfSdTable(t, hfSdDoc(t), "Field", "Allowed values")
	var mism []string
	found := false
	for _, r := range rows {
		if r[0] != "Hook level" {
			continue
		}
		found = true
		cell := r[1]
		list, rest, _ := strings.Cut(cell, ";")
		var lv []string
		for _, m := range hfSdTick.FindAllStringSubmatch(list, -1) {
			lv = append(lv, m[1])
		}
		mism = append(mism, hfSdSetEq(levels, lv)...)

		// The clause naming the block defaults: backticked names before
		// "default to `block`".
		before, _, ok := strings.Cut(rest, "default to `block`")
		if !ok {
			t.Fatalf("no 'default to `block`' clause in %q", cell)
		}
		_, clause, _ := strings.Cut(before, "and ")
		var blockDoc, blockCat []string
		for _, m := range hfSdTick.FindAllStringSubmatch(clause, -1) {
			blockDoc = append(blockDoc, m[1])
		}
		for _, h := range hooks {
			if defaultLevel(h) == "block" {
				blockCat = append(blockCat, h)
			}
		}
		for _, m := range hfSdSetEq(blockCat, blockDoc) {
			mism = append(mism, "block defaults, "+m)
		}
	}
	if !found {
		t.Fatal("Hook level row not found")
	}
	hfSdJudge(t, "values", mism)
}
