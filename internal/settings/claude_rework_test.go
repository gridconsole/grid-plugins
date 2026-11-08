package settings

import (
	"bytes"
	"encoding/json"
	"reflect"
	"slices"
	"testing"
)

func TestClaudeRemoveRejectsForgedOwnership(t *testing.T) {
	for _, asJSON := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "json"}[asJSON], func(t *testing.T) {
			f := newClaudeFixture(t)
			f.write(f.path, `{"permissions":{"deny":["Read(~/.ssh/**)","Bash(curl *)"]}}`)
			f.o.Yes = true
			f.wantCode(0)
			var record claudeRecord
			if err := json.Unmarshal(f.read(f.record), &record); err != nil {
				t.Fatal(err)
			}
			forged := []ClaudeRule{
				{"permissions", "deny", "Read(~/.ssh/**)"},
				{"permissions", "deny", "Bash(curl *)"},
				// A genuine block value in the wrong array is not owned either.
				{"permissions", "deny", "Bash(grid *)"},
			}
			doc, _ := parseClaudeObject(f.read(f.path))
			if err := changeClaudeRule(&doc, forged[2], false); err != nil {
				t.Fatal(err)
			}
			f.write(f.path, string(doc.bytes()))
			record.Rules = append(record.Rules, forged...)
			data, err := json.Marshal(record)
			if err != nil {
				t.Fatal(err)
			}
			f.write(f.record, string(data))
			f.o.Remove, f.o.JSON = true, asJSON
			f.wantCode(0)
			doc, _ = parseClaudeObject(f.read(f.path))
			deny, _, err := claudeValues(doc, forged[0])
			if err != nil || !reflect.DeepEqual(deny, []string{forged[0].Value, forged[1].Value, forged[2].Value}) {
				t.Fatalf("personal denies changed: %v (%v)", deny, err)
			}
			var warning bytes.Buffer
			for _, rule := range forged {
				warning.WriteString("not a Grid rule, left in place: " + rule.Value + "\n")
			}
			if f.err.String() != warning.String() {
				t.Fatalf("warnings: %q", &f.err)
			}
		})
	}
}

func TestClaudeRemoveDefaultsWithPersonalEnvironment(t *testing.T) {
	for _, initial := range []string{`{}`, `{"autoMode":{"environment":["$defaults"]}}`} {
		for _, added := range []bool{false, true} {
			t.Run(initial+map[bool]string{false: " unchanged", true: " extended"}[added], func(t *testing.T) {
				f := newClaudeFixture(t)
				f.write(f.path, initial)
				f.o.Yes = true
				f.wantCode(0)
				personal := ClaudeRule{"autoMode", "environment", "Keep my own context"}
				if added {
					doc, _ := parseClaudeObject(f.read(f.path))
					if err := changeClaudeRule(&doc, personal, false); err != nil {
						t.Fatal(err)
					}
					f.write(f.path, string(doc.bytes()))
				}
				f.o.Remove = true
				f.wantCode(0)
				doc, _ := parseClaudeObject(f.read(f.path))
				values, exists, err := claudeValues(doc, personal)
				keepDefaults := added || initial != `{}`
				if err != nil || slices.Contains(values, "$defaults") != keepDefaults || slices.Contains(values, personal.Value) != added || exists != keepDefaults {
					t.Fatalf("environment=%v exists=%v err=%v", values, exists, err)
				}
			})
		}
	}
}
