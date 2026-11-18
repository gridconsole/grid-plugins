package settings

import (
	"encoding/json"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// retiredDenies are the deny rules setup wrote before agents were allowed to write
// task folders, the brain, project.md, the settings files and the setup folder,
// as the earlier builder wrote them (home ~/.grid, projects folder <home>/projects).
var retiredDenies = []string{
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[tT][aA][sS][kK][sS]/[Tt]-[0-9]*/**)",
	"Edit(~/.grid/[pP][eE][rR][sS][oO][nN][aA][lL]/[tT][aA][sS][kK][sS]/[Tt]-[0-9]*/**)",
	"Edit(~/.grid/*/[tT][aA][sS][kK][sS]/[Tt]-[0-9]*/**)",
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**)",
	"Edit(~/.grid/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**)",
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[pP][rR][oO][jJ][eE][cC][tT].[mM][dD])",
	"Edit(~/.grid/[sS][eE][tT][tT][iI][nN][gG][sS].[jJ][sS][oO][nN])",
	"Edit(~/.grid/*/[sS][eE][tT][tT][iI][nN][gG][sS].[jJ][sS][oO][nN])",
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[sS][eE][tT][tT][iI][nN][gG][sS].[jJ][sS][oO][nN])",
	"Edit(~/.grid/.[sS][eE][tT][tT][iI][nN][gG][sS].[lL][oO][cC][kK])",
	"Edit(~/.grid/*/.[sS][eE][tT][tT][iI][nN][gG][sS].[lL][oO][cC][kK])",
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/.[sS][eE][tT][tT][iI][nN][gG][sS].[lL][oO][cC][kK])",
	"Edit(~/.grid/[wW][oO][rR][kK][fF][lL][oO][wW][sS].[mM][dD])",
	"Edit(~/.grid/*/[wW][oO][rR][kK][fF][lL][oO][wW][sS].[mM][dD])",
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[wW][oO][rR][kK][fF][lL][oO][wW][sS].[mM][dD])",
	"Edit(~/.grid/.[hH][oO][sS][tT]/**)",
	"Edit(~/.grid/setup/**)",
	"Write(~/.grid/setup/**)",
}

// legacyDenies are the forms the first releases wrote for the task, knowledge and
// project.md rules.
var legacyDenies = []string{
	"Edit(~/.grid/projects/*/tasks/[Tt]-[0-9]*/**)",
	"Edit(~/.grid/personal/tasks/[Tt]-[0-9]*/**)",
	"Edit(~/.grid/projects/*/knowledge/**)",
	"Edit(~/.grid/knowledge/**)",
	"Edit(~/.grid/projects/*/project.md)",
	"Edit(~/.grid/projects/*/[tT][aA][sS][kK][sS]/[Tt]-[0-9]*/**)",
	"Edit(~/.grid/projects/*/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**)",
	"Edit(~/.grid/[kK][nN][oO][wW][lL][eE][dD][gG][eE]/**)",
	"Edit(~/.grid/projects/*/[pP][rR][oO][jJ][eE][cC][tT].[mM][dD])",
}

// keptDenies are the denies that stay: a project's setup answers and held commands,
// and the person's approval identity.
var keptDenies = []string{
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[sS][eE][tT][uU][pP]-[aA][nN][sS][wW][eE][rR][sS].[jJ][sS][oO][nN])",
	"Write(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[sS][eE][tT][uU][pP]-[aA][nN][sS][wW][eE][rR][sS].[jJ][sS][oO][nN])",
	"Edit(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[sS][eE][tT][uU][pP]-[pP][eE][nN][dD][iI][nN][gG].[jJ][sS][oO][nN])",
	"Write(~/.grid/[pP][rR][oO][jJ][eE][cC][tT][sS]/*/[sS][eE][tT][uU][pP]-[pP][eE][nN][dD][iI][nN][gG].[jJ][sS][oO][nN])",
	"Edit(~/.grid-person/**)",
	"Write(~/.grid-person/**)",
}

func denyRules(values []string) []ClaudeRule {
	var rules []ClaudeRule
	for _, v := range values {
		rules = append(rules, ClaudeRule{"permissions", "deny", v})
	}
	return rules
}

func (f *claudeFixture) denies() []string {
	f.t.Helper()
	doc, err := parseClaudeObject(f.read(f.path))
	if err != nil {
		f.t.Fatal(err)
	}
	values, _, err := claudeValues(doc, ClaudeRule{"permissions", "deny", ""})
	if err != nil {
		f.t.Fatal(err)
	}
	return values
}

func (f *claudeFixture) recordRules() []ClaudeRule {
	f.t.Helper()
	var record claudeRecord
	if err := json.Unmarshal(f.read(f.record), &record); err != nil {
		f.t.Fatal(err)
	}
	return record.Rules
}

// seedOldSetup leaves what an earlier setup wrote, plus the person's own rules: the
// settings file carries every old deny, the record owns them.
func (f *claudeFixture) seedOldSetup() {
	f.t.Helper()
	mine := []string{"Read(~/.ssh/**)", "Edit(~/.grid/tasks-i-keep/**)"}
	denies, err := json.Marshal(slices.Concat(mine, legacyDenies, retiredDenies, keptDenies))
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(f.path, `{"theme":"dark","permissions":{"allow":["Bash(grid *)","Read(~/mine/**)"],"deny":`+string(denies)+`}}`)
	record := claudeRecord{Path: f.path, Rules: denyRules(slices.Concat(legacyDenies, retiredDenies, keptDenies))}
	record.Rules = append(record.Rules, ClaudeRule{"permissions", "allow", "Bash(grid *)"})
	data, err := json.Marshal(record)
	if err != nil {
		f.t.Fatal(err)
	}
	f.write(f.record, string(data))
}

func TestClaudeRetiredIsTheEarlierBlock(t *testing.T) {
	f := newClaudeFixture(t)
	retired, err := claudeRetired(f.o.Home, f.o.GridHome, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retired, denyRules(append(slices.Clone(retiredDenies), legacyDenies...))) {
		t.Fatalf("retired rules differ from the earlier setup's:\n%v", retired)
	}
	block, err := claudeBlock(f.o.Home, f.o.GridHome, "")
	if err != nil {
		t.Fatal(err)
	}
	var deny []string
	for _, rule := range block {
		if rule.Section == "permissions" && rule.Key == "deny" {
			deny = append(deny, rule.Value)
		}
		if slices.Contains(retired, rule) {
			t.Fatalf("setup still writes a retired rule: %s", rule.Value)
		}
	}
	if !reflect.DeepEqual(deny, keptDenies) {
		t.Fatalf("setup denies %v, want only %v", deny, keptDenies)
	}
}

func TestClaudeSetupTakesRetiredDeniesOut(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "json"}[asJSON], func(t *testing.T) {
			f := newClaudeFixture(t)
			f.seedOldSetup()
			f.o.Yes, f.o.JSON = true, asJSON
			f.wantCode(0)
			want := append([]string{"Read(~/.ssh/**)", "Edit(~/.grid/tasks-i-keep/**)"}, keptDenies...)
			if got := f.denies(); !reflect.DeepEqual(got, want) {
				t.Fatalf("denies %v, want %v", got, want)
			}
			doc, _ := parseClaudeObject(f.read(f.path))
			if theme := string(doc.get("theme")); theme != `"dark"` {
				t.Fatalf("unrelated setting changed: %s", theme)
			}
			for _, rule := range f.recordRules() {
				if slices.Contains(denyRules(retiredDenies), rule) {
					t.Fatalf("record still owns a retired rule: %s", rule.Value)
				}
			}
			f.wantCode(0) // a second run has nothing left to do
			if f.denies()[0] != "Read(~/.ssh/**)" || len(f.denies()) != len(want) {
				t.Fatalf("second run changed the denies: %v", f.denies())
			}
		})
	}
}

func TestClaudeSetupPreviewListsRetiredDenies(t *testing.T) {
	f := newClaudeFixture(t)
	f.seedOldSetup()
	f.wantCode(0) // not a terminal and no --yes: shows what it would do, writes nothing
	for _, value := range retiredDenies {
		if !slices.Contains(f.denies(), value) {
			t.Fatalf("preview run removed %s", value)
		}
		if !strings.Contains(f.out.String(), "- "+value+"\n") {
			t.Fatalf("preview does not list %s:\n%s", value, &f.out)
		}
	}
}

func TestClaudeRemoveTakesRetiredDeniesToo(t *testing.T) {
	f := newClaudeFixture(t)
	f.seedOldSetup()
	f.o.Yes, f.o.Remove = true, true
	f.wantCode(0)
	if got, want := f.denies(), []string{"Read(~/.ssh/**)", "Edit(~/.grid/tasks-i-keep/**)"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("denies %v, want %v", got, want)
	}
	if _, err := os.Stat(f.record); !os.IsNotExist(err) {
		t.Fatalf("record stays after removal: %v", err)
	}
}

func TestRetireClaudeRules(t *testing.T) {
	retire := func(f *claudeFixture) {
		t.Helper()
		if err := RetireClaudeRules(f.o); err != nil {
			t.Fatal(err)
		}
		if f.out.Len() != 0 || f.err.Len() != 0 {
			t.Fatalf("output: %q %q", &f.out, &f.err)
		}
	}
	t.Run("takes the retired rules out of the settings and the record", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.seedOldSetup()
		retire(f)
		want := append([]string{"Read(~/.ssh/**)", "Edit(~/.grid/tasks-i-keep/**)"}, keptDenies...)
		if got := f.denies(); !reflect.DeepEqual(got, want) {
			t.Fatalf("denies %v, want %v", got, want)
		}
		for _, rule := range f.recordRules() {
			if slices.Contains(denyRules(slices.Concat(legacyDenies, retiredDenies)), rule) {
				t.Fatalf("record still owns %s", rule.Value)
			}
		}
		before, _ := os.Stat(f.path)
		retire(f)
		after, _ := os.Stat(f.path)
		if !after.ModTime().Equal(before.ModTime()) {
			t.Fatal("a run with nothing to retire rewrote the settings")
		}
	})
	t.Run("deletes a record that held only retired rules", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.seedOldSetup()
		data, _ := json.Marshal(claudeRecord{Path: f.path, Rules: denyRules(retiredDenies)})
		f.write(f.record, string(data))
		retire(f)
		if _, err := os.Stat(f.record); !os.IsNotExist(err) {
			t.Fatalf("empty record stays: %v", err)
		}
		if got := len(f.denies()); got != 2+len(keptDenies) {
			t.Fatalf("%d denies left: %v", got, f.denies())
		}
	})
	t.Run("does nothing without a setup record", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.seedOldSetup()
		if err := os.Remove(f.record); err != nil {
			t.Fatal(err)
		}
		before := f.read(f.path)
		retire(f)
		if string(f.read(f.path)) != string(before) {
			t.Fatal("settings changed without a record")
		}
	})
	t.Run("cleans the file the record names, whichever account runs it", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.seedOldSetup()
		f.o.ConfigDir = f.o.Home + "/.claude-other"
		retire(f)
		if got := len(f.denies()); got != 2+len(keptDenies) {
			t.Fatalf("%d denies left: %v", got, f.denies())
		}
		if _, err := os.Stat(f.o.ConfigDir + "/settings.json"); !os.IsNotExist(err) {
			t.Fatalf("the running account's settings were created: %v", err)
		}
	})
	t.Run("leaves an invalid settings file alone", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.seedOldSetup()
		f.write(f.path, `{"permissions":`)
		retire(f)
		if string(f.read(f.path)) != `{"permissions":` {
			t.Fatalf("settings changed: %s", f.read(f.path))
		}
	})
	t.Run("never adds a rule", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.seedOldSetup()
		f.write(f.path, `{"permissions":{"deny":["Read(~/.ssh/**)"]}}`)
		retire(f)
		if got := f.denies(); !reflect.DeepEqual(got, []string{"Read(~/.ssh/**)"}) {
			t.Fatalf("denies %v", got)
		}
	})
}
