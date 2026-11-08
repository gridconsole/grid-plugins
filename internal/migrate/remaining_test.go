package migrate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These entries also run through every publication crash in crashLayouts.
func remainingFixture(t *testing.T, from string) {
	t.Helper()
	for _, f := range []string{".DS_Store", "config.json", "bin/tool", "alpha/custom/data"} {
		put(t, filepath.Join(from, f), "unchanged\r\n")
	}
	for _, h := range []string{"alpha", "beta"} {
		put(t, filepath.Join(from, h, "knowledge/shared.md"), "identical\r\n")
		put(t, filepath.Join(from, h, "knowledge", h+".md"), h)
	}
	put(t, filepath.Join(from, "alpha/sessions/unchanged.json"), "{\r\n  \"z\": 1, \"cwd\": \"/example/unrelated\", \"a\": 2\r\n}\r\n")
	f := "alpha/projects/widget/tasks/T-07-first/task.md"
	put(t, filepath.Join(from, f), strings.ReplaceAll(read(t, from, f), "\n", "\r\n"))
}

func TestCarryOverAndMerge(t *testing.T) {
	for _, inPlace := range []bool{false, true} {
		from, home := fixture(t)
		remainingFixture(t, from)
		if inPlace {
			home = from
		}
		p := scan(t, from, home)
		if len(p.Unmapped)+len(p.Conflicts) != 0 {
			t.Fatalf("blocked: %v %v", p.Unmapped, p.Conflicts)
		}
		if !strings.Contains(strings.Join(p.Notes, "\n"), "config.json: unknown") {
			t.Fatal("unknown entry not listed")
		}
		original := read(t, from, "alpha/sessions/unchanged.json")
		if _, err := p.Apply(); err != nil {
			t.Fatal(err)
		}
		if read(t, home, "sessions/unchanged.json") != original {
			t.Fatal("unchanged JSON was re-encoded")
		}
		for _, f := range []string{".DS_Store", "config.json", "bin/tool", "alpha/custom/data"} {
			if read(t, home, f) != "unchanged\r\n" {
				t.Fatal(f)
			}
		}
		for _, h := range []string{"alpha", "beta"} {
			if read(t, home, "knowledge/"+h+".md") != h {
				t.Fatal("knowledge not merged")
			}
		}
		task := read(t, home, "projects/widget/tasks/T-07-first/task.md")
		if strings.Contains(strings.ReplaceAll(task, "\r\n", ""), "\n") {
			t.Fatal("task CRLF lost")
		}
	}
}

func TestCarryOverRefusals(t *testing.T) {
	for _, kind := range []string{"unknown overwrite", "knowledge", "machine directory", "ancestor", "generated log", "generated parent"} {
		t.Run(kind, func(t *testing.T) {
			from, home := fixture(t)
			switch kind {
			case "unknown overwrite":
				put(t, filepath.Join(from, "config.json"), "old")
				put(t, filepath.Join(home, "config.json"), "different")
			case "knowledge":
				put(t, filepath.Join(from, "alpha/knowledge/shared.md"), "alpha")
				put(t, filepath.Join(from, "beta/knowledge/shared.md"), "beta")
			case "machine directory":
				put(t, filepath.Join(from, "machine/config.json"), "legacy")
			case "generated log":
				put(t, filepath.Join(from, "migration.log"), "unknown log")
			case "generated parent":
				put(t, filepath.Join(from, "projects"), "unknown file")
			case "ancestor":
				put(t, filepath.Join(home, "knowledge"), "file")
				put(t, filepath.Join(from, "alpha/knowledge/shared.md"), "alpha")
			}
			before := view(t, from)
			p := scan(t, from, home)
			if len(p.Conflicts) == 0 {
				t.Fatal("missing conflict list")
			}
			if kind == "machine directory" && !strings.Contains(strings.Join(p.Conflicts, "\n"), "mv machine machine-legacy") {
				t.Fatal(p.Conflicts)
			}
			if _, err := p.Apply(); err == nil {
				t.Fatal("overwrote conflict")
			}
			if !sameView(before, view(t, from)) {
				t.Fatal("source changed")
			}
			if _, err := os.Stat(home + "-backup"); !os.IsNotExist(err) {
				t.Fatal("blocked plan wrote backup")
			}
		})
	}
}

func TestUnchangedSessionThroughUnrelatedSymlink(t *testing.T) {
	from, home := fixture(t)
	alias := filepath.Join(filepath.Dir(from), "alias")
	if err := os.Symlink(filepath.Dir(from), alias); err != nil {
		t.Skip(err)
	}
	raw := "{\r\n  \"z\": 1, \"cwd\": \"" + filepath.Join(alias, "unrelated") + "\", \"a\": 2\r\n}\r\n"
	put(t, filepath.Join(from, "alpha/sessions/alias.json"), raw)
	if _, err := scan(t, from, home).Apply(); err != nil {
		t.Fatal(err)
	}
	if read(t, home, "sessions/alias.json") != raw {
		t.Fatal("unrelated symlink resolution re-encoded the session")
	}
}
