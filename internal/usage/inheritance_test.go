package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func storedThresholds(t *testing.T, s Store) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(s.TaskPath, "usage", "thresholds.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestOnlyExplicitThresholdsPersist(t *testing.T) {
	for _, unit := range []string{"dollars", "minutes", "tokens"} {
		for _, clear := range []bool{false, true} {
			t.Run(unit+map[bool]string{false: " set", true: " clear"}[clear], func(t *testing.T) {
				s := noticeStore(t)
				s.UsageNoticeThresholds = &NoticeThresholds{Dollars: []float64{90, 100}, Minutes: []float64{90, 100}, Tokens: []float64{90, 100}}
				values := []float64{80, 100}
				if clear {
					values = []float64{}
				}
				change := Change{}
				switch unit {
				case "dollars":
					change.Dollars = values
				case "minutes":
					change.Minutes = values
				case "tokens":
					change.Tokens = values
				}
				configure(t, s, change)
				configure(t, s, Change{Budget: true, Dollars: []float64{100}, Minutes: []float64{100}, BudgetTokens: []uint64{100}})
				post(t, s, 100, 100)
				postNoticeTokens(t, s, 100)
				fields := storedThresholds(t, s)
				for _, name := range []string{"dollars", "minutes", "tokens"} {
					raw, present := fields[name]
					if present != (name == unit) {
						t.Fatalf("stored inherited unit %s: %s", name, raw)
					}
					if present {
						var got []float64
						if err := json.Unmarshal(raw, &got); err != nil || !reflect.DeepEqual(got, values) {
							t.Fatalf("explicit %s = %s: %v", unit, raw, err)
						}
					}
				}
				// Removing workflow values restores defaults for every inherited unit,
				// while explicit values (including []) survive notices and budget clears.
				s.UsageNoticeThresholds = nil
				c := configure(t, s, Change{Budget: true, Dollars: []float64{}, Minutes: []float64{}, BudgetTokens: []uint64{}})
				for name, got := range map[string][]float64{"dollars": c.Dollars, "minutes": c.Minutes, "tokens": c.Tokens} {
					want := []float64{50, 80, 100}
					if name == "tokens" {
						want = []float64{80, 100}
					}
					if name == unit {
						want = values
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("%s = %v, want %v", name, got, want)
					}
				}
			})
		}
	}
}

func TestLegacyThresholdsRemainAuthoritative(t *testing.T) {
	for _, withTokens := range []bool{false, true} {
		t.Run(map[bool]string{false: "pre-token", true: "all-units"}[withTokens], func(t *testing.T) {
			s := noticeStore(t)
			old := defaults()
			data, err := json.Marshal(old)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(data, &fields); err != nil {
				t.Fatal(err)
			}
			if !withTokens {
				delete(fields, "tokens")
			}
			data, err = json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(s.TaskPath, "usage")
			if err := os.Mkdir(dir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "thresholds.json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			s.UsageNoticeThresholds = &NoticeThresholds{Dollars: []float64{90, 100}, Minutes: []float64{90, 100}, Tokens: []float64{90, 100}}
			configure(t, s, Change{Budget: true, BudgetTokens: []uint64{100}})
			s.UsageNoticeThresholds.Tokens = []float64{95, 100}
			c, err := s.Config()
			wantTokens := []float64{95, 100}
			if withTokens {
				wantTokens = old.Tokens
			}
			if err != nil || !reflect.DeepEqual(c.Dollars, old.Dollars) || !reflect.DeepEqual(c.Minutes, old.Minutes) || !reflect.DeepEqual(c.Tokens, wantTokens) {
				t.Fatalf("legacy values changed: %+v %v", c, err)
			}
			if _, present := storedThresholds(t, s)["tokens"]; present != withTokens {
				t.Fatal("legacy absent tokens became explicit")
			}
		})
	}
}

func TestSparseConfigurationValidation(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		valid       bool
	}{
		{"inherited", "", true},
		{"explicit clear", `,"tokens":[]`, true},
		{"future option", `,"future_option":{"values":[null,1]}`, true},
		{"null", `,"tokens":null`, false},
		{"bad type", `,"dollars":"bad"`, false},
		{"out of range", `,"minutes":[101]`, false},
		{"unsorted", `,"tokens":[100,80]`, false},
		{"duplicate", `,"dollars":[80,80]`, false},
		{"missing budget", `,"budget":null`, false},
		{"bad baseline", `,"baseline":{"dollars":-1,"minutes":0}`, false},
		{"bad fired", `,"fired":{"tokens:80":"bad"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data := `{"v":2,"budget":{},"baseline":{"dollars":0,"minutes":0,"tokens":0},"fired":{}` + tc.extra + `}`
			if err := os.WriteFile(filepath.Join(dir, "thresholds.json"), []byte(data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readConfig(dir)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
		})
	}
}
