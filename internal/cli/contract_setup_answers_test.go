package cli

import (
	"encoding/json"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
)

// grid setup <project> [--answers <file|->] reads --answers; the contract must
// list it, with a value, and the [project] operand, so a skill that points at the
// flag is checked against the inventory.
func TestContractSetupAnswersListedInCommands(t *testing.T) {
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatalf("contract/commands.json does not parse: %v", err)
	}
	var setup *contractCommandEntry
	for i := range doc.Commands {
		if doc.Commands[i].Name == "setup" {
			setup = &doc.Commands[i]
		}
	}
	if setup == nil || len(setup.Subcommands) != 1 || setup.Subcommands[0].Name != "" {
		t.Fatalf("setup must have exactly one subcommand with an empty name: %+v", setup)
	}
	sub := setup.Subcommands[0]
	if len(sub.Args) != 1 || sub.Args[0] != "[project]" {
		t.Errorf("setup args = %v, want [[project]]", sub.Args)
	}
	got := map[string]string{}
	for _, f := range sub.Flags {
		got[f.Name] = f.Takes
	}
	if takes, ok := got["--answers"]; !ok {
		t.Errorf("commands.json does not list --answers under setup; flags: %v", got)
	} else if takes != "value" {
		t.Errorf("setup --answers takes %q, want value", takes)
	}
	for _, name := range []string{"--json", "--remove", "--sandbox", "--yes"} {
		if _, ok := got[name]; !ok {
			t.Errorf("setup lost flag %s; flags: %v", name, got)
		}
	}
	if _, ok := got["--__contract_unknown"]; ok {
		t.Errorf("setup lists the unknown-flag probe")
	}
}
