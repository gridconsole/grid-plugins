package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestTaskNoncharactersWriteNothing(t *testing.T) {
	var forbidden []rune
	for r := rune(0xfdd0); r <= 0xfdef; r++ {
		forbidden = append(forbidden, r)
	}
	for plane := rune(0); plane <= 16; plane++ {
		forbidden = append(forbidden, plane<<16|0xfffe, plane<<16|0xffff)
	}
	for _, r := range forbidden {
		for _, field := range []string{"rename", "decline", "words", "workflow", "owner", "actor", "reason", "history", "approval", "outcome", "by"} {
			t.Run(fmt.Sprintf("U+%04X/%s", r, field), func(t *testing.T) {
				h := newHarness(t)
				fixture := taskFixture(t, h)
				value := "before" + string(r) + "after"
				var args []string
				switch field {
				case "rename":
					args = []string{"task", "rename", fixture.ID, "--title", value}
				case "decline":
					args = []string{"task", "decline", fixture.ID, "--reason", value}
				case "words":
					args = []string{"task", "new", value}
				case "workflow":
					args = []string{"task", "new", "Valid words", "--workflow", value}
				case "owner", "actor":
					h.env.Getenv = func(key string) string {
						if key == "GRID_ACTOR" {
							return value
						}
						return ""
					}
					args = []string{"task", "new", "Valid words"}
					if field == "actor" {
						args = []string{"task", "decline", fixture.ID, "--reason", "Later"}
					}
				case "reason":
					args = []string{"task", "close", fixture.ID, "--reason", value}
				case "history":
					args = []string{"history", "add", value, "--task", fixture.ID}
				case "approval":
					args = []string{"approve", fixture.ID, "--note", value}
				case "outcome":
					args = []string{"deliver", "--task", fixture.ID, "--outcome", value}
				case "by":
					args = []string{"deliver", "--task", fixture.ID, "--outcome", "Done", "--by", value}
				}
				before := taskTestSnapshot(t, h.gridHome)
				code := h.run(h.home, append(args, "--json")...)
				if code != 2 {
					t.Errorf("exit = %d, want 2", code)
				}
				assertErrorOutput(t, h.out.String(), h.errb.String(), CodeInvalidArgument)
				if strings.Count(h.errb.String(), "\n") != 1 {
					t.Error("expected one diagnostic line")
				}
				if !reflect.DeepEqual(before, taskTestSnapshot(t, h.gridHome)) {
					t.Error("invalid text changed files")
				}
			})
		}
	}
}

func TestTaskListUnreadableEntry(t *testing.T) {
	h := newHarness(t)
	good := taskFixture(t, h)
	badFolder := "T-02-broken"
	badPath := filepath.Join(filepath.Dir(good.Path), badFolder)
	h.write(filepath.Join(badPath, "task.md"), "---\ntitle: [invalid\n---\n")
	h.write(filepath.Join(badPath, "intent.md"), "Keep these words")
	before := taskTestSnapshot(t, h.gridHome)
	for _, state := range []string{"", "queued", "closed"} {
		for _, asJSON := range []bool{false, true} {
			t.Run(fmt.Sprintf("state=%s/json=%t", state, asJSON), func(t *testing.T) {
				args := []string{"task", "list", "--project", "personal", "--state", state}
				if asJSON {
					args = append(args, "--json")
				}
				if code := h.run(h.home, args...); code != 0 {
					t.Fatalf("list exit = %d, want 0", code)
				}
				if strings.Count(h.errb.String(), "\n") != 1 || !strings.Contains(h.errb.String(), "warning") || !strings.Contains(h.errb.String(), badFolder) {
					t.Fatalf("expected one warning naming %s", badFolder)
				}
				wantGood := state != "closed"
				if !asJSON {
					if strings.Contains(h.out.String(), good.Title) != wantGood || strings.Contains(h.out.String(), "T-02") {
						t.Fatal("plain output must contain only matching readable tasks")
					}
					return
				}
				var entries []struct {
					ID, Path, Project string
					Unreadable        bool
				}
				if err := json.Unmarshal(h.out.Bytes(), &entries); err != nil {
					t.Fatal(err)
				}
				wantLen := 1
				if wantGood {
					wantLen++
				}
				if len(entries) != wantLen {
					t.Fatalf("entries = %d, want %d", len(entries), wantLen)
				}
				bad := entries[len(entries)-1]
				if bad.ID != "T-02" || bad.Path != badPath || bad.Project != good.Project || !bad.Unreadable {
					t.Fatal("missing unreadable marker or folder identity")
				}
				if wantGood && (entries[0].ID != good.ID || entries[0].Unreadable) {
					t.Fatal("good task lost or marked unreadable")
				}
			})
		}
	}
	if code := h.run(h.home, "task", "show", "T-02", "--json"); code != 1 || h.out.Len() != 0 {
		t.Fatal("show must still fail for the unreadable task")
	}
	if !reflect.DeepEqual(before, taskTestSnapshot(t, h.gridHome)) {
		t.Fatal("read commands changed files")
	}
}
