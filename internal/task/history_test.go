package task

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestHistorySerializesAndCleans(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "history.md")
	if err := os.WriteFile(file, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AppendHistory(dir, "file written", "plan accepted", "claude\n· forged", ""); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(b), "\n") != 1 || !strings.Contains(string(b), `"plan accepted"`) || strings.Contains(string(b), "by claude\n") {
		t.Fatalf("unsafe or duplicate history: %q", b)
	}
}

func TestFolderAtPersonalPolicy(t *testing.T) {
	home := gridhome.Real(t.TempDir())
	for _, project := range []string{"personal", "alpha"} {
		dir := filepath.Join(gridhome.TasksDir(home, project), "T-01-example")
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte("legacy task"), 0600); err != nil {
			t.Fatal(err)
		}
		if project == "alpha" {
			if err := os.WriteFile(filepath.Join(home, "projects", project, "project-roles.json"), []byte(`{"people":[{"id":"gus"}]}`), 0600); err != nil {
				t.Fatal(err)
			}
		}
		for _, tc := range []struct {
			name    string
			caller  where.Caller
			policy  func(where.Caller) bool
			allowed bool
		}{
			{name: "owner", caller: where.Caller{Role: "owner"}, allowed: true},
			{name: "guest", caller: where.Caller{Role: "guest", Member: "gus"}},
			{name: "member", caller: where.Caller{Role: "member", Member: "sam"}},
			{name: "override", caller: where.Caller{Role: "owner"}, policy: func(where.Caller) bool { return false }},
		} {
			t.Run(project+"/"+tc.name, func(t *testing.T) {
				r := where.Resolver{GridHome: home, Caller: tc.caller, PersonalAllowed: tc.policy}
				got, err := FolderAt(r, filepath.Join(dir, "plan.md"))
				want := dir
				if project == "personal" && !tc.allowed {
					want = ""
				}
				if err != nil || got != want {
					t.Fatalf("FolderAt() = %q, %v; want %q", got, err, want)
				}
			})
		}
	}
}
