package task

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/taskfile"
)

func TestRenamePolicy(t *testing.T) {
	for _, role := range []string{"", "owner", "member", "guest", "worker"} {
		want := role == "" || role == "owner" || role == "member"
		if RenameAllowed(role) != want {
			t.Fatalf("RenameAllowed(%q) != %t", role, want)
		}
	}
	original := RenameAllowed
	t.Cleanup(func() { RenameAllowed = original })
	RenameAllowed = func(role string) bool { return role == "owner" }
	s, task := writeFixture(t)
	before, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: "Changed", Role: "member"})
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("custom policy: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
	if err != nil || string(after) != string(before) {
		t.Fatalf("custom refusal changed task: %v", err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, ".task.lock")); !os.IsNotExist(err) {
		t.Fatalf("policy must run before lock: %v", err)
	}
	if _, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: "Changed", Role: "owner"}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentRenames(t *testing.T) {
	s, task := writeFixture(t)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, title := range []string{"First title", "Second title"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: title})
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	got, err := Read(task.Path)
	if err != nil || (got.Title != "First title" && got.Title != "Second title") || got.State != "queued" {
		t.Fatalf("invalid task: %+v, %v", got, err)
	}
	lines, err := s.History("", task.ID)
	if err != nil || len(lines) != 2 {
		t.Fatalf("history = %v, %v", lines, err)
	}
	for _, title := range []string{"First title", "Second title"} {
		if strings.Count(strings.Join(lines, "\n"), "renamed · "+title) != 1 {
			t.Fatalf("lost receipt for %s: %v", title, lines)
		}
	}
	if !strings.Contains(lines[1], "renamed · "+got.Title) {
		t.Fatal("final task disagrees with last receipt")
	}
}

func TestRenameValidation(t *testing.T) {
	for _, title := range []string{"", " \t ", "one\ntwo", "one\rtwo", "bad\x00title", strings.Repeat("界", 201)} {
		_, err := (Store{}).Rename(RenameOptions{Title: title})
		if !errors.Is(err, ErrEditUsage) {
			t.Fatalf("title %q: %v", title, err)
		}
	}
}

func TestRenameAddsUpdatedAndPreservesBytes(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n"} {
		t.Run(strings.ReplaceAll(ending, "\n", "LF"), func(t *testing.T) {
			s, task := writeFixture(t)
			path := filepath.Join(task.Path, "task.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			original := strings.ReplaceAll(string(data), "\n", ending)
			write(t, path, original)
			event, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: "Changed"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var kept []string
			for _, line := range strings.Split(string(got), ending) {
				if strings.HasPrefix(line, "title:") {
					kept = append(kept, "title: Example task")
				} else if !strings.HasPrefix(line, "updated:") {
					kept = append(kept, line)
				}
			}
			if strings.Join(kept, ending) != original {
				t.Fatalf("unrelated bytes changed: %q", got)
			}
			if event.When != contract.FormatTime(s.clock()) {
				t.Fatalf("receipt time = %s", event.When)
			}
		})
	}
}

func TestRenameScalarStyles(t *testing.T) {
	for _, title := range []string{
		"title: 'Example task' # old title\n",
		"title: >-\n  Example\n  task\n",
		"title: \"Example\n  task\"\n",
	} {
		t.Run(title, func(t *testing.T) {
			s, task := writeFixture(t)
			path := filepath.Join(task.Path, "task.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			suffix := "  # Preserve this indented comment\n\n# Preserve this next-key comment\n"
			original := strings.Replace(string(data), "title: Example task\n", title+suffix, 1) + "\ntitle: Body title stays\nupdated: Body time stays\n"
			write(t, path, original)
			event, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: "Changed"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := strings.Replace(original, title, "title: \"Changed\"\n", 1)
			want = strings.Replace(want, "---\n\ntitle: Body", "updated: \""+event.When+"\"\n---\n\ntitle: Body", 1)
			if string(got) != want {
				t.Fatalf("unrelated bytes changed:\n%s", got)
			}
		})
	}
}

// Check the original input, including controls that whitespace folding removes.
func TestRenameDeclineRejectControls(t *testing.T) {
	var forbidden []rune
	for r := rune(0); r <= 0x9f; r++ {
		if r <= 0x1f || r >= 0x7f {
			forbidden = append(forbidden, r)
		}
	}
	for r := rune(0x202a); r <= 0x202e; r++ {
		forbidden = append(forbidden, r)
	}
	for r := rune(0x2066); r <= 0x2069; r++ {
		forbidden = append(forbidden, r)
	}
	for _, r := range forbidden {
		t.Run(fmt.Sprintf("U+%04X", r), func(t *testing.T) {
			value := "before" + string(r) + "after"
			if _, err := (Store{}).Rename(RenameOptions{Title: value}); !errors.Is(err, ErrEditUsage) {
				t.Errorf("rename: %v", err)
			}
			if _, err := (Store{}).Decline(DeclineOptions{Reason: value}); !errors.Is(err, ErrEditUsage) {
				t.Errorf("decline: %v", err)
			}
		})
	}
}

func TestRenameTitleRoundTrip(t *testing.T) {
	for _, title := range []string{"A < B & C", "A > B", "issue #12", "issue # 12", `say "hi"`, `back\slash`, "修正 café 🚀", "[one, two]", "{key: value}"} {
		t.Run(title, func(t *testing.T) {
			s, task := writeFixture(t)
			if _, err := s.Rename(RenameOptions{WriteOptions: WriteOptions{ID: task.ID}, Title: title}); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(task.Path, "task.md"))
			if err != nil {
				t.Fatal(err)
			}
			for _, escape := range []string{`\u003c`, `\u003e`, `\u0026`} {
				if strings.Contains(string(data), escape) {
					t.Errorf("HTML escape %s in task.md", escape)
				}
			}
			for name, parse := range map[string]func(string) *taskfile.Frontmatter{"frontmatter": taskfile.ParseFrontmatter, "scalar": taskfile.ScalarFrontmatter} {
				got := parse(string(data))
				if got == nil || got.Data["title"] != title {
					t.Errorf("%s: got %+v, want %q", name, got, title)
				}
			}
			got, err := Read(task.Path)
			if err != nil || got.Title != title {
				t.Errorf("task reader: got %q, %v; want %q", got.Title, err, title)
			}
		})
	}
}
