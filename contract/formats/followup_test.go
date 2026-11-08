package formats_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/contract"
)

func TestFormatOriginStrict(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stamp any
	}{
		{"null", nil},
		{"missing", map[string]any{}},
		{"unstamped-note", brain.Note{}.Origin},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := contract.Validate("fmt-origin", formatJSON(t, tc.stamp)); err == nil {
				t.Fatal("origin schema accepted an absent stamp")
			}
		})
	}
}

func TestFormatManifestMachineULIDs(t *testing.T) {
	const machineID = "01K9J000000000000000000002"
	var manifest map[string]any
	data := formatRead(t, filepath.Join("..", "fixtures", "fmt-sync-manifest", "valid", "cut-1.json"))
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	machines := manifest["machines"].([]any)
	machines[0].(map[string]any)["id"] = machineID
	machines[1].(map[string]any)["id"] = "01K9J000000000000000000003"
	project := manifest["projects"].([]any)[0].(map[string]any)
	task := project["tasks"].([]any)[0].(map[string]any)
	task["machine"] = machineID
	if err := contract.Validate("fmt-sync-manifest", formatJSON(t, manifest)); err != nil {
		t.Fatalf("valid ULID control: %v", err)
	}
	for _, field := range []struct {
		name   string
		object map[string]any
		key    string
	}{
		{"machines-id", machines[0].(map[string]any), "id"},
		{"tasks-machine", task, "machine"},
	} {
		t.Run(field.name, func(t *testing.T) {
			for _, bad := range []string{"laptop", "01k9j000000000000000000002", "01K9J00000000000000000002", "01K9J0000000000000000000020", "01K9I000000000000000000002"} {
				t.Run(bad, func(t *testing.T) {
					field.object[field.key] = bad
					defer func() { field.object[field.key] = machineID }()
					if err := contract.Validate("fmt-sync-manifest", formatJSON(t, manifest)); err == nil {
						t.Fatalf("manifest accepted non-ULID %q", bad)
					}
				})
			}
		})
	}
}
