package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

// The output schemas of grid account status, windows, scan, folder and statusline
// (and list, which shares the status writer). Every stdout here is the real
// writer's, on a fake machine: PATH holds no claude or codex, so the provider is
// never run, and both config folder variables point under the harness root.

var yhAcStamp = regexp.MustCompile(`\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ`)

var yhAcCommands = []string{"status", "windows", "scan", "folder", "statusline"}

func yhAcCodexAuth(email, plan string) string {
	b64 := base64.RawURLEncoding.EncodeToString
	claims := fmt.Sprintf(`{"email":%q,"https://api.openai.com/auth":{"chatgpt_plan_type":%q}}`, email, plan)
	return fmt.Sprintf(`{"tokens":{"id_token":"%s.%s.sig"}}`, b64([]byte(`{}`)), b64([]byte(claims)))
}

// yhAcNew is a harness whose account folders all live under its root. Populated
// has a signed-in-by-details Claude home (ann), a Claude config folder (bob) and
// a Codex folder (cara).
func yhAcNew(t *testing.T, populated bool) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	if !populated {
		return h
	}
	claudeDir, codexDir := filepath.Join(h.root, "cfg-bob"), filepath.Join(h.root, "codex-cara")
	h.write(filepath.Join(h.home, ".claude.json"), `{"oauthAccount":{"emailAddress":"Ann@Example.com"}}`)
	h.write(filepath.Join(h.home, ".claude", "settings.json"), `{}`)
	h.write(filepath.Join(claudeDir, ".claude.json"), `{"oauthAccount":{"emailAddress":"bob@example.com"}}`)
	h.write(filepath.Join(codexDir, "auth.json"), yhAcCodexAuth("cara@example.com", "plus"))
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	t.Setenv("CODEX_HOME", codexDir)
	return h
}

func yhAcRun(t *testing.T, h *harness, args ...string) []byte {
	t.Helper()
	if code := h.run(h.home, args...); code != 0 {
		t.Fatalf("grid %s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return append([]byte(nil), h.out.Bytes()...)
}

// yhAcCapture runs one command with --json and answers its stdout.
func yhAcCapture(t *testing.T, h *harness, name string, populated bool) []byte {
	t.Helper()
	switch name {
	case "status", "scan":
		return yhAcRun(t, h, "account", name, "--json")
	case "windows":
		if populated {
			yhAcRun(t, h, "account", "status", "--json") // registers ann, bob and cara
			h.env.Stdin = strings.NewReader(`{"rate_limits":{"five_hour":{"used_percentage":42,"resets_at":4102444800},"seven_day":{"used_percentage":18.5,"resets_at":4102444800}}}`)
			yhAcRun(t, h, "account", "note") // keeps bob's numbers: CLAUDE_CONFIG_DIR
		}
		return yhAcRun(t, h, "account", "windows", "--json")
	case "folder":
		if populated {
			yhAcRun(t, h, "account", "folder", "--kind", "claude", "--id", "claude-dan", "--json")
		}
		return yhAcRun(t, h, "account", "folder", "--kind", "claude", "--id", "claude-dan", "--json")
	case "statusline":
		if populated {
			h.write(filepath.Join(os.Getenv("CLAUDE_CONFIG_DIR"), "settings.json"), `{"statusLine":{"type":"command","command":"my-line"}}`)
			return yhAcRun(t, h, "account", "statusline", "on", "--json")
		}
		return yhAcRun(t, h, "account", "statusline", "--json")
	}
	t.Fatalf("unknown command %q", name)
	return nil
}

func yhAcDecode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("stdout is not one JSON object: %v: %s", err, raw)
	}
	return doc
}

func yhAcList(doc map[string]any, key string) []map[string]any {
	var out []map[string]any
	items, _ := doc[key].([]any)
	for _, item := range items {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// yhAcWindowed is the first account entry that has a window, and that window.
func yhAcWindowed(doc map[string]any) (account, window map[string]any) {
	for _, a := range yhAcList(doc, "accounts") {
		if ws := yhAcList(a, "windows"); len(ws) > 0 {
			return a, ws[0]
		}
	}
	return nil, nil
}

// yhAcPopulatedHolds keeps a populated run from passing on an empty answer.
func yhAcPopulatedHolds(t *testing.T, name string, doc map[string]any) {
	t.Helper()
	switch name {
	case "status":
		if n := len(yhAcList(doc, "accounts")); n != 3 {
			t.Errorf("status accounts = %d, want 3 (ann, bob, cara)", n)
		}
	case "windows":
		if _, w := yhAcWindowed(doc); w == nil || w["resetsAt"] == nil {
			t.Errorf("windows has no window with resetsAt: %v", doc)
		}
	case "scan":
		if n := len(yhAcList(doc, "folders")); n != 3 {
			t.Errorf("scan folders = %d, want 3", n)
		}
	case "folder":
		if doc["created"] != false {
			t.Errorf("second folder run created = %v, want false", doc["created"])
		}
	case "statusline":
		if doc["on"] != true || doc["kept"] != "my-line" {
			t.Errorf("statusline = %v, want on with the kept command", doc)
		}
	}
}

func TestYhAccountStdoutValidates(t *testing.T) {
	for _, name := range yhAcCommands {
		t.Run(name, func(t *testing.T) {
			for _, variant := range []string{"basic", "populated"} {
				t.Run(variant, func(t *testing.T) {
					populated := variant == "populated"
					h := yhAcNew(t, populated)
					out := yhAcCapture(t, h, name, populated)
					if err := contract.Validate("cmd-account-"+name, out); err != nil {
						t.Fatalf("%v: %s", err, out)
					}
					doc := yhAcDecode(t, out)
					if populated {
						yhAcPopulatedHolds(t, name, doc)
					}
					if name == "folder" && !populated && doc["created"] != true {
						t.Errorf("first folder run created = %v, want true", doc["created"])
					}
					if os.Getenv("UPDATE_CONTRACT_COMMAND_FIXTURES") == "1" {
						yhAcWriteFixtures(t, name, variant, string(out), h.root)
					}
				})
			}
		})
	}
}

// account list writes through the status writer; its stdout is the same object.
func TestYhAccountListIsStatus(t *testing.T) {
	for _, variant := range []string{"basic", "populated"} {
		t.Run(variant, func(t *testing.T) {
			h := yhAcNew(t, variant == "populated")
			yhAcRun(t, h, "account", "status", "--json") // registers, so list has rows
			status := yhAcDecode(t, yhAcRun(t, h, "account", "status", "--json"))
			list := yhAcRun(t, h, "account", "list", "--json")
			if err := contract.Validate("cmd-account-status", list); err != nil {
				t.Fatalf("%v: %s", err, list)
			}
			got := yhAcDecode(t, list)
			for _, doc := range []map[string]any{status, got} {
				for _, a := range yhAcList(doc, "accounts") {
					delete(a, "checked") // the clock differs by a second
				}
			}
			if !reflect.DeepEqual(status, got) {
				t.Errorf("list differs from status:\nlist   %v\nstatus %v", got, status)
			}
			if variant == "populated" && len(yhAcList(got, "accounts")) != 3 {
				t.Errorf("list accounts = %v, want 3", got)
			}
		})
	}
}

// The fields the app reads stay required: a renamed key fails Validate. A key the
// writer leaves out when zero (windows[].resetsAt, omitempty) is in properties,
// not in required, so a rename fails by the closed object and a removal is valid.
func TestYhAccountRequiredFields(t *testing.T) {
	for _, tc := range []struct {
		name, key string
		target    func(map[string]any) map[string]any
		required  bool
	}{
		{"status", "id", func(d map[string]any) map[string]any { return yhAcList(d, "accounts")[0] }, true},
		{"status", "signedIn", func(d map[string]any) map[string]any { return yhAcList(d, "accounts")[0] }, true},
		{"windows", "resetsAt", func(d map[string]any) map[string]any { _, w := yhAcWindowed(d); return w }, false},
		{"scan", "dir", func(d map[string]any) map[string]any { return yhAcList(d, "folders")[0] }, true},
	} {
		t.Run(tc.name+"/"+tc.key, func(t *testing.T) {
			h := yhAcNew(t, true)
			doc := yhAcDecode(t, yhAcCapture(t, h, tc.name, true))
			schema := "cmd-account-" + tc.name
			check := func(d map[string]any) error {
				b, err := json.Marshal(d)
				if err != nil {
					t.Fatal(err)
				}
				return contract.Validate(schema, b)
			}
			if err := check(doc); err != nil {
				t.Fatalf("unchanged stdout: %v", err)
			}
			obj := tc.target(doc)
			value, ok := obj[tc.key]
			if !ok {
				t.Fatalf("stdout has no %s: %v", tc.key, obj)
			}
			delete(obj, tc.key)
			obj[tc.key+"X"] = value
			if err := check(doc); err == nil {
				t.Errorf("%s renamed to %sX is accepted", tc.key, tc.key)
			}
			delete(obj, tc.key+"X")
			if err := check(doc); (err != nil) != tc.required {
				t.Errorf("%s removed: Validate = %v, required %v", tc.key, err, tc.required)
			}
		})
	}
}

// yhAcMutations are the fixtures made from the populated stdout: more valid
// shapes, and the invalid ones (a required key gone, a key of the wrong type).
var yhAcMutations = map[string][]struct {
	file  string
	valid bool
	edit  func(map[string]any)
}{
	"status": {
		{"valid/signed-in-states", true, func(d map[string]any) {
			a := yhAcList(d, "accounts")
			a[0]["signedIn"], a[1]["signedIn"], a[2]["signedIn"] = true, false, nil
		}},
		{"invalid/missing-field", false, func(d map[string]any) { delete(yhAcList(d, "accounts")[0], "signedIn") }},
		{"invalid/wrong-type", false, func(d map[string]any) { yhAcList(d, "accounts")[0]["default"] = "yes" }},
		{"invalid/renamed-field", false, func(d map[string]any) {
			a := yhAcList(d, "accounts")[0]
			a["accountId"] = a["id"]
			delete(a, "id")
		}},
	},
	"windows": {
		{"valid/stream-without-reset", true, func(d map[string]any) {
			a, w := yhAcWindowed(d)
			a["source"] = "stream"
			delete(w, "resetsAt")
		}},
		{"invalid/missing-field", false, func(d map[string]any) { delete(yhAcList(d, "accounts")[0], "source") }},
		{"invalid/wrong-type", false, func(d map[string]any) { _, w := yhAcWindowed(d); w["used"] = "42" }},
		{"invalid/renamed-field", false, func(d map[string]any) {
			_, w := yhAcWindowed(d)
			w["resets_at"] = w["resetsAt"]
			delete(w, "resetsAt")
		}},
	},
	"scan": {
		{"valid/signed-states", true, func(d map[string]any) {
			f := yhAcList(d, "folders")
			f[0]["signedIn"], f[1]["signedIn"] = false, nil
		}},
		{"invalid/missing-field", false, func(d map[string]any) { delete(yhAcList(d, "folders")[0], "dir") }},
		{"invalid/wrong-type", false, func(d map[string]any) { yhAcList(d, "folders")[0]["home"] = "yes" }},
	},
	"folder": {
		{"invalid/missing-field", false, func(d map[string]any) { delete(d, "created") }},
		{"invalid/wrong-type", false, func(d map[string]any) { d["dir"] = 5 }},
	},
	"statusline": {
		{"invalid/missing-field", false, func(d map[string]any) { delete(d, "kept") }},
		{"invalid/wrong-type", false, func(d map[string]any) { d["on"] = "yes" }},
	},
}

// yhAcWriteFixtures writes valid/<variant>.json from live stdout, with the temp
// root and the clock made synthetic, and (for populated) the mutated fixtures.
// Run it through make: make test UPDATE_CONTRACT_COMMAND_FIXTURES=1 "GOFLAGS=-count=1 -run=TestYhAccountStdoutValidates".
func yhAcWriteFixtures(t *testing.T, name, variant, stdout, root string) {
	t.Helper()
	text := strings.ReplaceAll(stdout, root, "/home/example/grid-test")
	text = yhAcStamp.ReplaceAllString(text, "2026-11-12T10:00:00Z")
	dir := filepath.Join("..", "..", "contract", "fixtures", "cmd-account-"+name)
	schema := "cmd-account-" + name
	write := func(file string, doc map[string]any, valid bool) {
		b, err := json.Marshal(doc)
		if err != nil {
			t.Fatal(err)
		}
		if err := contract.Validate(schema, b); (err == nil) != valid {
			t.Fatalf("fixture %s: Validate = %v, want valid %v", file, err, valid)
		}
		contractCommandWrite(t, filepath.Join(dir, file+".json"), doc)
	}
	write("valid/"+variant, yhAcDecode(t, []byte(text)), true)
	if variant != "populated" {
		return
	}
	for _, m := range yhAcMutations[name] {
		doc := yhAcDecode(t, []byte(text))
		m.edit(doc)
		write(m.file, doc, m.valid)
	}
}
