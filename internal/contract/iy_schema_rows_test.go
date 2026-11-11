package contract_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// iySchemaUnlinked returns the schema file names in dir that the doc text does
// not link as `](schemas/<name>)`. A name starting with fmt- must be linked
// from formatsText; any other from readmeText.
func iySchemaUnlinked(t *testing.T, dir, readmeText, formatsText string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var missing []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		doc := readmeText
		if strings.HasPrefix(name, "fmt-") {
			doc = formatsText
		}
		if !strings.Contains(doc, "](schemas/"+name+")") {
			missing = append(missing, name)
		}
	}
	return missing
}

func TestIySchemaRows(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join("..", "..", "contract", "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	formats, err := os.ReadFile(filepath.Join("..", "..", "contract", "formats.md"))
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join("..", "..", "contract", "schemas")
	for _, name := range iySchemaUnlinked(t, dir, string(readme), string(formats)) {
		t.Errorf("schema %s has no row in the contract docs", name)
	}
}

func TestIySchemaRowsNegative(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a.json", "unlinked.json", "fmt-x.json", "fmt-y.json"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	readme := "[a](schemas/a.json) [x](schemas/fmt-x.json)"
	formats := "[y](schemas/fmt-y.json)"
	got := strings.Join(iySchemaUnlinked(t, dir, readme, formats), ",")
	// unlinked.json is in neither doc; fmt-x.json is linked only from the README.
	if want := "fmt-x.json,unlinked.json"; got != want {
		t.Fatalf("unlinked = %q, want %q", got, want)
	}
}
