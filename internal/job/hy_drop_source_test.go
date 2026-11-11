package job_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/job"
	"github.com/gridconsole/grid-plugins/internal/job/jobtest"
)

// A dropped note is refused as a Job source, and is a source again once restored.
func TestHyDrJobSource(t *testing.T) {
	h := jobtest.New(t, filepath.Join("..", "..", "testdata"))
	e := h.Note("project", "deploy", "conventions", "How we deploy", "# Deploy\nRun the release.\n")
	data, err := os.ReadFile(filepath.Join(h.GridHome, filepath.FromSlash(e.Path)))
	if err != nil {
		t.Fatal(err)
	}
	rev := brain.NoteRev(data)
	ana := brain.Origin{Kind: "person", ID: "ana"}
	// A new Files for each read: Resolve keeps what it read.
	resolve := func() error {
		_, err := (&job.Files{Task: h.Task, Notes: h.Store}).Resolve("brain:project/deploy")
		return err
	}
	if err := resolve(); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Store.Drop("project", "conventions", "deploy", brain.OpDrop, rev, ana, ""); err != nil {
		t.Fatal(err)
	}
	srRefused(t, resolve(), "brain:project/deploy:", "note is dropped")
	if _, err := h.Store.Drop("project", "conventions", "deploy", brain.OpRestore, rev, ana, ""); err != nil {
		t.Fatal(err)
	}
	if err := resolve(); err != nil {
		t.Fatalf("after restore: %v", err)
	}
}
