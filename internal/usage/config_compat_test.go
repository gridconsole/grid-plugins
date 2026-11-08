package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigurationUnknownFields(t *testing.T) {
	want := defaults()
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["future_option"] = json.RawMessage(`{"enabled":true,"values":[null,1,"new"]}`)
	for _, tc := range []struct {
		name, field, value string
	}{
		{name: "unknown optional field"},
		{"bad version", "v", `2`},
		{"bad type", "minutes", `"bad"`},
		{"bad percent", "dollars", `[101]`},
		{"unsorted", "tokens", `[80,50]`},
		{"duplicates", "minutes", `[50,50]`},
		{"missing required value", "dollars", `null`},
		{"bad budget", "budget", `{"tokens":-1}`},
		{"quoted budget", "budget", `{"tokens":"100"}`},
		{"overflowing budget", "budget", `{"tokens":18446744073709551616}`},
		{"bad baseline", "baseline", `{"dollars":0,"minutes":0,"tokens":1.5}`},
		{"bad fired entry", "fired", `{"tokens:80":"invalid"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy := make(map[string]json.RawMessage, len(fields))
			for key, value := range fields {
				copy[key] = value
			}
			if tc.field != "" {
				copy[tc.field] = json.RawMessage(tc.value)
			}
			data, err := json.Marshal(copy)
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "thresholds.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			got, err := readConfig(dir)
			if tc.field != "" {
				if err == nil {
					t.Fatal("accepted invalid known field alongside unknown optional field")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unknown field changed settings: got %+v, want %+v", got, want)
			}
		})
	}
}
