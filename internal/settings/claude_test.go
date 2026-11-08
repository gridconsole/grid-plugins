package settings

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
)

type claudeFixture struct {
	t            *testing.T
	o            ClaudeSetupOptions
	out, err     bytes.Buffer
	path, record string
}

func newClaudeFixture(t *testing.T) *claudeFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(home, ".claude"))
	f := &claudeFixture{t: t}
	f.o = ClaudeSetupOptions{Home: home, GridHome: filepath.Join(home, ".grid"), ConfigDir: os.Getenv("CLAUDE_CONFIG_DIR"), Stdin: strings.NewReader(""), Stdout: &f.out, Stderr: &f.err}
	f.path = filepath.Join(f.o.ConfigDir, "settings.json")
	f.record = filepath.Join(f.o.GridHome, "setup", "claude-block.json")
	return f
}
func (f *claudeFixture) run() int {
	f.out.Reset()
	f.err.Reset()
	return RunClaudeSetup(f.o)
}
func (f *claudeFixture) write(path, data string) {
	f.t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), 0640); err != nil {
		f.t.Fatal(err)
	}
}
func (f *claudeFixture) read(path string) []byte {
	f.t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		f.t.Fatal(err)
	}
	return b
}
func (f *claudeFixture) wantCode(want int) {
	f.t.Helper()
	if got := f.run(); got != want {
		f.t.Fatalf("code %d want %d; stdout=%s stderr=%s", got, want, &f.out, &f.err)
	}
}
func (f *claudeFixture) golden(name string) {
	f.t.Helper()
	want := f.read(filepath.Join("testdata", "claude", name))
	if !bytes.Equal(f.out.Bytes(), want) {
		f.t.Fatalf("output:\n%s\nwant:\n%s", &f.out, want)
	}
}
func TestClaudeFreshAndSecondRun(t *testing.T) {
	f := newClaudeFixture(t)
	f.o.ConfigDir = "" // default path, still wholly inside the test HOME
	f.o.terminal = func(_ io.Reader) bool { return true }
	f.o.Stdin = strings.NewReader("yes\n")
	f.wantCode(0)
	f.golden("fresh.txt")
	if f.err.Len() != 0 {
		t.Fatal(&f.err)
	}
	for path, mode := range map[string]os.FileMode{f.path: 0600, filepath.Dir(f.path): 0700, f.record: 0600, filepath.Dir(f.record): 0700} {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != mode {
			t.Fatalf("%s mode %o", path, info.Mode().Perm())
		}
	}
	before, record := f.read(f.path), f.read(f.record)
	info, _ := os.Stat(f.path)
	f.o.Yes = true
	f.wantCode(0)
	f.golden("second.txt")
	afterInfo, _ := os.Stat(f.path)
	if !bytes.Equal(before, f.read(f.path)) || !bytes.Equal(record, f.read(f.record)) || !os.SameFile(info, afterInfo) {
		t.Fatal("second run wrote files")
	}
}

func TestClaudePreservesKeysValuesOrderAndMode(t *testing.T) {
	f := newClaudeFixture(t)
	original := `{"z":{"n":9007199254740993,"literal":"<keep>","order":{"z":1,"a":2}},"permissions":{"ask":["Bash(rm *)"],"allow":["Read(~/mine/**)","Bash(grid *)"],"extra":{"x":true}},"a":[1,null,"x"],"autoMode":{"environment":["User policy"],"hard_deny":["No deploys"]}}`
	f.write(f.path, original)
	f.o.Yes = true
	f.wantCode(0)
	before, _ := parseClaudeObject([]byte(original))
	after, err := parseClaudeObject(f.read(f.path))
	if err != nil {
		t.Fatal(err)
	}
	keys := func(o claudeObject) []string {
		var out []string
		for _, v := range o {
			out = append(out, v.key)
		}
		return out
	}
	if !reflect.DeepEqual(keys(before), keys(after)) {
		t.Fatal("top-level order changed")
	}
	for _, key := range []string{"z", "a"} {
		if !bytes.Equal(before.get(key), after.get(key)) {
			t.Fatalf("%s changed", key)
		}
	}
	bp, _ := parseClaudeObject(before.get("permissions"))
	ap, _ := parseClaudeObject(after.get("permissions"))
	if !reflect.DeepEqual(keys(ap), []string{"ask", "allow", "extra", "deny"}) {
		t.Fatal("permission order changed")
	}
	for _, key := range []string{"ask", "extra"} {
		if !bytes.Equal(bp.get(key), ap.get(key)) {
			t.Fatalf("%s changed", key)
		}
	}
	allow, _, _ := claudeValues(after, ClaudeRule{"permissions", "allow", ""})
	if !reflect.DeepEqual(allow[:2], []string{"Read(~/mine/**)", "Bash(grid *)"}) {
		t.Fatal(allow)
	}
	env, _, _ := claudeValues(after, ClaudeRule{"autoMode", "environment", ""})
	if env[0] != "User policy" || slices.Contains(env, "$defaults") {
		t.Fatal(env)
	}
	info, _ := os.Stat(f.path)
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0640 {
		t.Fatal(info.Mode())
	}
	var record claudeRecord
	if err := json.Unmarshal(f.read(f.record), &record); err != nil {
		t.Fatal(err)
	}
	for _, r := range record.Rules {
		if r.Value == "Bash(grid *)" {
			t.Fatal("claimed preexisting rule")
		}
	}
}

func TestClaudeInvalidAndRefusals(t *testing.T) {
	for _, data := range []string{"{", "", "null", "[]", "{}{}", `{"permissions":null}`, `{"permissions":{"allow":false}}`, `{"permissions":{"deny":null}}`, `{"autoMode":{"environment":3}}`, `{"x":1,"x":2}`} {
		t.Run(data, func(t *testing.T) {
			f := newClaudeFixture(t)
			f.write(f.path, data)
			f.o.Yes = true
			f.wantCode(2)
			want := "Settings file " + f.path + " is not valid JSON; fix it or remove it. Nothing written.\n"
			if f.err.String() != want || string(f.read(f.path)) != data {
				t.Fatalf("%q %s", f.err.String(), f.read(f.path))
			}
		})
	}
	for _, answer := range []string{"n\n", "\n", "", "sure\n", "yes please\n"} {
		t.Run("answer="+answer, func(t *testing.T) {
			f := newClaudeFixture(t)
			f.write(f.path, "{}")
			f.o.Stdin = strings.NewReader(answer)
			f.o.terminal = func(_ io.Reader) bool { return true }
			f.wantCode(0)
			if string(f.read(f.path)) != "{}" || !strings.HasSuffix(f.out.String(), "Write these? [y/N] Nothing written.\n") {
				t.Fatal(&f.out)
			}
		})
	}
	t.Run("pipe", func(t *testing.T) {
		f := newClaudeFixture(t)
		f.write(f.path, "{}")
		f.o.Stdin = strings.NewReader("yes\n")
		f.wantCode(0)
		if string(f.read(f.path)) != "{}" || strings.Contains(f.out.String(), "[y/N]") || !strings.HasSuffix(f.out.String(), "Nothing written.\n") {
			t.Fatal(&f.out)
		}
	})
	t.Run("null device", func(t *testing.T) {
		f, err := os.Open(os.DevNull)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if claudeTerminal(f) {
			t.Fatal("null is not a terminal")
		}
	})
}

func TestClaudeConcurrentChange(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "install", true: "remove"}[remove], func(t *testing.T) {
			f := newClaudeFixture(t)
			f.write(f.path, "{}")
			f.o.Yes = true
			if remove {
				f.wantCode(0)
				f.o.Remove = true
			}
			f.o.beforeRename = func() { f.write(f.path, `{"changed":true}`) }
			f.wantCode(1)
			if f.err.String() != "Settings file changed while writing. Nothing written.\n" || string(f.read(f.path)) != `{"changed":true}` {
				t.Fatal(&f.err)
			}
			if remove {
				_ = f.read(f.record)
			} else if _, err := os.Stat(f.record); !os.IsNotExist(err) {
				t.Fatal("created ownership record", err)
			}
			stages, _ := filepath.Glob(filepath.Join(filepath.Dir(f.path), ".grid-claude-*"))
			if len(stages) != 0 {
				t.Fatal(stages)
			}
		})
	}
}

func TestClaudeRemoveOnlyOwnedRules(t *testing.T) {
	f := newClaudeFixture(t)
	f.write(f.path, `{"permissions":{"allow":["Bash(grid *)","Read(~/mine/**)"],"deny":["Edit(~/secret/**)"]},"other":17}`)
	f.o.Yes = true
	f.wantCode(0)
	record := f.read(f.record)
	// A second no-op must retain ownership from the first installation.
	f.wantCode(0)
	if !bytes.Equal(record, f.read(f.record)) {
		t.Fatal("record changed")
	}
	f.o.Remove = true
	f.o.Yes = false
	f.o.terminal = func(_ io.Reader) bool { return true }
	f.o.Stdin = strings.NewReader("n\n")
	before := f.read(f.path)
	f.wantCode(0)
	if !bytes.Equal(before, f.read(f.path)) || !strings.Contains(f.out.String(), "Remove these? [y/N] Nothing written.") {
		t.Fatal(&f.out)
	}
	f.o.Yes = true
	f.wantCode(0)
	if strings.Contains(f.out.String(), "- Bash(grid *)") {
		t.Fatal("removed preexisting rule")
	}
	doc, _ := parseClaudeObject(f.read(f.path))
	allow, _, _ := claudeValues(doc, ClaudeRule{"permissions", "allow", ""})
	deny, _, _ := claudeValues(doc, ClaudeRule{"permissions", "deny", ""})
	if !reflect.DeepEqual(allow, []string{"Bash(grid *)", "Read(~/mine/**)"}) || !reflect.DeepEqual(deny, []string{"Edit(~/secret/**)"}) || string(doc.get("other")) != "17" {
		t.Fatal(string(f.read(f.path)))
	}
	if _, err := os.Stat(f.record); !os.IsNotExist(err) {
		t.Fatal("record remains", err)
	}
	f.wantCode(0)
	if f.out.String() != "Nothing to remove.\n" {
		t.Fatal(&f.out)
	}
}

func TestClaudeJSON(t *testing.T) {
	f := newClaudeFixture(t)
	f.o.JSON = true
	for _, yes := range []bool{false, true, true} {
		f.o.Yes = yes
		f.wantCode(0)
		var got claudeResult
		if err := json.Unmarshal(f.out.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Path != f.path || got.Added == nil || got.Present == nil || got.Written != (yes && len(got.Added) > 0) {
			t.Fatal(&f.out)
		}
		var fields map[string]json.RawMessage
		_ = json.Unmarshal(f.out.Bytes(), &fields)
		if len(fields) != 4 {
			t.Fatal(fields)
		}
	}
}

func TestClaudeSymlinksAndNonRegularFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink privilege required")
	}
	for _, kind := range []string{"file", "directory", "dangling", "retarget"} {
		t.Run(kind, func(t *testing.T) {
			f := newClaudeFixture(t)
			target := filepath.Join(f.o.Home, "target")
			f.write(target, "{}")
			if err := os.MkdirAll(filepath.Dir(f.path), 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "directory" {
				target = f.o.Home
			}
			if kind == "dangling" {
				target += "-missing"
			}
			if err := os.Symlink(target, f.path); err != nil {
				t.Fatal(err)
			}
			f.o.Yes = true
			if kind == "retarget" {
				other := target + "-other"
				f.write(other, "{}")
				f.o.beforeRename = func() {
					if err := os.Remove(f.path); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(other, f.path); err != nil {
						t.Fatal(err)
					}
				}
			}
			if kind == "file" {
				f.wantCode(0)
				if bytes.Equal(f.read(target), []byte("{}")) {
					t.Fatal("target unchanged")
				}
			} else {
				f.wantCode(1)
			}
			info, err := os.Lstat(f.path)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatal("link replaced", err)
			}
		})
	}
}

func TestClaudeWriterReservedKindsStayInSync(t *testing.T) {
	// Any new writer predicate must be represented in ClaudeBlock and the CLI
	// parity cases before refreshing this reviewed source snapshot.
	got, err := os.ReadFile(filepath.Join("..", "guard", "writer.go"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "claude", "writer.go.golden"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("writer.go changed: reconcile ClaudeBlock and TestClaudeDenyRulesEqualWriterGuard before updating the reserved-kind snapshot")
	}
}

func TestClaudeRemoveRestoresClassifierDefaults(t *testing.T) {
	f := newClaudeFixture(t)
	f.o.Yes = true
	f.wantCode(0)
	f.o.Remove = true
	f.wantCode(0)
	doc, err := parseClaudeObject(f.read(f.path))
	if err != nil {
		t.Fatal(err)
	}
	_, exists, err := claudeValues(doc, ClaudeRule{"autoMode", "environment", ""})
	if err != nil || exists {
		t.Fatal("empty environment array would replace classifier defaults", err)
	}
}

func TestClaudeReinstallRetainsEarlierOwnership(t *testing.T) {
	f := newClaudeFixture(t)
	f.o.Yes = true
	f.wantCode(0)
	doc, _ := parseClaudeObject(f.read(f.path))
	if err := changeClaudeRule(&doc, ClaudeRule{"permissions", "allow", "Bash(grid *)"}, true); err != nil {
		t.Fatal(err)
	}
	f.write(f.path, string(doc.bytes()))
	f.wantCode(0)
	var record claudeRecord
	if err := json.Unmarshal(f.read(f.record), &record); err != nil {
		t.Fatal(err)
	}
	rules, _ := ClaudeBlock(f.o.Home, f.o.GridHome)
	if len(record.Rules) != len(rules) {
		t.Fatal(record)
	}
	f.o.Remove = true
	f.wantCode(0)
	doc, _ = parseClaudeObject(f.read(f.path))
	for _, rule := range rules {
		values, _, err := claudeValues(doc, rule)
		if err != nil || slices.Contains(values, rule.Value) {
			t.Fatal("owned rule remains", rule, err)
		}
	}
}

func TestClaudeCustomGridHomeAndInvalidRules(t *testing.T) {
	f := newClaudeFixture(t)
	rules, err := ClaudeBlock(f.o.Home, filepath.Join(f.o.Home, "data", "grid"))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rules {
		if r.Value == "Bash(grid *)" || r.Value == "$defaults" {
			continue
		}
		if !strings.Contains(r.Value, "~/data/grid") {
			t.Fatal(r)
		}
	}
	for _, grid := range []string{filepath.Dir(f.o.Home), f.o.Home, filepath.Join(f.o.Home, "grid*"), ""} {
		if _, err := ClaudeBlock(f.o.Home, grid); err == nil {
			t.Fatalf("accepted %q", grid)
		}
	}
	f.write(f.path, `{"permissions":{"allow":[null]}}`)
	f.o.Yes = true
	f.wantCode(2)
}

func TestClaudeRemoveAfterManualRemoval(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(map[bool]string{false: "entries", true: "file"}[missing], func(t *testing.T) {
			f := newClaudeFixture(t)
			f.o.Yes = true
			f.wantCode(0)
			if missing {
				if err := os.Remove(f.path); err != nil {
					t.Fatal(err)
				}
			} else {
				f.write(f.path, "{}")
			}
			f.o.Remove = true
			f.wantCode(0)
			if _, err := os.Stat(f.record); !os.IsNotExist(err) {
				t.Fatal("record remains", err)
			}
			if missing {
				if _, err := os.Stat(f.path); !os.IsNotExist(err) {
					t.Fatal("recreated settings", err)
				}
			} else if string(f.read(f.path)) != "{}" {
				t.Fatal("wrote settings")
			}
		})
	}
}

func TestClaudeRemovePreservesManualClassifierReplacement(t *testing.T) {
	f := newClaudeFixture(t)
	f.o.Yes = true
	f.wantCode(0)
	doc, err := parseClaudeObject(f.read(f.path))
	if err != nil {
		t.Fatal(err)
	}
	doc.set("autoMode", []byte(`{"environment":[]}`))
	f.write(f.path, string(doc.bytes()))
	f.o.Remove = true
	f.wantCode(0)
	doc, err = parseClaudeObject(f.read(f.path))
	if err != nil {
		t.Fatal(err)
	}
	if string(doc.get("autoMode")) != `{"environment":[]}` {
		t.Fatal("changed the user's replacement classifier context")
	}
}
