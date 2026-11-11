package task

import (
	"os"
	"path/filepath"
	"testing"
)

// Waiting brain proposals never stop a delivery: it goes ahead and names them in
// its result, so the orchestrator can start the curator.
func TestDeliverGoesAheadWithWaitingProposalsAndNamesThem(t *testing.T) {
	s, task := deliverFixture(t, "")
	for _, name := range []string{"first-fact", "second-fact"} {
		write(t, filepath.Join(task.Path, "brain-proposals", name+".md"), "---\nstatus: proposed\ntopic: testing\nreason: Reuse "+name+"\n---\n")
	}

	dry, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid", DryRun: true})
	if err != nil {
		t.Fatalf("dry run refused: %v", err)
	}
	if len(dry.Waiting) != 2 {
		t.Fatalf("dry run: %+v", dry.Waiting)
	}

	d, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"})
	if err != nil {
		t.Fatalf("delivery refused with proposals waiting: %v", err)
	}
	if len(d.Waiting) != 2 || d.Waiting[0].ID != "first-fact" || d.Waiting[1].ID != "second-fact" {
		t.Fatalf("result does not name the two proposals: %+v", d.Waiting)
	}
	got, err := Read(task.Path)
	if err != nil || got.State != "waiting" {
		t.Fatalf("task did not reach waiting: %+v %v", got, err)
	}
	// The proposals stay where they are for the curator.
	for _, name := range []string{"first-fact", "second-fact"} {
		if _, err := os.Stat(filepath.Join(task.Path, "brain-proposals", name+".md")); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDeliverWithNoProposalsReturnsAnEmptyList(t *testing.T) {
	s, task := deliverFixture(t, "")
	d, err := s.Deliver(DeliverOptions{Cwd: task.Path, Task: task.ID, Outcome: "Ready", By: "grid"})
	if err != nil {
		t.Fatal(err)
	}
	if d.Waiting == nil || len(d.Waiting) != 0 {
		t.Fatalf("%+v", d.Waiting)
	}
}
