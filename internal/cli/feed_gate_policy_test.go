package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func TestFeedGatePolicyExitCodes(t *testing.T) {
	for _, policy := range []string{"chmod", "directory", "malformed", "missing"} {
		t.Run(policy, func(t *testing.T) {
			if policy == "chmod" {
				if runtime.GOOS == "windows" {
					t.Skip("Windows uses the directory row for an unreadable workflow.md")
				}
				if os.Geteuid() == 0 {
					t.Skip("root can read chmod 000 files")
				}
			}
			if policy == "missing" && runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
				t.Skip("successful Feed writes require Linux or macOS")
			}
			h, task := feedHarness(t)
			path := filepath.Join(task, "workflow.md")
			want := 1
			switch policy {
			case "chmod":
				h.write(path, "---\n{}\n---\n")
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			case "directory":
				h.mkdir(path)
			case "malformed":
				h.write(path, "---\ngate_post_roles: [\n---\n")
				want = 3
			case "missing":
				want = 0
			}
			args := []string{"feed", "post", "--type", "question", "--body", `{"question":"Continue?","who":"owner","gate":{"id":"Q-1","kind":"person"}}`}
			if code := h.run(task, args...); code != want {
				t.Errorf("grid feed post exit = %d, want %d", code, want)
			}
			store := feed.Store{TaskDir: task, TaskID: feedTaskID}
			if want != 0 {
				if !strings.Contains(h.errb.String(), "workflow.md") {
					t.Error("missing workflow.md diagnostic")
				}
				if _, err := os.Stat(store.Dir()); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("policy failure created or changed Feed: %v", err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				h.write(path, "---\n{}\n---\n")
				if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
					t.Log("retry acceptance requires Linux or macOS; policy failure assertions completed")
					return
				}
				if code := h.run(task, args...); code != 0 {
					t.Fatalf("retry of the same gate ID exited %d", code)
				}
			}
			read, err := store.ReadOnly(feed.Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if len(read.Records) != 1 || read.Records[0].Record.Seq != 1 {
				t.Fatal("expected exactly one gate record at sequence 1")
			}
		})
	}
}
