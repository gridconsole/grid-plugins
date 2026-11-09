package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/golden"
	"github.com/gridconsole/grid-plugins/internal/project"
)

func moveTree(t *testing.T, root string) [32]byte {
	t.Helper()
	h := sha256.New()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		fmt.Fprintln(h, rel, d.Type())
		if !d.IsDir() {
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			h.Write(b)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var sum [32]byte
	copy(sum[:], h.Sum(nil))
	return sum
}

func TestSignalMoveCLI(t *testing.T) {
	for _, status := range []string{"new", "placed", "snoozed"} {
		t.Run(status, func(t *testing.T) {
			h, source, before := actFixture(t, status)
			h.record(project.Record{Name: "second"})
			h.record(project.Record{Name: "personal"})
			h.env.Getenv = func(string) string { return "" }
			for _, target := range []string{"second", "-"} {
				from := "example"
				if target == "-" {
					from = "second"
				}
				if code := h.run(h.home, "signal", "move", actID, "--project", from, "--to", target, "--json"); code != 0 {
					t.Fatalf("move exit %d: %s", code, h.errb)
				}
				var got struct{ From, Project, Status, Became string }
				if err := json.Unmarshal(h.out.Bytes(), &got); err != nil {
					t.Fatal(err)
				}
				projectName, want := target, status
				if target == "-" {
					projectName = "personal"
					if status != "snoozed" {
						want = "new"
					}
				} else if status == "new" {
					want = "placed"
				}
				if got.From != from || got.Project != projectName || got.Status != want || got.Became != want {
					t.Fatalf("result %+v", got)
				}
				if _, err := os.Stat(source); !os.IsNotExist(err) {
					t.Fatalf("source remains: %v", err)
				}
				source = filepath.Join(h.gridHome, "projects", projectName, "notes", actID+".md")
				after, _ := actRead(t, source)
				expected := bytes.Replace(before, []byte("\nstatus: "+status+"\n"), []byte("\nstatus: "+want+"\n"), 1)
				if !bytes.Equal(expected, after) {
					t.Fatal("move changed content beyond status")
				}
			}
		})
	}
}

func TestSignalMoveRefusalsCLI(t *testing.T) {
	for _, name := range []string{"missing-note", "unknown-source", "unknown-target", "done", "dismissed", "kept", "same", "clash", "cap"} {
		t.Run(name, func(t *testing.T) {
			status := "new"
			if name == "done" || name == "dismissed" || name == "kept" {
				status = name
			}
			h, _, original := actFixture(t, status)
			h.env.Getenv = func(string) string { return "" }
			h.record(project.Record{Name: "second"})
			targetDir := filepath.Join(h.gridHome, "projects", "second", "notes")
			h.mkdir(targetDir)
			from, to, id, message := "example", "second", actID, ""
			switch name {
			case "missing-note":
				id = "S-01ARZ3NDEKTSV4RRFFQ69G5FAW"
				message = "no such note"
			case "unknown-source":
				from = "absent"
				message = "no such project"
			case "unknown-target":
				to = "absent"
				message = "no such project"
			case "done", "dismissed", "kept":
				message = "this note is already decided (" + name + ")"
			case "same":
				to = from
				message = "the note is already there"
			case "clash":
				h.write(filepath.Join(targetDir, actID+".md"), string(original))
				message = "a note with this id is already there"
			case "cap":
				for i := 0; i < 4000; i++ {
					h.write(filepath.Join(targetDir, fmt.Sprintf("note-%d.md", i)), "")
				}
				message = "notes already holds 4000 files"
			}
			root := filepath.Join(h.gridHome, "projects")
			before := moveTree(t, root)
			if code := h.run(h.home, "signal", "move", id, "--project", from, "--to", to, "--json"); code != 3 || h.errb.String() != "grid: "+message+"\n" {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			assertSignalErrorJSON(t, h.out.String(), "refused", message)
			if before != moveTree(t, root) {
				t.Fatal("refusal changed project trees")
			}
		})
	}
}

func TestSignalMoveGuestAndUnknownRoles(t *testing.T) {
	for _, role := range []string{"guest", "external", "unknown"} {
		for _, invited := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/invited=%t", role, invited), func(t *testing.T) {
				h, _, _ := actFixture(t, "new")
				h.record(project.Record{Name: "second"})
				if invited {
					h.write(filepath.Join(h.gridHome, "projects", "example", "project-roles.json"), `{"people":[{"id":"visitor"}]}`)
				}
				h.env.Getenv = func(k string) string {
					if k == "GRID_ROLE" {
						return role
					}
					if k == "GRID_MEMBER" {
						return "visitor"
					}
					return ""
				}
				root := filepath.Join(h.gridHome, "projects")
				before := moveTree(t, root)
				if code := h.run(h.home, "signal", "move", actID, "--project", "example", "--to", "second", "--json"); code != 3 || h.errb.String() != "grid: guests cannot act on signals\n" {
					t.Fatalf("exit %d: %s", code, h.errb)
				}
				assertSignalErrorJSON(t, h.out.String(), "refused", "guests cannot act on signals")
				if before != moveTree(t, root) {
					t.Fatal("guest changed trees")
				}
			})
		}
	}
}

func TestSignalMoveSignedOutNoPersonAndDefaultProject(t *testing.T) {
	h, _, _ := actFixture(t, "new")
	h.record(project.Record{Name: "personal"})
	h.env.Getenv = func(string) string { return "" }
	// The existing local caller policy permits both verbs without a person record.
	if code := h.run(h.home, "signal", "move", actID, "--project", "example", "--to", "-", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if code := h.run(h.home, "signal", "move", actID, "--to", "example", "--json"); code != 0 {
		t.Fatalf("default project exit %d: %s", code, h.errb)
	}
	if code := h.run(h.home, "signal", "act", actID, "--project", "example", "--json"); code != 0 {
		t.Fatalf("act exit %d: %s", code, h.errb)
	}
}

func TestSignalMoveBadArguments(t *testing.T) {
	for _, args := range [][]string{{actID}, {actID, "--to", ""}, {"../x", "--to", "second"}, {"a/b", "--to", "second"}, {`a\b`, "--to", "second"}, {"", "--to", "second"}, {"\x00", "--to", "second"}, {actID, "--to", "../x"}, {actID, "--to", "a/b"}, {actID, "--to", `a\b`}, {actID, "--to", "\x00"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			h, _, _ := actFixture(t, "new")
			before := moveTree(t, h.gridHome)
			if code := h.run(h.home, append([]string{"signal", "move", "--json"}, args...)...); code != 2 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
			if !strings.Contains(h.out.String(), `"error":"invalid"`) {
				t.Fatalf("stdout %s", h.out)
			}
			if before != moveTree(t, h.gridHome) {
				t.Fatal("invalid input touched storage")
			}
		})
	}
}

func TestSignalMoveHelp(t *testing.T) {
	h := newHarness(t)
	if code := h.run(h.home, "signal"); code != 2 {
		t.Fatal(code)
	}
	golden.Assert(t, "signal-help", h.errb.String())
	if code := h.run(h.home, "signal", "move", "--help"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	golden.Assert(t, "signal-move-help", h.errb.String())
}
