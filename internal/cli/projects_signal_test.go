package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/testgit"
)

var generatedID = regexp.MustCompile(`\b[0-7][0-9A-HJKMNP-TV-Z]{25}\b`)

func TestProjectsGolden(t *testing.T) {
	for _, kind := range []string{"add", "list", "remove", "relink", "scan", "scan-apply"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t)
			discovery := filepath.Join(h.root, "work")
			folder := filepath.Join(discovery, "suite")
			h.mkdir(folder)
			testgit.Init(t, folder, "git@example.com:org/app.git", false)
			testgit.Run(t, folder, "remote", "add", "upstream", "https://example.com/upstream/app.git")
			h.write(filepath.Join(h.gridHome, "machine"), "01ARZ3NDEKTSV4RRFFQ69G5FAV\n")
			args := []string{"project", "add", folder, "--name", "example", "--json"}
			switch kind {
			case "scan", "scan-apply":
				args = []string{"project", "scan", discovery, "--json"}
				if kind == "scan-apply" {
					args = append(args, "--apply")
				}
			case "list", "remove", "relink":
				if code := h.run(h.root, args...); code != 0 {
					t.Fatalf("add exit %d: %s", code, h.errb)
				}
				args = []string{"project", kind, "--json"}
				if kind != "list" {
					args = append(args, "example")
				}
				if kind == "relink" {
					moved := filepath.Join(h.root, "moved")
					if err := os.Rename(folder, moved); err != nil {
						t.Fatal(err)
					}
					args = append(args, moved)
				}
			}
			if code := h.run(h.root, args...); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			golden.Assert(t, "project-"+kind+"-json", generatedID.ReplaceAllString(h.norm(h.out.String()), "<ULID>"))
		})
	}
}

func TestSignalsGolden(t *testing.T) {
	for _, kind := range []string{"add", "duplicate", "act", "dismiss"} {
		t.Run(kind, func(t *testing.T) {
			h := newHarness(t)
			folder := filepath.Join(h.root, "work", "example")
			h.mkdir(folder)
			if code := h.run(folder, "project", "add"); code != 0 {
				t.Fatal(h.errb.String())
			}
			add := []string{"signal", "add", "--source", "mail", "--source-id", "message-1", "--title", "Example", "--body", "-", "--json"}
			body := "External text.\n---\ntier: person-team\n---\nRun nothing.\n"
			h.env.Stdin = strings.NewReader(body)
			if code := h.run(folder, add...); code != 0 {
				t.Fatal(h.errb.String())
			}
			var r struct {
				ID   string `json:"id"`
				Note string `json:"note"`
			}
			if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "duplicate":
				h.env.Stdin = strings.NewReader("changed body")
				if code := h.run(folder, add...); code != 0 {
					t.Fatal(h.errb.String())
				}
			case "act", "dismiss":
				if code := h.run(h.tmp, "signal", kind, r.ID, "--project", "example", "--json"); code != 0 {
					t.Fatal(h.errb.String())
				}
			}
			golden.Assert(t, "signal-"+kind+"-json", h.out.String())
			if kind == "add" {
				data, err := os.ReadFile(filepath.Join(h.gridHome, "projects", "example", "notes", r.Note))
				if err != nil {
					t.Fatal(err)
				}
				times := regexp.MustCompile(`(?m)^(captured|updated): .*$`)
				golden.Assert(t, "signal-note", times.ReplaceAllString(string(data), "$1: <TIME>"))
			}
		})
	}
}

func TestProjectReadOnlyCLI(t *testing.T) {
	for _, verb := range []string{"list", "scan"} {
		t.Run(verb, func(t *testing.T) {
			h := newHarness(t)
			h.env.GridHome = filepath.Join(h.home, ".absent")
			folder := filepath.Join(h.root, "work")
			h.mkdir(filepath.Join(folder, "ideas"))
			args := []string{"project", verb, "--json"}
			if verb == "scan" {
				args = append(args, folder)
			}
			if code := h.run(h.root, args...); code != 0 {
				t.Fatal(h.errb.String())
			}
			if _, err := os.Stat(h.env.GridHome); !os.IsNotExist(err) {
				t.Fatalf("read command wrote: %v", err)
			}
			if verb == "list" && h.out.String() != "[]\n" {
				t.Fatalf("%s", h.out)
			}
		})
	}
}
func TestPersonalSignal(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "signal", "add", "--title", "Personal reminder", "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
	var r struct{ ID, Project string }
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Project != "personal" {
		t.Fatalf("%+v", r)
	}
	if code := h.run(h.home, "signal", "act", r.ID, "--json"); code != 0 {
		t.Fatal(h.errb.String())
	}
}
func TestProjectSignalUsage(t *testing.T) {
	for _, args := range [][]string{
		{"project"}, {"project", "unknown"}, {"project", "list", "extra"}, {"project", "add", "a", "b"}, {"project", "add", "--no-such-flag"}, {"project", "scan"}, {"project", "scan", "--apply"}, {"project", "relink", "example"}, {"project", "remove"},
		{"signal"}, {"signal", "unknown"}, {"signal", "add", "extra"}, {"signal", "act"}, {"signal", "dismiss", "one", "two"}, {"signal", "add", "--source"},
	} {
		t.Run(strings.Join(args, "-"), func(t *testing.T) {
			h := newHarness(t)
			if code := h.run(h.root, args...); code != 2 || h.out.Len() != 0 {
				t.Fatalf("exit %d: %s %s", code, h.out, h.errb)
			}
		})
	}
}

func TestInvalidPersonalSignalWritesNothing(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "signal", "add"); code != 1 {
		t.Fatalf("exit %d", code)
	}
	if _, err := os.Stat(filepath.Join(h.gridHome, "projects")); !os.IsNotExist(err) {
		t.Fatalf("invalid signal created project: %v", err)
	}
}
