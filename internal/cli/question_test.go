package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/question"
)

func questionAdd(t *testing.T, h *harness, task, who string, extra ...string) question.Record {
	t.Helper()
	args := []string{"question", "add", "What should ship?", "--who", who, "--default", "Wait", "--applies", "2099-01-01T00:00:00Z", "--json"}
	args = append(args, extra...)
	if code := h.run(task, args...); code != 0 {
		t.Fatalf("add %d: %s", code, h.errb)
	}
	var r question.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func TestAnswerFeedAndRecovery(t *testing.T) {
	h, task := feedHarness(t)
	q := questionAdd(t, h, task, "visitor", "--job", "J-2.1", "--item", "3")
	// A person must clear agent lease markers before answering.
	t.Setenv("GRID_LEASE", "J-2.1")
	t.Setenv("GRID_LEASE_EPOCH", "7.42")
	if code := h.run(h.gridHome, "answer", task, q.ID, "Ship a text export", "--json"); code != 3 {
		t.Fatalf("lease-marked answer exit %d", code)
	}
	t.Setenv("GRID_LEASE", "")
	t.Setenv("GRID_LEASE_EPOCH", "")
	if code := h.run(h.gridHome, "answer", task, q.ID, "Ship a text export", "--json"); code != 0 {
		t.Fatalf("answer %d: %s", code, h.errb)
	}
	var a question.Record
	if err := json.Unmarshal(h.out.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if a.Post != q.Post || a.Answer != "Ship a text export" || a.Gate.Kind != "person" || a.Job != "J-2.1" || a.Item != 3 || len(a.Options) != 0 {
		t.Fatalf("projection: %+v", a)
	}
	f := feed.Store{TaskDir: task, TaskID: feedTaskID}
	read, err := f.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != 2 {
		t.Fatalf("records: %+v", read.Records)
	}
	for _, p := range read.Records {
		if p.Fence != 0 || p.Lease != nil || p.File != "member@"+feedMachine+".0.jsonl" {
			t.Fatalf("writer: %+v", p)
		}
		if p.Type == "question" {
			gate, _ := json.Marshal(p.Body["gate"])
			fileGate, _ := json.Marshal(a.Gate)
			var x, y any
			_ = json.Unmarshal(gate, &x)
			_ = json.Unmarshal(fileGate, &y)
			gx, _ := json.Marshal(x)
			gy, _ := json.Marshal(y)
			if !bytes.Equal(gx, gy) {
				t.Fatalf("gate mismatch: %s %s", gx, gy)
			}
		}
		if p.Type == "answer" && (p.Re == nil || *p.Re != q.Post || a.History[0].Post != p.ID || a.History[0].Who != "member" || a.History[0].Role != "member") {
			t.Fatalf("answer link: %+v %+v", p, a)
		}
	}
	state, err := f.State()
	if err != nil {
		t.Fatal(err)
	}
	if state.Questions[q.Post].Open || len(state.NeedsYou) != 0 || state.Questions[q.Post].Applied == nil {
		t.Fatalf("gate did not close: %+v", state)
	}
	path := filepath.Join(task, "questions", q.ID+".md")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, damage := range []string{"delete", "edit", "invalid"} {
		t.Run(damage, func(t *testing.T) {
			switch damage {
			case "delete":
				err = os.Remove(path)
			case "edit":
				err = os.WriteFile(path, bytes.ReplaceAll(original, []byte("Ship a text export"), []byte("Hand edited")), 0600)
			case "invalid":
				err = os.WriteFile(path, []byte("broken front matter"), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if code := h.run(h.gridHome, "question", "sync", task); code != 0 {
				t.Fatalf("sync %d: %s", code, h.errb)
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(original, got) {
				t.Fatalf("repair: %s %v", got, err)
			}
		})
	}
	if code := h.run(task, "answer", task, q.ID, "Ship a text export"); code != 0 {
		t.Fatalf("retry: %s", h.errb)
	}
	read, err = f.Read(feed.Filter{})
	if err != nil || len(read.Records) != 2 {
		t.Fatalf("retry duplicated post: %v %+v", err, read)
	}
}

func TestAnswerCallerRoles(t *testing.T) {
	for _, role := range []string{"owner", "member", "lead", "agent", "cli", "guest"} {
		t.Run(role, func(t *testing.T) {
			h, task := feedHarness(t)
			q := questionAdd(t, h, task, "visitor")
			t.Setenv("GRID_MEMBER", "visitor")
			t.Setenv("GRID_ROLE", role)
			if role == "agent" || role == "cli" || role == "lead" || role == "guest" {
				if code := h.run(task, "answer", task, q.ID, "Yes"); code == 0 {
					t.Fatal("non-person answer accepted")
				}
				read, err := (feed.Store{TaskDir: task, TaskID: feedTaskID}).Read(feed.Filter{Type: "answer"})
				if err != nil || len(read.Records) != 0 {
					t.Fatalf("refused answer posted: %+v %v", read, err)
				}
				return
			}
			if code := h.run(task, "answer", task, q.ID, "Yes", "--json"); code != 0 {
				t.Fatalf("answer %d: %s", code, h.errb)
			}
			var a question.Record
			if err := json.Unmarshal(h.out.Bytes(), &a); err != nil {
				t.Fatal(err)
			}
			if a.History[0].Who != "visitor" || a.History[0].Role != role {
				t.Fatalf("stamp: %+v", a.History)
			}
			t.Setenv("GRID_MEMBER", "other")
			t.Setenv("GRID_ROLE", "guest")
			if code := h.run(task, "answer", task, q.ID, "Yes"); code == 0 {
				t.Fatal("other guest's identical retry was accepted")
			}
		})
	}
	h, task := feedHarness(t)
	q := questionAdd(t, h, task, "visitor")
	t.Setenv("GRID_ROLE", "guest")
	t.Setenv("GRID_MEMBER", "other")
	if code := h.run(task, "answer", task, q.ID, "No"); code == 0 {
		t.Fatal("unaddressed guest answered")
	}
	read, err := (feed.Store{TaskDir: task, TaskID: feedTaskID}).Read(feed.Filter{Type: "answer"})
	if err != nil || len(read.Records) != 0 {
		t.Fatalf("refused answer posted: %+v %v", read, err)
	}
}

func TestQuestionCommandSurface(t *testing.T) {
	h, task := feedHarness(t)
	q := questionAdd(t, h, task, "member")
	for _, args := range [][]string{
		{"question", "answer", q.ID, "Yes"},
		{"answer", task, q.ID, "Yes", "--by", "owner"},
		{"question", "withdraw", q.ID, "--by", "owner"},
	} {
		if code := h.run(task, args...); code != 2 {
			t.Fatalf("old surface %v: %d %s", args, code, h.errb)
		}
	}
	if code := h.run(task, "answer", "--help"); code != 0 || !bytes.Contains(h.out.Bytes(), []byte("grid answer <task>")) {
		t.Fatalf("help: %d %s", code, h.out)
	}
}

// An answer post closes a person gate, so grid feed post refuses the type for
// every role, GRID_ROLE unset included (the CLI then writes as member).
func TestFeedPostRefusesAnswer(t *testing.T) {
	for _, role := range []string{"", "agent", "cli", "member", "owner"} {
		t.Run("role="+role, func(t *testing.T) {
			h, task := feedHarness(t)
			q := questionAdd(t, h, task, "member")
			t.Setenv("GRID_ROLE", role)
			if code := h.run(task, "feed", "post", "--type", "answer", "--re", q.Post, "--body", `{"answer":"Yes"}`); code != 1 || !strings.Contains(h.errb.String(), "grid answer") {
				t.Fatalf("feed post %d: %s", code, h.errb)
			}
			read, err := (feed.Store{TaskDir: task, TaskID: feedTaskID}).Read(feed.Filter{Type: "answer"})
			if err != nil || len(read.Records) != 0 {
				t.Fatalf("refused answer posted: %+v %v", read, err)
			}
			state, err := (feed.Store{TaskDir: task, TaskID: feedTaskID}).State()
			if err != nil {
				t.Fatal(err)
			}
			if !state.Questions[q.Post].Open || state.Questions[q.Post].Applied != nil || len(state.NeedsYou) != 1 {
				t.Fatalf("refused answer closed gate: %+v", state)
			}
			if code := h.run(task, "question", "list", "--json"); code != 0 {
				t.Fatalf("list %d: %s", code, h.errb)
			}
			var listed []question.Record
			if err := json.Unmarshal(h.out.Bytes(), &listed); err != nil {
				t.Fatal(err)
			}
			if len(listed) != 1 || listed[0].State != "open" || len(listed[0].History) != 0 {
				t.Fatalf("refused answer projected: %+v", listed)
			}
		})
	}
}
