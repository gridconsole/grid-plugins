package machinefolder_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

// seed fills a folder the way Grid fills it: through the folder's own writers.
func seed(t *testing.T, f machinefolder.Folder) {
	t.Helper()
	l, err := f.Launcher("alice")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Start(4242); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Stop() })
	lg, err := l.OpenLog()
	if err != nil {
		t.Fatal(err)
	}
	lg.WriteString("2026-11-17 launcher started\n")
	lg.Close()
	if err := f.WriteState("T-01", []byte(`{"posts":[{"id":"P1","type":"result","text":"done"}]}`)); err != nil {
		t.Fatal(err)
	}
	bc, _ := f.BrainCache()
	if err := f.WriteFile(filepath.Join(bc, "index.json"), []byte(`{"notes":3}`)); err != nil {
		t.Fatal(err)
	}
	wt, _ := f.Worktrees()
	if err := f.WriteFile(filepath.Join(wt, "J-01", "README.md"), []byte("# a checkout\n")); err != nil {
		t.Fatal(err)
	}
	vd, _ := f.Videos()
	if err := f.WriteFile(filepath.Join(vd, "T-01", "run.mp4"), append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42 token=notatoken")...)); err != nil {
		t.Fatal(err)
	}
}

// Row 6: the secrets check over a seeded folder finds none.
func TestSecretsCheckFindsNoneInASeededFolder(t *testing.T) {
	f := tempFolder(t)
	seed(t, f)
	hits, err := machinefolder.ScanSecrets(f.Path, brain.LooksSecret)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 0 {
		t.Fatalf("the seeded folder holds secrets: %+v", hits)
	}
}

// Row 6: a planted token file is reported, and the report never carries the token.
func TestSecretsCheckReportsPlantedFiles(t *testing.T) {
	f := tempFolder(t)
	seed(t, f)
	token := "ghp_" + strings.Repeat("a1B2", 9)
	plant := map[string]string{
		"launcher/alice/notes.txt":      "my token is " + token + "\n",
		"brain-cache/.credentials.json": "{}",
		".claude/settings.json":         "{}",
		"launcher/bob/.claude-work/x":   "x",
		"worktrees/J-01/.env":           "A=1\n",
		"worktrees/J-01/deploy.pem":     "x",
		"tasks/T-01/api.txt":            "api_key = " + strings.Repeat("z", 12) + "\n",
	}
	for rel, text := range plant {
		p := filepath.Join(f.Path, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	hits, err := machinefolder.ScanSecrets(f.Path, brain.LooksSecret)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, h := range hits {
		got[h.Path] = h.Reason
		if strings.Contains(h.Path+h.Reason, token) {
			t.Fatalf("a hit carries the token: %+v", h)
		}
	}
	want := map[string]string{
		"launcher/alice/notes.txt":      "content",
		"brain-cache/.credentials.json": "file name",
		".claude":                       "folder name",
		"launcher/bob/.claude-work":     "folder name",
		"worktrees/J-01/.env":           "file name",
		"worktrees/J-01/deploy.pem":     "file name",
		"tasks/T-01/api.txt":            "content",
	}
	for p, reason := range want {
		if got[p] != reason {
			t.Errorf("%s: reported as %q, want %q", p, got[p], reason)
		}
	}
	if len(got) != len(want) {
		t.Errorf("hits = %v, want exactly %v", got, want)
	}
	for i := 1; i < len(hits); i++ {
		if hits[i-1].Path > hits[i].Path {
			t.Fatal("hits are not sorted")
		}
	}
}

func TestSecretsCheckDoesNotFollowLinks(t *testing.T) {
	f := tempFolder(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "t.txt"), []byte("token = abcdef123456\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(f.Path, "link")); err != nil {
		t.Skip("symlinks are not available here")
	}
	hits, err := machinefolder.ScanSecrets(f.Path, brain.LooksSecret)
	if err != nil || len(hits) != 0 {
		t.Fatalf("followed a link out of the folder: %+v, %v", hits, err)
	}
}
