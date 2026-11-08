package task

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
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
