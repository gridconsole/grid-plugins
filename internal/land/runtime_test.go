package land

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/task"
)

func TestDeployEnvironmentExcludesSecrets(t *testing.T) {
	s, _, repo := fixture(t)
	t.Setenv("LAND_SECRET", "must-not-reach-child")
	t.Setenv("GRID_LAND_BRANCH", "untrusted")
	path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	write(t, path, "---\nlands:\n  target: local\n  branch: main\n  command: env > outcome\n  needs-approve: true\n---\n")
	approve(t, s)
	if _, err := s.Run(context.Background(), repo, "T-01", false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(repo, "outcome"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "LAND_SECRET=") {
		t.Fatal("secret inherited by deploy child")
	}
	if !strings.Contains(string(b), "GRID_LAND_BRANCH=main\n") || strings.Contains(string(b), "untrusted") {
		t.Fatalf("land metadata: %s", b)
	}
}

func TestConcurrentHistoryAppend(t *testing.T) {
	s, dir, _ := fixture(t)
	write(t, filepath.Join(dir, "history.md"), "# History\n")
	const count = 24
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < count; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			if err := s.note("", "T-01", fmt.Sprintf("land concurrent %02d", i)); err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()
			<-start
			if err := task.AppendHistory(dir, fmt.Sprintf("hook concurrent %02d", i), "", "worker", ""); err != nil {
				t.Error(err)
			}
		}()
	}
	close(start)
	wg.Wait()
	b, err := os.ReadFile(filepath.Join(dir, "history.md"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < count; i++ {
		for _, kind := range []string{"land", "hook"} {
			marker := fmt.Sprintf("%s concurrent %02d", kind, i)
			if strings.Count(string(b), marker) != 1 {
				t.Errorf("lost or duplicated %s", marker)
			}
		}
	}
}

func TestDeployEnvironmentAllowlist(t *testing.T) {
	s, _, repo := fixture(t)
	t.Setenv("LAND_ALLOWED", "explicit-value")
	t.Setenv("LAND_SECRET", "secret-value")
	t.Setenv("GRID_LAND_BRANCH", "forged-branch")
	path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
	write(t, path, "---\nlands:\n  target: local\n  branch: main\n  command: env > outcome\n  needs-approve: true\n  env: [LAND_ALLOWED, GRID_LAND_BRANCH]\n---\n")
	approve(t, s)
	if _, err := s.Run(context.Background(), repo, "T-01", false); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(repo, "outcome"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "LAND_ALLOWED=explicit-value\n") || strings.Contains(string(b), "LAND_SECRET=") || strings.Contains(string(b), "forged-branch") {
		t.Fatalf("incorrect child environment: %s", b)
	}
}

func TestInvalidEnvironmentAllowlist(t *testing.T) {
	for _, value := range []string{"TOKEN", "[bad=name]", "[123]", "[null]", "null", "['']"} {
		t.Run(value, func(t *testing.T) {
			s, _, repo := fixture(t)
			path := filepath.Join(s.Resolver.GridHome, "projects", "sample", "knowledge", "deploy", "lands.md")
			b, _ := os.ReadFile(path)
			write(t, path, strings.Replace(string(b), "  target:", "  env: "+value+"\n  target:", 1))
			if _, err := s.Run(context.Background(), repo, "T-01", true); err == nil {
				t.Fatal("invalid environment accepted")
			}
		})
	}
}
