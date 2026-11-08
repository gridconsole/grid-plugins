package task

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func waitingFixture(t *testing.T) (Store, Task) {
	t.Helper()
	s, task := writeFixture(t)
	if _, err := s.Change(WriteOptions{ID: task.ID, Action: "status", State: "waiting"}); err != nil {
		t.Fatal(err)
	}
	return s, task
}

func TestApprovalTracksOpaqueHash(t *testing.T) {
	s, task := waitingFixture(t)
	o := WriteOptions{ID: task.ID, Note: "Reviewed"}
	if _, err := s.Approve(o, ""); err == nil {
		t.Fatal("approved absent hash")
	}
	write(t, filepath.Join(task.Path, "plan.hash"), "opaque:v1\n")
	event, err := s.Approve(o, "")
	if err != nil || event.PlanHash != "opaque:v1" || event.Note != "Reviewed" {
		t.Fatalf("%+v: %v", event, err)
	}
	got, err := s.Approval("", task.ID)
	if err != nil || got != event {
		t.Fatalf("%+v: %v", got, err)
	}
	before, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
	oldApproval, _ := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	write(t, filepath.Join(task.Path, "plan.hash"), "opaque:v2\n")
	if _, err := s.Approval("", task.ID); err == nil || !strings.Contains(err.Error(), "Changed since you approved") {
		t.Fatalf("stale gate accepted: %v", err)
	}
	for _, hash := range []string{"opaque:v1", ""} {
		if _, err := s.Approve(o, hash); err == nil {
			t.Fatalf("approved stale hash %q", hash)
		}
	}
	after, _ := os.ReadFile(filepath.Join(task.Path, "history.md"))
	approval, _ := os.ReadFile(filepath.Join(task.Path, "approval.json"))
	if !bytes.Equal(before, after) || !bytes.Equal(oldApproval, approval) {
		t.Fatal("refused approval wrote data")
	}
	if _, err := s.Approve(o, "opaque:v2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Approval("", task.ID); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"close", "reopen"} {
		if _, err := s.Change(WriteOptions{ID: task.ID, Action: action}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.Approval("", task.ID); err == nil {
		t.Fatal("reopen reused an old approval")
	}
}

func TestApproveStateAndFiles(t *testing.T) {
	for _, state := range []string{"queued", "running", "closed", "waiting"} {
		t.Run(state, func(t *testing.T) {
			s, task := writeFixture(t)
			if state != "queued" {
				o := WriteOptions{ID: task.ID, Action: "status", State: state}
				if state == "closed" {
					o.Action = "close"
				}
				if _, err := s.Change(o); err != nil {
					t.Fatal(err)
				}
			}
			write(t, filepath.Join(task.Path, "plan.hash"), "hash")
			_, err := s.Approve(WriteOptions{ID: task.ID}, "hash")
			if (err == nil) != (state == "waiting") {
				t.Fatalf("%s: %v", state, err)
			}
		})
	}
	for _, name := range []string{"plan.hash", "approval.json"} {
		t.Run("symlink/"+name, func(t *testing.T) {
			s, task := waitingFixture(t)
			write(t, filepath.Join(task.Path, "plan.hash"), "hash")
			target := filepath.Join(filepath.Dir(task.Path), "outside")
			write(t, target, "hash")
			if name == "plan.hash" {
				if err := os.Remove(filepath.Join(task.Path, name)); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Symlink(target, filepath.Join(task.Path, name)); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Approve(WriteOptions{ID: task.ID}, "hash"); err == nil {
				t.Fatal("followed symlink")
			}
		})
	}
}

func TestRefusalDecisions(t *testing.T) {
	for _, decision := range []string{"allow", "deny"} {
		t.Run(decision, func(t *testing.T) {
			s, task := writeFixture(t)
			o := WriteOptions{ID: task.ID, Note: "One command only"}
			if _, err := s.Decide(o, "R-1", decision); err == nil {
				t.Fatal("decided a missing refusal")
			}
			write(t, filepath.Join(task.Path, "refusals", "R-1.json"), `{"id":"R-1","command":"echo example","rule":"Bash"}`)
			for _, id := range []string{"../R-1", "R-0", "R-1/other"} {
				if _, err := s.Decide(o, id, decision); err == nil {
					t.Fatal("accepted invalid id")
				}
			}
			if _, err := s.Decide(o, "R-1", "yes"); err == nil {
				t.Fatal("accepted invalid decision")
			}
			event, err := s.Decide(o, "R-1", decision)
			if err != nil {
				t.Fatal(err)
			}
			data, _ := os.ReadFile(filepath.Join(task.Path, "refusal-R-1.json"))
			var got Event
			if err := json.Unmarshal(data, &got); err != nil || got != event || got.Decision != decision || got.FromState != got.ToState {
				t.Fatalf("%+v: %v", got, err)
			}
			if _, err := s.Decide(o, "R-1", decision); err == nil {
				t.Fatal("overwrote decision")
			}
			lines, err := s.History("", task.ID)
			if err != nil || len(lines) != 1 {
				t.Fatalf("%v: %v", lines, err)
			}
			if _, err := s.Change(WriteOptions{ID: task.ID, Action: "close"}); err != nil {
				t.Fatal(err)
			}
			write(t, filepath.Join(task.Path, "refusals", "R-2.json"), `{}`)
			if _, err := s.Decide(o, "R-2", decision); err == nil {
				t.Fatal("decided a closed task")
			}
		})
	}
}
