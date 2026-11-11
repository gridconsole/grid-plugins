package packaging

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stageRealPlugin(t *testing.T) string {
	t.Helper()
	plugin := filepath.Join(t.TempDir(), "plugin")
	if err := stage(filepath.Join("..", ".."), plugin, "v0.0.0-test"); err != nil {
		t.Fatal(err)
	}
	return plugin
}

func TestStagedPluginHasThePublicReadme(t *testing.T) {
	plugin := stageRealPlugin(t)
	got, err := os.ReadFile(filepath.Join(plugin, "README.md"))
	if err != nil {
		t.Fatalf("no README.md at the plugin root: %v", err)
	}
	want, err := os.ReadFile("plugin-readme.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("staged README.md differs from internal/packaging/plugin-readme.md")
	}
	source, err := os.ReadFile(filepath.Join("..", "..", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, source) {
		t.Fatal("staged README.md is the developer's source README")
	}
}

func TestPublicReadmeSaysWhatAnInstallerNeeds(t *testing.T) {
	got, err := os.ReadFile(filepath.Join(stageRealPlugin(t), "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, need := range []string{"claude plugin install", "grid setup", "--sandbox", "~/.grid", "MIT"} {
		if !strings.Contains(text, need) {
			t.Errorf("public README does not name %q", need)
		}
	}
	for _, banned := range []string{"make build", "not built yet", "/srv/", "/Users/", "/home/"} {
		if strings.Contains(text, banned) {
			t.Errorf("public README contains %q", banned)
		}
	}
}

// licenseValues collects every "license" string found anywhere in a decoded manifest.
func licenseValues(v any, out *[]any) {
	switch x := v.(type) {
	case map[string]any:
		for key, value := range x {
			if key == "license" {
				*out = append(*out, value)
			}
			licenseValues(value, out)
		}
	case []any:
		for _, value := range x {
			licenseValues(value, out)
		}
	}
}

func TestStagedLicenseIsMITEverywhere(t *testing.T) {
	plugin := stageRealPlugin(t)
	license, err := os.ReadFile(filepath.Join(plugin, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(license), "MIT License") {
		t.Fatalf("LICENSE starts with %q", firstLine(string(license)))
	}
	seen := 0
	err = filepath.WalkDir(plugin, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var manifest any
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil // not a manifest; hooks and skill fixtures need not parse
		}
		var values []any
		licenseValues(manifest, &values)
		rel, _ := filepath.Rel(plugin, path)
		for _, value := range values {
			seen++
			if value != "MIT" {
				t.Errorf("%s: license = %v, want MIT", filepath.ToSlash(rel), value)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Fatal("no staged manifest has a license key, so the check proved nothing")
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}
