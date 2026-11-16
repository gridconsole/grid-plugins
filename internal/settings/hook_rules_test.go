package settings

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

// wiredEvents reads hooks/hooks.json: each event it wires and the matchers it gives.
func wiredEvents(t *testing.T) map[string][]string {
	t.Helper()
	b, err := os.ReadFile("../../hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]struct {
			Matcher string `json:"matcher"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for event, entries := range doc.Hooks {
		for _, e := range entries {
			out[event] = append(out[event], e.Matcher)
		}
	}
	return out
}

// The table names exactly the rules grid hook levels lists, and its levels follow
// the catalog default when a level is not stored.
func TestHookRulesCoverEveryHook(t *testing.T) {
	rules := HookRules(map[string]string{})
	names := HookNames()
	if len(rules) != len(names) {
		t.Fatalf("%d rules, want %d", len(rules), len(names))
	}
	for i, r := range rules {
		if r.Name != names[i] || r.Level != defaultLevel(r.Name) || r.Desc == "" || r.Events == nil {
			t.Errorf("rule %d = %+v, want %s at %s with a description and a non-nil events list", i, r, names[i], defaultLevel(names[i]))
		}
	}
	if got := HookRules(map[string]string{"plan-guard": "off"})[6]; got.Name != "plan-guard" || got.Level != "off" {
		t.Errorf("a stored level is not read: %+v", got)
	}
}

// Every event the table gives a rule is an event hooks/hooks.json wires, with the
// matcher it gives that event (none for an event with no matcher).
func TestHookRuleEventsAreWired(t *testing.T) {
	wired := wiredEvents(t)
	for _, r := range HookRules(nil) {
		for _, e := range r.Events {
			matchers, ok := wired[e.Event]
			if !ok {
				t.Errorf("%s runs on %s, which hooks/hooks.json does not wire", r.Name, e.Event)
				continue
			}
			if !slices.Contains(matchers, e.Matcher) {
				t.Errorf("%s: %s has matcher %q in the table; hooks/hooks.json gives %q", r.Name, e.Event, e.Matcher, matchers)
			}
		}
	}
}
