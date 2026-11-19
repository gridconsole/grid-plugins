package jobcmd

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/gridconfig"
)

// With the brain key set, recall reads the vault and still finds a note accepted
// under the Grid home.
func TestRecallReadsTheConfiguredBrain(t *testing.T) {
	e, _, _, _ := recallDeployFixture(t)
	e.Home = t.TempDir()
	vault := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(vault, 0o755); err != nil {
		t.Fatal(err)
	}
	note := "---\ndescription: How we deploy the export page by hand\nsummary: Deploy the export page by hand when Netlify is down.\n---\nSteps.\n"
	if err := os.WriteFile(filepath.Join(vault, "Manual-deploy.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := gridconfig.Config{Current: "w", Workspaces: []gridconfig.Workspace{{Name: "w", Data: e.GridHome, Brain: vault, Member: "Sam"}}}
	if err := gridconfig.Write(gridconfig.PathFor(runtime.GOOS, func(string) string { return "" }, e.Home), cfg); err != nil {
		t.Fatal(err)
	}
	task := filepath.Join(e.Cwd, "T-01")
	w, err := sources(task, e)
	if err != nil {
		t.Fatal(err)
	}
	j, err := loadJob(task, "1")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, l := range w.recall(j) {
		got[l.To] = true
	}
	for _, want := range []string{"brain:home/Manual-deploy", "brain:project/deploy"} {
		if !got[want] {
			t.Errorf("recall lacks %s: %v", want, got)
		}
	}
}
