package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

// These tests run the real writers of grid setup, project scan, add, remove, repo and
// project-md set with --json in a temp home and check each stdout against its cmd-
// schema. UPDATE_YN_FIXTURES=1 rewrites the fixtures from that real output.

var (
	ynSpULID = regexp.MustCompile(`\b[0-9A-HJKMNP-TV-Z]{26}\b`)
	ynSpSHA  = regexp.MustCompile(`\b[0-9a-f]{40}\b`)
)

// ynSpNew makes a harness in an owner's temp home with no inherited Grid or Claude settings.
func ynSpNew(t *testing.T) *harness {
	t.Helper()
	h := newHarness(t)
	for _, key := range strings.Fields("GRID_AGENT GRID_WORKER_ID GRID_WORKER GRID_LEASE GRID_LEASE_EPOCH GRID_WAKE GRID_FEED_TASK GRID_JOB GRID_ATTEMPT GRID_SESSION_ID CLAUDE_SESSION_ID GRID_OWNER GRID_PROJECTS") {
		t.Setenv(key, "")
	}
	for _, key := range []string{"GRID_MEMBER", "GRID_ACTOR", "GRID_WHO"} {
		t.Setenv(key, "member")
	}
	t.Setenv("GRID_ROLE", "owner")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(h.home, ".claude"))
	return h
}

// ynSpRun runs grid and returns a copy of stdout; a failing exit fails the test.
func ynSpRun(t *testing.T, h *harness, cwd string, args ...string) []byte {
	t.Helper()
	if code := h.run(cwd, args...); code != 0 {
		t.Fatalf("grid %s: exit %d: %s", strings.Join(args, " "), code, h.errb)
	}
	return append([]byte(nil), h.out.Bytes()...)
}

// ynSpCheckout makes a git checkout under <root>/work and returns its folder.
func ynSpCheckout(t *testing.T, h *harness, name, remote string) string {
	t.Helper()
	dir := filepath.Join(h.root, "work", name)
	h.mkdir(dir)
	testgit.Init(t, dir, remote, true)
	return dir
}

// ynSpRegister registers a checkout as a project and points the harness at this machine.
func ynSpRegister(t *testing.T, h *harness, name, remote string) string {
	t.Helper()
	dir := ynSpCheckout(t, h, name, remote)
	ynSpRun(t, h, dir, "project", "add", "--name", name)
	h.env.Machine = gridhome.Machine(h.gridHome)
	return dir
}

func ynSpValid(t *testing.T, schema string, out []byte) {
	t.Helper()
	if err := contract.Validate(schema, out); err != nil {
		t.Fatalf("%s rejects the real output: %v\n%s", schema, err, out)
	}
}

// ynSpMutate decodes raw, applies fn to the object or to every object of the array, and encodes it again.
func ynSpMutate(t *testing.T, raw []byte, fn func(map[string]any)) []byte {
	t.Helper()
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	switch x := v.(type) {
	case map[string]any:
		fn(x)
	case []any:
		if len(x) == 0 {
			t.Fatal("nothing to mutate in an empty array")
		}
		for _, item := range x {
			fn(item.(map[string]any))
		}
	default:
		t.Fatalf("unexpected output %s", raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// ynSpRequire renames each key the app reads in a copy of the real stdout and expects Validate to refuse it.
func ynSpRequire(t *testing.T, schema string, raw []byte, keys ...string) {
	t.Helper()
	ynSpValid(t, schema, raw)
	for _, key := range keys {
		renamed := ynSpMutate(t, raw, func(m map[string]any) {
			v, ok := m[key]
			if !ok {
				t.Fatalf("the real output has no %q: %s", key, raw)
			}
			delete(m, key)
			m[key+"_renamed"] = v
		})
		if err := contract.Validate(schema, renamed); err == nil {
			t.Errorf("%s accepts %s renamed to %s_renamed: %s", schema, key, key, renamed)
		}
	}
}

// ynSpWrite writes one fixture when UPDATE_YN_FIXTURES=1, with the temp folder, ids and hashes made fixed.
func ynSpWrite(t *testing.T, h *harness, schema, kind, name string, data []byte) {
	t.Helper()
	if os.Getenv("UPDATE_YN_FIXTURES") != "1" {
		return
	}
	text := strings.ReplaceAll(string(data), h.root, "/home/example")
	text = ynSpULID.ReplaceAllString(text, "01HZX3EXAMPLE000000000000")
	text = ynSpSHA.ReplaceAllString(text, "e3b0c44298fc1c149afbf4c8996fb92427ae41e4")
	var v any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	if kind == "valid" {
		ynSpValid(t, schema, b)
	} else if contract.Validate(schema, b) == nil {
		t.Fatalf("%s/%s must be refused", schema, name)
	}
	dir := filepath.Join("..", "..", "contract", "fixtures", schema, kind)
	h.mkdir(dir)
	if err := os.WriteFile(filepath.Join(dir, name+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// ynSpInvalid writes the two invalid fixtures from a real output: one without key, one with key of the wrong type.
func ynSpInvalid(t *testing.T, h *harness, schema string, raw []byte, key string) {
	t.Helper()
	ynSpWrite(t, h, schema, "invalid", "missing-field", ynSpMutate(t, raw, func(m map[string]any) { delete(m, key) }))
	ynSpWrite(t, h, schema, "invalid", "wrong-type", ynSpMutate(t, raw, func(m map[string]any) { m[key] = 7 }))
}

func TestYnSetup(t *testing.T) {
	h := ynSpNew(t)
	written := ynSpRun(t, h, h.home, "setup", "--yes", "--json")
	ynSpRequire(t, "cmd-setup", written, "path", "added", "present", "written")
	var first struct {
		Written bool     `json:"written"`
		Added   []string `json:"added"`
	}
	if err := json.Unmarshal(written, &first); err != nil {
		t.Fatal(err)
	}
	if !first.Written || len(first.Added) == 0 {
		t.Fatalf("setup --yes wrote nothing: %s", written)
	}
	if _, err := os.Stat(filepath.Join(h.home, ".claude", "settings.json")); err != nil {
		t.Fatalf("setup wrote outside the temp home: %v", err)
	}
	again := ynSpRun(t, h, h.home, "setup", "--yes", "--json")
	ynSpValid(t, "cmd-setup", again)
	var second struct {
		Written bool     `json:"written"`
		Present []string `json:"present"`
	}
	if err := json.Unmarshal(again, &second); err != nil {
		t.Fatal(err)
	}
	if second.Written || len(second.Present) == 0 {
		t.Fatalf("a second setup --yes should find its rules present: %s", again)
	}
	// Without --yes and with --json setup previews and writes nothing.
	preview := ynSpRun(t, h, h.home, "setup", "--json")
	ynSpRequire(t, "cmd-setup", preview, "path", "added", "present", "written")

	// The project form records answers and answers with the project and the answers.
	cwd := ynSpRegister(t, h, "example", "https://example.com/org/example.git")
	h.write(filepath.Join(cwd, "setup-answers.json"), `{"description":"Example project","test":"go test ./..."}`)
	answers := ynSpRun(t, h, cwd, "setup", "example", "--json")
	ynSpRequire(t, "cmd-setup", answers, "project", "answers")

	ynSpWrite(t, h, "cmd-setup", "valid", "basic", written)
	ynSpWrite(t, h, "cmd-setup", "valid", "populated", again)
	ynSpWrite(t, h, "cmd-setup", "valid", "answers", answers)
	ynSpInvalid(t, h, "cmd-setup", written, "path")
}

func TestYnProjectScan(t *testing.T) {
	h := ynSpNew(t)
	root := filepath.Join(h.root, "work")
	ynSpRegister(t, h, "registered", "https://example.com/org/registered.git")
	ynSpCheckout(t, h, "fresh", "https://example.com/org/fresh.git")
	empty := filepath.Join(h.root, "empty")
	h.mkdir(empty)

	none := ynSpRun(t, h, h.home, "project", "scan", empty, "--json")
	ynSpValid(t, "cmd-project-scan", none)

	out := ynSpRun(t, h, h.home, "project", "scan", root, "--json")
	ynSpRequire(t, "cmd-project-scan", out, "name", "path", "rule", "registered")
	var rows []struct {
		Name       string `json:"name"`
		Registered bool   `json:"registered"`
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want one registered and one new checkout, got %s", out)
	}
	byName := map[string]bool{}
	for _, r := range rows {
		byName[r.Name] = r.Registered
	}
	if registered, ok := byName["registered"]; !ok || !registered {
		t.Fatalf("registered checkout not reported as registered: %s", out)
	}
	if registered, ok := byName["fresh"]; !ok || registered {
		t.Fatalf("new checkout not reported as not registered: %s", out)
	}

	applied := ynSpRun(t, h, h.home, "project", "scan", root, "--apply", "--json")
	ynSpRequire(t, "cmd-project-scan", applied, "name", "path", "rule", "registered")
	if strings.Contains(string(applied), `"registered":false`) {
		t.Fatalf("--apply left a checkout unregistered: %s", applied)
	}

	// error is optional: a row that failed under --apply carries it.
	failed := ynSpMutate(t, out, func(m map[string]any) {
		if m["registered"] == false {
			m["error"] = "not registered: another folder failed its check"
		}
	})
	ynSpValid(t, "cmd-project-scan", failed)

	ynSpWrite(t, h, "cmd-project-scan", "valid", "basic", none)
	ynSpWrite(t, h, "cmd-project-scan", "valid", "populated", out)
	ynSpWrite(t, h, "cmd-project-scan", "valid", "failed", failed)
	ynSpInvalid(t, h, "cmd-project-scan", out, "registered")
}

func TestYnProjectAdd(t *testing.T) {
	h := ynSpNew(t)
	bare := ynSpCheckout(t, h, "bare", "")
	plain := ynSpRun(t, h, bare, "project", "add", "--name", "bare", "--json")
	ynSpValid(t, "cmd-project-add", plain)

	dir := ynSpCheckout(t, h, "example", "https://example.com/org/example.git")
	out := ynSpRun(t, h, dir, "project", "add", "--name", "example", "--json")
	ynSpRequire(t, "cmd-project-add", out, "name", "id", "folders", "repos", "keys")
	var entry struct {
		Name    string           `json:"name"`
		Folders []map[string]any `json:"folders"`
		Repos   []map[string]any `json:"repos"`
		Keys    []map[string]any `json:"keys"`
	}
	if err := json.Unmarshal(out, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Name != "example" || len(entry.Folders) == 0 || len(entry.Repos) == 0 || len(entry.Keys) == 0 {
		t.Fatalf("project add did not fill the entry: %s", out)
	}

	ynSpWrite(t, h, "cmd-project-add", "valid", "basic", plain)
	ynSpWrite(t, h, "cmd-project-add", "valid", "populated", out)
	ynSpInvalid(t, h, "cmd-project-add", out, "name")
}

func TestYnProjectRepo(t *testing.T) {
	h := ynSpNew(t)
	cwd := ynSpRegister(t, h, "example", "https://example.com/org/example.git")
	added := ynSpRun(t, h, cwd, "project", "repo", "add", "example", "--where", "services/api", "--remote", "git@example.com:org/api.git", "--branch", "main", "--json")
	ynSpRequire(t, "cmd-project-repo", added, "state", "item")
	edited := ynSpRun(t, h, cwd, "project", "repo", "edit", "example", "--id", "api", "--branch", "develop", "--json")
	ynSpRequire(t, "cmd-project-repo", edited, "state", "item")
	removed := ynSpRun(t, h, cwd, "project", "repo", "remove", "example", "--id", "api", "--json")
	ynSpRequire(t, "cmd-project-repo", removed, "state")
	if strings.Contains(string(removed), "item") {
		t.Fatalf("remove answers with an item: %s", removed)
	}

	ynSpWrite(t, h, "cmd-project-repo", "valid", "basic", removed)
	ynSpWrite(t, h, "cmd-project-repo", "valid", "populated", added)
	ynSpWrite(t, h, "cmd-project-repo", "valid", "edited", edited)
	ynSpInvalid(t, h, "cmd-project-repo", added, "state")
}

func TestYnProjectMDSet(t *testing.T) {
	h := ynSpNew(t)
	cwd := ynSpRegister(t, h, "example", "https://example.com/org/example.git")
	text := ynSpRun(t, h, cwd, "project-md", "set", "example", "description", "Example project", "--json")
	ynSpRequire(t, "cmd-project-md-set", text, "project", "key", "value")
	list := ynSpRun(t, h, cwd, "project-md", "set", "example", "tags", "[alpha, beta]", "--json")
	ynSpRequire(t, "cmd-project-md-set", list, "project", "key", "value")

	ynSpWrite(t, h, "cmd-project-md-set", "valid", "basic", text)
	ynSpWrite(t, h, "cmd-project-md-set", "valid", "populated", list)
	ynSpInvalid(t, h, "cmd-project-md-set", text, "key")
}

func TestYnProjectRemove(t *testing.T) {
	h := ynSpNew(t)
	bare := ynSpRegister(t, h, "bare", "")
	plain := ynSpRun(t, h, bare, "project", "remove", "bare", "--json")
	ynSpValid(t, "cmd-project-remove", plain)

	dir := ynSpRegister(t, h, "example", "https://example.com/org/example.git")
	out := ynSpRun(t, h, dir, "project", "remove", "example", "--json")
	ynSpRequire(t, "cmd-project-remove", out, "name", "id", "folders", "repos", "keys")
	var entry struct {
		Name    string `json:"name"`
		Folders []any  `json:"folders"`
	}
	if err := json.Unmarshal(out, &entry); err != nil {
		t.Fatal(err)
	}
	if entry.Name != "example" || len(entry.Folders) != 0 {
		t.Fatalf("remove should unbind this machine's folder and keep the entry: %s", out)
	}

	ynSpWrite(t, h, "cmd-project-remove", "valid", "basic", plain)
	ynSpWrite(t, h, "cmd-project-remove", "valid", "populated", out)
	ynSpInvalid(t, h, "cmd-project-remove", out, "name")
}
