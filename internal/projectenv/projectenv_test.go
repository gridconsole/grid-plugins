package projectenv

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
)

// The app pins exactly these bytes (C-ln-env-note-read).
const lnEnvExample = `---
name: stage
---
## Environments
### stage
name: Staging
kind: remote
where: stage box
runs: app
test: make smoke
secrets:
  - API_TOKEN (set)
approval: yes
`

func lnEnvBool(b bool) *bool { return &b }

func lnEnvStage() Entry {
	return Entry{ID: "stage", Name: "Staging", Kind: "remote", Where: "stage box", Runs: "app", Test: "make smoke",
		Secrets: []Secret{{Name: "API_TOKEN", Set: true}}, Approval: lnEnvBool(true)}
}

func lnEnvStore(t *testing.T) brain.Store {
	t.Helper()
	return brain.Store{GridHome: t.TempDir(), Project: "alpha"}
}

func lnEnvPath(s brain.Store, id string) string {
	return filepath.Join(s.GridHome, "projects", s.Project, "knowledge", "environments", id+".md")
}

func lnEnvRead(t *testing.T, s brain.Store, id string) []byte {
	t.Helper()
	b, err := s.ReadConfig("project", "environments", id)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func lnEnvEmptyHome(t *testing.T, s brain.Store) {
	t.Helper()
	entries, err := os.ReadDir(s.GridHome)
	if err != nil || len(entries) != 0 {
		t.Fatalf("a refusal wrote into the Grid home: %v %v", entries, err)
	}
}

func lnEnvAuditLines(t *testing.T, s brain.Store) []string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(s.GridHome, "projects", s.Project, "knowledge", "log.md"))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.Contains(l, "environments") && strings.Contains(l, "stage") {
			out = append(out, l)
		}
	}
	return out
}

func TestLnEnvExampleBytes(t *testing.T) {
	got, err := Render(lnEnvStage())
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != lnEnvExample {
		t.Fatalf("bytes differ:\n%q\nwant\n%q", got, lnEnvExample)
	}
	e, err := Parse("stage", []byte(lnEnvExample))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e, lnEnvStage()) {
		t.Fatalf("parse: %+v", e)
	}
}

func TestLnEnvRoundTripEveryField(t *testing.T) {
	in := Entry{ID: "prod-2", Name: "Production", Kind: "cloud", Where: "eu region", Runs: "api and web", Test: "make e2e",
		How: "deploy from main", From: "stage", Who: []string{"sam", "ada"},
		Secrets:  []Secret{{Name: "API_TOKEN", Set: true}, {Name: "DB_URL", Set: false}},
		Steps:    []Step{{WF: "release", Step: "build", Text: `run "make" <all> & check`}, {WF: "release", Step: "ship"}},
		Approval: lnEnvBool(false)}
	b, err := Render(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "---\nname: prod-2\n---\n## Environments\n### prod-2\nname: Production\nkind: cloud\nwhere: eu region\nruns: api and web\ntest: make e2e\nhow: deploy from main\nfrom: stage\n" +
		"who:\n  - sam\n  - ada\nsecrets:\n  - API_TOKEN (set)\n  - DB_URL (missing)\n" +
		"steps:\n  - {\"wf\":\"release\",\"step\":\"build\",\"text\":\"run \\\"make\\\" <all> & check\"}\n  - {\"wf\":\"release\",\"step\":\"ship\"}\napproval: no\n"
	if string(b) != want {
		t.Fatalf("bytes:\n%s\nwant\n%s", b, want)
	}
	out, err := Parse("prod-2", b)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip:\n%+v\n%+v", in, out)
	}
}

func TestLnEnvStringsAreStoredTrimmed(t *testing.T) {
	e := lnEnvStage()
	e.Name, e.Where, e.Who = "  Staging  ", "\tstage box ", []string{" sam "}
	b, err := Render(e)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Parse("stage", b)
	if err != nil || got.Name != "Staging" || got.Where != "stage box" || !reflect.DeepEqual(got.Who, []string{"sam"}) {
		t.Fatalf("trim: %+v %v", got, err)
	}
}

func TestLnEnvSaveStoresAConfigEntry(t *testing.T) {
	s := lnEnvStore(t)
	if err := Save(s, lnEnvStage()); err != nil {
		t.Fatal(err)
	}
	disk, err := os.ReadFile(lnEnvPath(s, "stage"))
	if err != nil || string(disk) != lnEnvExample {
		t.Fatalf("file: %q %v", disk, err)
	}
	if got := lnEnvRead(t, s, "stage"); !bytes.Equal(got, disk) {
		t.Fatalf("ReadConfig differs from the file: %q", got)
	}
	if n := len(lnEnvAuditLines(t, s)); n != 1 {
		t.Fatalf("audit lines after the first save: %d", n)
	}

	e := lnEnvStage()
	e.Test = "make smoke2"
	if err := Save(s, e); err != nil {
		t.Fatal(err)
	}
	before, after := strings.Split(lnEnvExample, "\n"), strings.Split(string(lnEnvRead(t, s, "stage")), "\n")
	if len(before) != len(after) {
		t.Fatalf("line count changed: %q", after)
	}
	for i := range before {
		if (before[i] == after[i]) == (before[i] == "test: make smoke") {
			t.Fatalf("line %d: %q became %q", i, before[i], after[i])
		}
	}
	if !strings.Contains(string(lnEnvRead(t, s, "stage")), "test: make smoke2\n") {
		t.Fatal("test line not changed")
	}
	if n := len(lnEnvAuditLines(t, s)); n != 2 {
		t.Fatalf("audit lines after the second save: %d", n)
	}

	idx, err := s.Index()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range idx.Topics {
		if row.Topic == "environments" {
			found = true
			if row.Entries != 1 || row.Invalid != 0 {
				t.Fatalf("index row: %+v", row)
			}
		}
	}
	if !found {
		t.Fatal("no environments row in the index")
	}
}

func TestLnEnvRemove(t *testing.T) {
	s := lnEnvStore(t)
	if err := Save(s, lnEnvStage()); err != nil {
		t.Fatal(err)
	}
	if err := Remove(s, "stage"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(lnEnvPath(s, "stage")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("file after remove: %v", err)
	}
	if _, err := s.ReadConfig("project", "environments", "stage"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadConfig after remove: %v", err)
	}
	if err := Remove(s, "stage"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("remove of an absent entry: %v", err)
	}
	if err := Remove(s, "Bad_Id"); err == nil {
		t.Fatal("removed a bad id")
	}
}

func TestLnEnvListIsInIdOrder(t *testing.T) {
	s := lnEnvStore(t)
	// "a-b" sorts before "a" as a file name (a-b.md) and after it as an id.
	for _, id := range []string{"b", "a-b", "a"} {
		e := lnEnvStage()
		e.ID = id
		if err := Save(s, e); err != nil {
			t.Fatal(err)
		}
	}
	got, err := List(s)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, e := range got {
		ids = append(ids, e.ID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "a-b", "b"}) {
		t.Fatalf("order: %v", ids)
	}
	if !reflect.DeepEqual(got[0], func() Entry { e := lnEnvStage(); e.ID = "a"; return e }()) {
		t.Fatalf("entry: %+v", got[0])
	}
}

func TestLnEnvListWithNoKnowledgeFolder(t *testing.T) {
	s := lnEnvStore(t)
	got, err := List(s)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("no project folder: %v %v", got, err)
	}
	if err := os.MkdirAll(filepath.Join(s.GridHome, "projects", "alpha"), 0o700); err != nil {
		t.Fatal(err)
	}
	got, err = List(s)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("project folder without knowledge: %v %v", got, err)
	}
}

func TestLnEnvRefusesSecretValues(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Entry)
		value  string
	}{
		{"secret name", func(e *Entry) { e.Secrets = []Secret{{Name: "API_TOKEN=abc123", Set: true}} }, "abc123"},
		{"test", func(e *Entry) { e.Test = `curl -H "Authorization: Bearer abc123"` }, "abc123"},
		{"where", func(e *Entry) { e.Where = "https://u:pw@host" }, "u:pw@"},
		{"runs", func(e *Entry) { e.Runs = "password: hunter2" }, "hunter2"},
		{"how", func(e *Entry) { e.How = "export API_KEY=zzz9" }, "zzz9"},
		{"from", func(e *Entry) { e.From = "secret = qqq7" }, "qqq7"},
		{"name", func(e *Entry) { e.Name = "token: www5" }, "www5"},
		{"who", func(e *Entry) { e.Who = []string{"sam", "passwd=eee3"} }, "eee3"},
		{"step text", func(e *Entry) { e.Steps = []Step{{WF: "w", Step: "s", Text: "authorization: rrr1"}} }, "rrr1"},
		{"step name", func(e *Entry) { e.Steps = []Step{{WF: "w", Step: "token=ttt2"}} }, "ttt2"},
		{"private key", func(e *Entry) { e.How = "-----BEGIN RSA PRIVATE KEY-----" }, "BEGIN"},
		{"token shape", func(e *Entry) { e.How = "use ghp_abcdefghijklmnopqrstuvwxyz0123" }, "ghp_abcdefghijklmnopqrstuvwxyz0123"},
		{"secret name token shape", func(e *Entry) { e.Secrets = []Secret{{Name: "AKIAIOSFODNN7EXAMPLE"}} }, "AKIAIOSFODNN7EXAMPLE"},
		{"id token shape", func(e *Entry) { e.ID = "sk-abcdefghijklmnopqrstuvwxyz" }, "sk-abcdefghijklmnopqrstuvwxyz"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := lnEnvStore(t)
			e := lnEnvStage()
			tc.mutate(&e)
			err := Save(s, e)
			if err == nil {
				t.Fatal("saved a secret value")
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Fatalf("error echoes the value: %v", err)
			}
			if _, rerr := Render(e); rerr == nil || strings.Contains(rerr.Error(), tc.value) {
				t.Fatalf("Render: %v", rerr)
			}
			lnEnvEmptyHome(t, s)
		})
	}
}

// A refused edit leaves a saved entry byte for byte as it was.
func TestLnEnvRefusalLeavesTheSavedEntry(t *testing.T) {
	s := lnEnvStore(t)
	if err := Save(s, lnEnvStage()); err != nil {
		t.Fatal(err)
	}
	e := lnEnvStage()
	e.Test = `curl -H "Authorization: Bearer abc123"`
	if err := Save(s, e); err == nil || strings.Contains(err.Error(), "abc123") {
		t.Fatalf("save: %v", err)
	}
	if got := lnEnvRead(t, s, "stage"); string(got) != lnEnvExample {
		t.Fatalf("entry changed: %q", got)
	}
	if n := len(lnEnvAuditLines(t, s)); n != 1 {
		t.Fatalf("audit lines: %d", n)
	}
}

func TestLnEnvRefusesBadFields(t *testing.T) {
	long := strings.Repeat("a", 101)
	for name, mutate := range map[string]func(*Entry){
		"id underscore":        func(e *Entry) { e.ID = "Bad_Id" },
		"id empty":             func(e *Entry) { e.ID = "" },
		"id leading hyphen":    func(e *Entry) { e.ID = "-x" },
		"id too long":          func(e *Entry) { e.ID = long },
		"id TOPIC":             func(e *Entry) { e.ID = "TOPIC" },
		"id slash":             func(e *Entry) { e.ID = "a/b" },
		"kind moon":            func(e *Entry) { e.Kind = "moon" },
		"name empty":           func(e *Entry) { e.Name = "" },
		"name blank":           func(e *Entry) { e.Name = "  \t" },
		"name newline":         func(e *Entry) { e.Name = "Stag\ning" },
		"name trailing lf":     func(e *Entry) { e.Name = "Staging\n" },
		"where cr":             func(e *Entry) { e.Where = "a\rb" },
		"runs nul":             func(e *Entry) { e.Runs = "a\x00b" },
		"who newline":          func(e *Entry) { e.Who = []string{"a\nb"} },
		"who empty":            func(e *Entry) { e.Who = []string{" "} },
		"secret trailing lf":   func(e *Entry) { e.Secrets = []Secret{{Name: "API_TOKEN\n"}} },
		"secret leading digit": func(e *Entry) { e.Secrets = []Secret{{Name: "1TOKEN"}} },
		"secret hyphen":        func(e *Entry) { e.Secrets = []Secret{{Name: "API-TOKEN"}} },
		"secret space":         func(e *Entry) { e.Secrets = []Secret{{Name: "API TOKEN"}} },
		"secret empty":         func(e *Entry) { e.Secrets = []Secret{{Name: ""}} },
		"step no workflow":     func(e *Entry) { e.Steps = []Step{{Step: "s"}} },
		"step no step":         func(e *Entry) { e.Steps = []Step{{WF: "w"}} },
		"step text newline":    func(e *Entry) { e.Steps = []Step{{WF: "w", Step: "s", Text: "a\nb"}} },
	} {
		t.Run(name, func(t *testing.T) {
			s := lnEnvStore(t)
			e := lnEnvStage()
			mutate(&e)
			if err := Save(s, e); err == nil {
				t.Fatal("saved a bad entry")
			}
			lnEnvEmptyHome(t, s)
		})
	}
	// The id limits hold on the edge: 100 bytes is fine.
	s := lnEnvStore(t)
	e := lnEnvStage()
	e.ID = strings.Repeat("a", 100)
	if err := Save(s, e); err != nil {
		t.Fatalf("100 byte id: %v", err)
	}
}

// Ids the YAML dump may quote (numbers, booleans, null) still read back.
func TestLnEnvIdsThatLookLikeYAMLScalars(t *testing.T) {
	s := lnEnvStore(t)
	ids := []string{"123", "true", "null", "no", "1e3", "0x1f", "007", "2026-11-11"}
	for _, id := range ids {
		e := lnEnvStage()
		e.ID = id
		if err := Save(s, e); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
	}
	got, err := List(s)
	if err != nil || len(got) != len(ids) {
		t.Fatalf("list: %d %v", len(got), err)
	}
	for _, id := range ids {
		b := lnEnvRead(t, s, id)
		if !strings.HasPrefix(string(b), "---\nname: "+id+"\n---\n") && !strings.HasPrefix(string(b), "---\nname: \""+id+"\"\n---\n") && !strings.HasPrefix(string(b), "---\nname: '"+id+"'\n---\n") {
			t.Fatalf("front matter for %s: %q", id, b)
		}
	}
}

func TestLnEnvParseRefusesWhatSaveRefuses(t *testing.T) {
	body := func(lines ...string) []byte {
		return []byte("---\nname: stage\n---\n## Environments\n### stage\n" + strings.Join(lines, "\n") + "\n")
	}
	for name, data := range map[string][]byte{
		"no front matter":     []byte("## Environments\n### stage\nname: Staging\n"),
		"front name differs":  []byte("---\nname: other\n---\n## Environments\n### stage\nname: Staging\n"),
		"no heading":          []byte("---\nname: stage\n---\nname: Staging\n"),
		"wrong ### id":        []byte("---\nname: stage\n---\n## Environments\n### other\nname: Staging\n"),
		"no name":             body("kind: remote"),
		"bad kind":            body("name: Staging", "kind: moon"),
		"secret value":        body("name: Staging", "test: password: hunter2"),
		"repeated key":        body("name: Staging", "name: Again"),
		"unknown key":         body("name: Staging", "colour: red"),
		"empty scalar":        body("name: Staging", "where:"),
		"item outside a list": body("name: Staging", "  - sam"),
		"item after scalar":   body("name: Staging", "who:", "  - sam", "kind: local", "  - ada"),
		"secret no state":     body("name: Staging", "secrets:", "  - API_TOKEN"),
		"secret value name":   body("name: Staging", "secrets:", "  - API_TOKEN=abc (set)"),
		"step not json":       body("name: Staging", "steps:", "  - build"),
		"step unknown key":    body("name: Staging", "steps:", `  - {"wf":"w","step":"s","x":"y"}`),
		"approval maybe":      body("name: Staging", "approval: maybe"),
	} {
		t.Run(name, func(t *testing.T) {
			got, err := Parse("stage", data)
			if err == nil {
				t.Fatalf("parsed: %+v", got)
			}
			for _, v := range []string{"hunter2", "abc"} {
				if strings.Contains(err.Error(), v) {
					t.Fatalf("error echoes %q: %v", v, err)
				}
			}
		})
	}
}

func TestLnEnvListNamesABadEntry(t *testing.T) {
	s := lnEnvStore(t)
	if err := Save(s, lnEnvStage()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lnEnvPath(s, "hand"), []byte("---\nname: hand\n---\n## Environments\n### hand\nkind: remote\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := List(s)
	if err == nil || !strings.Contains(err.Error(), "hand") {
		t.Fatalf("list: %v", err)
	}
}

func TestLnEnvLooksSecret(t *testing.T) {
	for in, want := range map[string]bool{
		"make smoke": false, "API_TOKEN (set)": false, "stage box": false, "https://host/path": false,
		"password: hunter2": true, "https://u:pw@host": true, `Authorization: Bearer x`: true,
		"-----BEGIN PRIVATE KEY-----": true, "ghp_abcdefghijklmnopqrstuvwxyz0123": true,
	} {
		if got := brain.LooksSecret(in); got != want {
			t.Errorf("LooksSecret(%q) = %v", in, got)
		}
	}
}
