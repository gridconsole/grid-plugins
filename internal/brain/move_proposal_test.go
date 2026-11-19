package brain

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The move and the keeper gate both take the project lock before the home lock,
// so run together they finish.
func TestMoveProposalAndKeeperGateDoNotDeadlock(t *testing.T) {
	gh := t.TempDir()
	taskDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(gh, "projects", "example"), 0o700); err != nil {
		t.Fatal(err)
	}
	s := Store{GridHome: gh, Project: "example"}
	done := make(chan error, 2)
	for i := 0; i < 50; i++ {
		name := "p" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		if _, err := s.Propose(Note{Name: name, Topic: "decisions", Description: "Before checks.", Summary: "Checks.", Source: "x", Body: "Run.", Level: "home"}, Origin{Kind: "session", ID: "s"}); err != nil {
			t.Fatal(err)
		}
		go func() { done <- s.moveProposal(name, "home", "project", true, nil) }()
		go func() { done <- s.WithKeeperGate(taskDir, func() error { return nil }) }()
		for j := 0; j < 2; j++ {
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("move and keeper gate deadlocked")
			}
		}
	}
}
