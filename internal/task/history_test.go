package task

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHistoryLines(t *testing.T) {
	for _, tc := range []struct {
		line string
		ok   bool
	}{{"built item 1", true}, {"", false}, {"two\nlines", false}, {"carriage\rreturn", false}, {"null\x00", false}} {
		t.Run(fmt.Sprintf("%q", tc.line), func(t *testing.T) {
			dir := t.TempDir()
			err := AppendHistory(dir, tc.line, time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
			if (err == nil) != tc.ok {
				t.Fatal(err)
			}
			if tc.ok {
				b, e := os.ReadFile(filepath.Join(dir, "history.md"))
				if e != nil || string(b) != "- 2026-01-02T03:04:05Z · "+tc.line+"\n" {
					t.Fatalf("%s %v", b, e)
				}
			}
		})
	}
}

func TestHistoryProcess(t *testing.T) {
	if os.Getenv("GRID_HISTORY_HELPER") != "1" {
		return
	}
	for i := 0; i < 40; i++ {
		if err := AppendHistory(os.Getenv("GRID_HISTORY_DIR"), fmt.Sprintf("%s-%d", os.Getenv("GRID_HISTORY_WRITER"), i), time.Now()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConcurrentHistoryProcesses(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "history.md"), []byte("Earlier line"), 0600); err != nil {
		t.Fatal(err)
	}
	cmds := make([]*exec.Cmd, 2)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestHistoryProcess$")
		cmds[i].Env = append(os.Environ(), "GRID_HISTORY_HELPER=1", "GRID_HISTORY_DIR="+dir, fmt.Sprintf("GRID_HISTORY_WRITER=writer%d", i))
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")) != 81 {
		t.Fatalf("lost lines: %s", b)
	}
	for i := 0; i < 2; i++ {
		for j := 0; j < 40; j++ {
			if strings.Count(string(b), fmt.Sprintf(" · writer%d-%d\n", i, j)) != 1 {
				t.Fatalf("missing or repeated %d-%d", i, j)
			}
		}
	}
}
