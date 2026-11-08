package signal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/where"
)

func TestPersonalStoreRefused(t *testing.T) {
	s := Store{GridHome: t.TempDir(), Project: "personal"}
	s.Resolver.GridHome = s.GridHome
	s.Resolver.Caller = where.Caller{Role: "guest", Member: "gus"}
	if err := project.Write(s.GridHome, project.Record{Name: "personal", ID: "01PERSONAL"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(Input{Source: "test", Title: "x"}); err == nil || err.Error() != `project "personal" is not registered` {
		t.Fatalf("Add error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.GridHome, "projects", "personal", "notes")); !os.IsNotExist(err) {
		t.Fatalf("notes created: %v", err)
	}
}
