package feed

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestGatePolicyReadErrors(t *testing.T) {
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
			s, w := testStore(t), testWriter()
			path := filepath.Join(s.TaskDir, "workflow.md")
			switch policy {
			case "chmod":
				if err := os.WriteFile(path, []byte("---\n{}\n---\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			case "malformed":
				if err := os.WriteFile(path, []byte("---\ngate_post_roles: [\n---\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			in := gatedQuestion("Q-1")
			_, err := s.Post(in, w)
			if policy == "missing" {
				if err != nil {
					t.Fatalf("missing workflow did not retain defaults: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("policy failure admitted a gate")
				}
				var invalid *InvalidRecordError
				wantInvalid := policy == "malformed"
				if errors.As(err, &invalid) != wantInvalid || errors.Is(err, ErrInvalidRecord) != wantInvalid {
					t.Errorf("InvalidRecordError = %t, want %t", errors.As(err, &invalid), wantInvalid)
				}
				if !wantInvalid {
					var pathErr *fs.PathError
					if !errors.As(err, &pathErr) {
						t.Errorf("read failure lost PathError: %T", err)
					}
					if policy == "chmod" && !errors.Is(err, fs.ErrPermission) {
						t.Errorf("read failure lost permission cause: %T", err)
					}
				}
				if _, err := os.Stat(s.Dir()); !errors.Is(err, fs.ErrNotExist) {
					t.Fatalf("policy failure created or changed Feed: %v", err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte("---\n{}\n---\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
					t.Log("retry acceptance requires Linux or macOS; policy failure assertions completed")
					return
				}
				if _, err := s.Post(in, w); err != nil {
					t.Fatalf("policy failure reserved the gate ID: %v", err)
				}
			}
			read, err := s.ReadOnly(Filter{})
			if err != nil {
				t.Fatal(err)
			}
			if len(read.Records) != 1 || gateID(read.Records[0].Record) != "Q-1" || read.Records[0].Record.Seq != 1 {
				t.Fatal("expected exactly one gate record at sequence 1")
			}
		})
	}
}
