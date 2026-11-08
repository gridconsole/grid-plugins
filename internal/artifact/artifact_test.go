package artifact

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImport(t *testing.T) {
	for _, content := range []string{"", "a report\n", "\x00\xff\n"} {
		t.Run(fmt.Sprintf("bytes-%d", len(content)), func(t *testing.T) {
			home := t.TempDir()
			task := filepath.Join(home, "tasks", "T-01-example")
			if err := os.MkdirAll(task, 0o755); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(home, "report.txt")
			if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			s := Store{TaskPath: task, Now: func() time.Time { return time.Date(2026, 11, 8, 0, 0, 0, 0, time.UTC) }}
			first, err := s.Import(source, "")
			if err != nil {
				t.Fatal(err)
			}
			second, err := s.Import(source, "Another report")
			if err != nil {
				t.Fatal(err)
			}
			if first.ID == second.ID || first.File == second.File {
				t.Fatal("imports collided")
			}
			if first.Title != "report.txt" || first.Source != source || first.Kind != "file" || first.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(content))) || first.Created != "2026-11-08T00:00:00Z" {
				t.Fatalf("record: %+v", first)
			}
			for _, path := range []string{source, filepath.Join(task, "artifacts", first.File), filepath.Join(task, "artifacts", second.File)} {
				got, err := os.ReadFile(path)
				if err != nil || string(got) != content {
					t.Fatalf("copy changed bytes: %q, %v", got, err)
				}
			}
			published, err := s.Publish(first.ID)
			if err != nil || !published.Published {
				t.Fatalf("publish: %+v, %v", published, err)
			}
			again, err := s.Publish(first.ID)
			if err != nil || again != published {
				t.Fatalf("repeat publish changed record: %+v, %v", again, err)
			}
			list, err := s.List()
			if err != nil || len(list) != 2 {
				t.Fatalf("list: %+v, %v", list, err)
			}
		})
	}
}

func TestLink(t *testing.T) {
	for _, tc := range []struct {
		url   string
		valid bool
	}{
		{"https://example.com/report?a=1#summary", true},
		{"http://example.com", false}, {"file:///report", false}, {"javascript:alert(1)", false},
		{"//example.com", false}, {"https://", false}, {"https://user:secret@example.com", false},
		{"https://example.com/with space", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			s := Store{TaskPath: t.TempDir()}
			r, err := s.Link(tc.url, "Report")
			if (err == nil) != tc.valid {
				t.Fatalf("Link: %v", err)
			}
			if tc.valid && (r.Kind != "link" || r.Source != tc.url || r.File != "" || r.SHA256 != "") {
				t.Fatalf("record: %+v", r)
			}
			if !tc.valid {
				if _, err := os.Stat(filepath.Join(s.TaskPath, "artifacts")); !os.IsNotExist(err) {
					t.Fatal("invalid link wrote artifacts")
				}
			}
		})
	}
}

func TestStoreErrors(t *testing.T) {
	for _, op := range []string{"directory-source", "missing-source", "traversal-id", "missing-id", "symlink-directory", "symlink-record", "bad-record"} {
		t.Run(op, func(t *testing.T) {
			root := t.TempDir()
			s := Store{TaskPath: root}
			var err error
			switch op {
			case "directory-source":
				_, err = s.Import(root, "")
			case "missing-source":
				_, err = s.Import(filepath.Join(root, "missing"), "")
			case "traversal-id":
				_, err = s.Publish("../task.md")
			case "missing-id":
				_, err = s.Publish("A-" + strings.Repeat("0", 32))
			case "symlink-directory":
				if err := os.Symlink(t.TempDir(), filepath.Join(root, "artifacts")); err != nil {
					t.Fatal(err)
				}
				_, err = s.Link("https://example.com", "")
			default:
				r, e := s.Link("https://example.com", "")
				if e != nil {
					t.Fatal(e)
				}
				path := filepath.Join(root, "artifacts", r.ID+".json")
				if op == "symlink-record" {
					if e := os.Remove(path); e != nil {
						t.Fatal(e)
					}
					if e := os.Symlink(filepath.Join(root, "task.md"), path); e != nil {
						t.Fatal(e)
					}
				} else if e := os.WriteFile(path, []byte("{}"), 0o644); e != nil {
					t.Fatal(e)
				}
				_, err = s.List()
			}
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
	s := Store{TaskPath: t.TempDir()}
	records, err := s.List()
	if err != nil || records == nil || len(records) != 0 {
		t.Fatalf("empty list: %v, %v", records, err)
	}
}

func TestListRequiresExistingTask(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "missing")} {
		if _, err := (Store{TaskPath: path}).List(); err == nil {
			t.Fatal("list accepted a missing task")
		}
	}
}

func TestImportSizeLimit(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "report.txt")
	if err := os.WriteFile(source, []byte("0123456789"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Store{TaskPath: root, MaxBytes: 10}).Import(source, ""); err != nil {
		t.Fatalf("import at the limit: %v", err)
	}
	_, err := (Store{TaskPath: root, MaxBytes: 9}).Import(source, "")
	if err == nil || !strings.Contains(err.Error(), "import limit of 9 bytes") {
		t.Fatalf("import above the limit: %v", err)
	}
	entries, _ := os.ReadDir(filepath.Join(root, "artifacts"))
	if len(entries) != 2 {
		t.Fatalf("a refused import left files: %v", entries)
	}
}

func TestCopyLimitedStopsAtTheLimit(t *testing.T) {
	var out strings.Builder
	if err := copyLimited(&out, strings.NewReader("0123456789"), 10); err != nil || out.Len() != 10 {
		t.Fatalf("at the limit: %v, %d bytes", err, out.Len())
	}
	out.Reset()
	// A source that never ends must stop one byte past the limit.
	err := copyLimited(&out, endless{}, 10)
	if err == nil || out.Len() != 11 {
		t.Fatalf("above the limit: %v, %d bytes", err, out.Len())
	}
}

type endless struct{}

func (endless) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

func TestDefaultImportLimit(t *testing.T) {
	if DefaultMaxImportBytes != 200*1024*1024 {
		t.Fatalf("default limit %d", DefaultMaxImportBytes)
	}
}
