package gridconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A v3 file round-trips with every unknown key kept, at the top and in an entry.
func TestWriteKeepsUnknownKeys(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "contract", "gridconfig", "v3", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), File)
	if err := os.WriteFile(path, src, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, found, err := Read(path)
	if err != nil || !found {
		t.Fatalf("Read: found %v err %v", found, err)
	}
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	var want, got any
	_ = json.Unmarshal(src, &want)
	b, _ := os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	wb, _ := json.Marshal(want)
	gb, _ := json.Marshal(got)
	// The empty brain is not written back; every other key is.
	if strings.Replace(string(wb), `"brain":"",`, "", 1) != string(gb) {
		t.Fatalf("round trip changed the file:\nwant %s\ngot  %s", wb, gb)
	}
	if _, err := os.Stat(path + ".v3"); err == nil {
		t.Fatal("a v3 file was copied aside")
	}
}

// A v2 file is kept as config.json.v2 and rewritten as v3 when setup saves.
func TestWriteKeepsOlderFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, File)
	old := `{"v":2,"current":"acme","workspaces":[{"name":"acme","folder":"/w","member":"sam","setup":"done","step":null}]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	if b, err := os.ReadFile(path + ".v2"); err != nil || string(b) != old {
		t.Fatalf("config.json.v2 = %q, %v", b, err)
	}
	again, _, _ := Read(path)
	if again.V != Version || again.CurrentEntry() == nil || again.CurrentEntry().Member != "sam" {
		t.Fatalf("rewritten config = %+v", again)
	}
}

// A newer file is read for its known keys and never rewritten.
func TestWriteRefusesNewerFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), File)
	newer := `{"v":4,"current":"acme","workspaces":[{"name":"acme"}],"v4thing":1}`
	if err := os.WriteFile(path, []byte(newer), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, _, err := Read(path)
	if err != nil || cfg.CurrentEntry() == nil {
		t.Fatalf("Read v4: %+v %v", cfg, err)
	}
	if err := Write(path, cfg); err == nil {
		t.Fatal("Write rewrote a v4 file")
	}
	if b, _ := os.ReadFile(path); string(b) != newer {
		t.Fatalf("file changed: %s", b)
	}
}

// A v1 file migrates in memory and writes the v3 shape without the old keys.
func TestMigrateV1WritesNoOldKeys(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".grid", File)
	cfg, err := Parse([]byte(`{"workspace":"` + filepath.ToSlash(filepath.Join(home, ".grid", "old")) + `","member":"sam","setup":"done","x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Migrate(home, "") {
		t.Fatal("Migrate made no entry")
	}
	if err := Write(path, cfg); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	var top map[string]any
	_ = json.Unmarshal(b, &top)
	for _, k := range legacyKeys {
		if _, ok := top[k]; ok {
			t.Fatalf("old key %q written: %s", k, b)
		}
	}
	if top["x"] == nil || top["current"] != "old" {
		t.Fatalf("written: %s", b)
	}
}

func TestNames(t *testing.T) {
	for _, s := range []string{"acme", "a.b_c-1", "Sam"} {
		if !ValidName(s) {
			t.Errorf("%q: want valid", s)
		}
	}
	for _, s := range []string{"", ".x", "a b", "end.", "con", "COM1.txt", "lpt9", "a/b", strings.Repeat("a", 129)} {
		if ValidName(s) {
			t.Errorf("%q: want invalid", s)
		}
	}
	for in, want := range map[string]string{"Sam.Example": "sam-example", "--Ann  B--": "ann-b", "!!": ""} {
		if got := HandleFromLogin(in); got != want {
			t.Errorf("HandleFromLogin(%q) = %q, want %q", in, got, want)
		}
	}
	w := Workspace{Name: "machine"}
	if got := w.DataFolder("/home/x"); got != "" {
		t.Errorf("machine name: DataFolder = %q", got)
	}
}
