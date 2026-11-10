package cli

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type accountRow struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Dir      string `json:"dir"`
	Default  bool   `json:"default"`
	SignedIn *bool  `json:"signedIn"`
	Added    string `json:"added"`
}

// accountHarness is a temporary HOME with no provider command on PATH, so the
// login email comes from the account details file.
func accountHarness(t *testing.T, email string) *harness {
	t.Helper()
	h := newHarness(t)
	t.Setenv("PATH", t.TempDir())
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("CODEX_HOME", "")
	h.mkdir(filepath.Join(h.home, ".claude"))
	if email != "" {
		h.loginAs(email)
	}
	return h
}

func (h *harness) loginAs(email string) {
	h.write(filepath.Join(h.home, ".claude.json"), `{"oauthAccount":{"emailAddress":"`+email+`"}}`)
}

func (h *harness) accounts(args ...string) []accountRow {
	h.t.Helper()
	if code := h.run(h.home, append([]string{"account"}, append(args, "--json")...)...); code != 0 {
		h.t.Fatalf("grid account %v exited %d: %s", args, code, h.errb.String())
	}
	var doc struct {
		Accounts []accountRow `json:"accounts"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &doc); err != nil {
		h.t.Fatalf("%v: %s", err, h.out.String())
	}
	return doc.Accounts
}

func TestAccountStatusRegistersEachEmailOnceAndMovesTheDefault(t *testing.T) {
	h := accountHarness(t, "Ann@Example.com")
	for i := 0; i < 2; i++ {
		got := h.accounts("status")
		if len(got) != 1 || got[0].ID != "claude-ann" || got[0].Email != "ann@example.com" || !got[0].Default || got[0].Added != "auto" {
			t.Fatalf("run %d: %+v", i, got)
		}
		if got[0].Dir != filepath.Join(h.home, ".claude-ann") {
			t.Fatalf("own folder: %+v", got[0])
		}
	}
	if st, err := os.Stat(filepath.Join(h.home, ".claude-ann")); err != nil || !st.IsDir() {
		t.Fatalf("the folder was not made: %v", err)
	}
	h.loginAs("bob@example.com")
	got := h.accounts("status")
	if len(got) != 2 || got[0].Default || !got[1].Default || got[1].ID != "claude-bob" {
		t.Fatalf("the tag moves to the new email: %+v", got)
	}
	if again := h.accounts("status"); len(again) != 2 {
		t.Fatalf("still two after another call: %+v", again)
	}
	// list answers from the records and registers nothing.
	h.loginAs("carol@example.com")
	if listed := h.accounts("list"); len(listed) != 2 || listed[0].Default || listed[1].Default {
		t.Fatalf("list: %+v", listed)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".claude-carol")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("list made a folder: %v", err)
	}
}

func TestAccountStatusJSONHasTheShapeTheDecisionFixed(t *testing.T) {
	h := accountHarness(t, "ann@example.com")
	if code := h.run(h.home, "account", "status", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var doc struct {
		Accounts []map[string]any `json:"accounts"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &doc); err != nil || len(doc.Accounts) != 1 {
		t.Fatalf("%v: %s", err, h.out.String())
	}
	var keys []string
	for k := range doc.Accounts[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if got := strings.Join(keys, ","); got != "added,checked,default,dir,email,id,kind,name,plan,signedIn" {
		t.Fatalf("keys: %s", got)
	}
	if code := h.run(h.home, "account", "scan", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var scan struct {
		Folders []map[string]any `json:"folders"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &scan); err != nil || len(scan.Folders) != 2 {
		t.Fatalf("%v: %s", err, h.out.String())
	}
	keys = nil
	for k := range scan.Folders[0] {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if got := strings.Join(keys, ","); got != "dir,email,home,kind,plan,signedIn" {
		t.Fatalf("scan keys: %s", got)
	}
}

func TestAccountFolderIsIdempotent(t *testing.T) {
	h := accountHarness(t, "")
	created := func() bool {
		if code := h.run(h.home, "account", "folder", "--kind", "claude", "--id", "claude-work", "--json"); code != 0 {
			t.Fatal(h.errb.String())
		}
		var res struct {
			Dir     string `json:"dir"`
			Created bool   `json:"created"`
		}
		if err := json.Unmarshal(h.out.Bytes(), &res); err != nil || res.Dir != filepath.Join(h.home, ".claude-work") {
			t.Fatalf("%v: %s", err, h.out.String())
		}
		return res.Created
	}
	if !created() || created() {
		t.Fatal("the first run creates the folder and the second reports created:false")
	}
	if code := h.run(h.home, "account", "folder", "--kind", "claude"); code != 2 {
		t.Fatalf("a missing --id is a usage error, got %d", code)
	}
	if code := h.run(h.home, "account", "folder", "--kind", "claude", "--id", "ssh"); code != 1 {
		t.Fatalf("an id that is not claude-<name> is refused, got %d", code)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".ssh")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a refused id made a folder: %v", err)
	}
}

func TestAccountWindowsSaysNoDataThenRoundTripsTheStatusLine(t *testing.T) {
	h := accountHarness(t, "")
	if code := h.run(h.home, "account", "windows", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	if out := h.out.String(); !strings.Contains(out, `"source":"none"`) || !strings.Contains(out, `"windows":[]`) || strings.Contains(out, `"used"`) {
		t.Fatalf("with no source the answer is no data, never 0: %s", out)
	}
	if code := h.run(h.home, "account", "windows"); code != 0 || !strings.Contains(h.out.String(), "no data") {
		t.Fatalf("plain output: %d %q", code, h.out.String())
	}
	reset := time.Now().Add(time.Hour).Unix()
	h.env.Stdin = strings.NewReader(`{"session_id":"s","rate_limits":{"five_hour":{"used_percentage":12,"resets_at":` + itoa(reset) + `},"seven_day":{"used_percentage":55,"resets_at":` + itoa(reset+9) + `}}}`)
	if code := h.run(h.home, "account", "note"); code != 0 || h.out.Len() != 0 {
		t.Fatalf("note prints nothing: %d %q %s", code, h.out.String(), h.errb.String())
	}
	if code := h.run(h.home, "account", "windows", "--json", "--id", "claude"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var doc struct {
		Accounts []struct {
			ID      string `json:"id"`
			Source  string `json:"source"`
			AsOf    string `json:"asOf"`
			Windows []struct {
				Label    string  `json:"label"`
				Used     float64 `json:"used"`
				ResetsAt int64   `json:"resetsAt"`
			} `json:"windows"`
		} `json:"accounts"`
	}
	if err := json.Unmarshal(h.out.Bytes(), &doc); err != nil || len(doc.Accounts) != 1 {
		t.Fatalf("%v: %s", err, h.out.String())
	}
	a := doc.Accounts[0]
	if a.Source != "statusline" || a.AsOf == "" || len(a.Windows) != 2 || a.Windows[0].Label != "5h" || a.Windows[0].Used != 12 || a.Windows[1].Label != "7d" || a.Windows[1].ResetsAt != reset+9 {
		t.Fatalf("%+v", a)
	}
}

func TestAccountWritesNoSecretToTheGridHome(t *testing.T) {
	h := accountHarness(t, "ann@example.com")
	h.write(filepath.Join(h.home, ".claude", ".credentials.json"), `{"accessToken":"SECRET-TOKEN-77"}`)
	h.write(filepath.Join(h.home, ".claude.json"), `{"userID":"SECRET-TOKEN-77","oauthAccount":{"emailAddress":"ann@example.com"}}`)
	h.accounts("status")
	h.run(h.home, "account", "windows", "--json")
	for _, root := range []string{h.gridHome, filepath.Join(h.home, ".claude-ann")} {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if data, err := os.ReadFile(path); err == nil && strings.Contains(string(data), "SECRET-TOKEN-77") {
					t.Errorf("%s holds a secret", path)
				}
			}
			return nil
		})
	}
	if strings.Contains(h.out.String(), "SECRET-TOKEN-77") {
		t.Fatal("a secret reached the output")
	}
}

func TestAccountUnknownCommandIsAUsageError(t *testing.T) {
	h := accountHarness(t, "")
	if code := h.run(h.home, "account", "frobnicate"); code != 2 {
		t.Fatalf("got %d", code)
	}
	if code := h.run(h.home, "account"); code != 0 || !strings.Contains(h.out.String(), "usage: grid account") {
		t.Fatalf("help: %d %q", code, h.out.String())
	}
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }
