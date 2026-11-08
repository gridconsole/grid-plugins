package proof

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) Store {
	t.Helper()
	s := Store{TaskDir: t.TempDir()}
	if err := os.WriteFile(filepath.Join(s.TaskDir, "plan.md"), []byte("# Plan 1\n- [ ] 2 Capture screen\n  proof: screenshot\n- [x] 1 Check result\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddRefusals(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      Input
		message string
	}{
		{"unknown", Input{Item: "3", Text: "ok", By: "grid"}, "unknown plan item"},
		{"escape", Input{Item: "../1", Text: "ok", By: "grid"}, "unknown plan item"},
		{"kind", Input{Item: "1", Text: "ok", Kind: "bad", By: "grid"}, "kind must"},
		{"both", Input{Item: "1", Text: "ok", File: "file", By: "grid"}, "one file"},
		{"none", Input{Item: "1", By: "grid"}, "one file"},
		{"empty-author", Input{Item: "1", Text: "ok"}, "by and note"},
		{"text-video", Input{Item: "1", Text: "ok", Kind: "video", By: "grid"}, "need a file"},
		{"multiline-note", Input{Item: "1", Text: "ok", By: "grid", Note: "one\ntwo"}, "single-line"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			_, err := s.Add(tc.in)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatal(err)
			}
		})
	}
}

func TestIndexStableAndGate(t *testing.T) {
	s := fixture(t)
	for _, item := range []string{"1", "item-1", "i-1"} {
		if _, err := s.Add(Input{Item: item, Text: "checks pass", By: "grid", Kind: "log"}); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := s.Rebuild()
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.Check(); err == nil || !strings.Contains(err.Error(), "i-2 lacks screenshot") {
		t.Fatal(err)
	}
	shot := filepath.Join(t.TempDir(), "image.png")
	if err = os.WriteFile(shot, []byte("image bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := s.Add(Input{Item: "2", File: shot, Kind: "screenshot", By: "grid", Note: "Screen result"})
	if err != nil {
		t.Fatal(err)
	}
	idx, err = s.Rebuild()
	if err != nil {
		t.Fatal(err)
	}
	if err = idx.Check(); err != nil {
		t.Fatal(err)
	}
	if idx.Items[0].ID != "i-2" || len(idx.Items[1].Evidence) != 3 {
		t.Fatal(idx)
	}
	path := filepath.Join(s.TaskDir, "proof", "index.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Rebuild(); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("unstable index: %v", err)
	}
	if err = os.WriteFile(filepath.Join(s.TaskDir, filepath.FromSlash(r.File)), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Build(); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatal(err)
	}
}

func TestPlanChangeAndUnknownLevel(t *testing.T) {
	for _, level := range []string{"check", "magic"} {
		t.Run(level, func(t *testing.T) {
			s := fixture(t)
			if _, err := s.Add(Input{Item: "1", Text: "pass", By: "grid"}); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(s.TaskDir, "plan.md"), []byte("# Plan 2\n- [x] 1 Changed requirement\n  proof: "+level+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			idx, err := s.Build()
			if level == "magic" {
				if err == nil {
					t.Fatal("unknown level accepted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = idx.Check(); err == nil {
				t.Fatal("old proof counted")
			}
		})
	}
}

func TestSymlinkProof(t *testing.T) {
	s := fixture(t)
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(s.TaskDir, "proof")); err != nil {
		t.Skip(err)
	}
	if _, err := s.Add(Input{Item: "1", Text: "pass", By: "grid"}); err == nil {
		t.Fatal("followed symlink")
	}
	if _, err := s.Build(); err == nil {
		t.Fatal("read symlink")
	}
	files, err := os.ReadDir(outside)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}
