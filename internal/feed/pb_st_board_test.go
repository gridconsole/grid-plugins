package feed

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/board/postfile"
	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/machinefolder"
)

const (
	pbStTask  = "01K9J0000000000000000000T1"
	pbStHomeA = "01K9J00000000000000000000A"
	pbStHomeB = "01K9J00000000000000000000B"
)

var (
	pbStLead   = Writer{Author: Author{ID: "lead", Role: "lead"}}
	pbStCli    = Writer{Author: Author{ID: "orchestrator", Role: "cli"}}
	pbStWorker = Writer{Author: Author{ID: "worker", Role: "worker"}}
)

// pbStMachine is a machine folder of its own.
func pbStMachine(t *testing.T) machinefolder.Folder {
	return machinefolder.Folder{Path: t.TempDir(), Source: machinefolder.SourceEnv}
}

// pbStTaskMd writes task.md with the task id and a home: line naming home.
func pbStTaskMd(t *testing.T, dir, home string) {
	t.Helper()
	md := "---\nid: " + pbStTask + "\ntitle: Board task\nhome: " + home + "\n---\n\n# Board task\n"
	if err := os.WriteFile(filepath.Join(dir, "task.md"), []byte(md), 0o600); err != nil {
		t.Fatal(err)
	}
}

// pbStStore is a board Store on machine A. Its task.md names the task and the given
// home; the Store finds the task id there.
func pbStStore(t *testing.T, home string) Store {
	t.Helper()
	dir := t.TempDir()
	pbStTaskMd(t, dir, home)
	return Store{TaskDir: dir, Machine: pbStMachine(t), Board: &BoardHome{Rel: "projects/demo/tasks/T-1", By: pbStHomeA + ":grid"}}
}

func pbStPost(t *testing.T, s Store, in Input, w Writer) Record {
	t.Helper()
	r, err := s.Post(in, w)
	if err != nil {
		t.Fatalf("post %s: %v", in.Type, err)
	}
	return r
}

// pbStRunning posts J-1's record naming home A, the cli's assign of attempt 1 to
// worker@A from A, and the worker's ack: J-1 runs.
func pbStRunning(t *testing.T, s Store) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "contract", "fixtures", "job-record", "valid", "basic.json"))
	if err != nil {
		t.Fatal(err)
	}
	var job map[string]any
	if err := json.Unmarshal(raw, &job); err != nil {
		t.Fatal(err)
	}
	job["id"], job["home"] = "J-1", pbStHomeA
	pbStPost(t, s, Input{Type: "job", Job: "J-1", Body: job}, pbStLead)
	a := contract.Assignment{Job: "J-1", Attempt: 1, To: "worker@" + pbStHomeA, Fence: 1, Until: contract.FormatTime(time.Now().UTC().Add(time.Hour))}
	assign := pbStPost(t, s, Input{Type: "control", Body: map[string]any{"assign": a}}, pbStCli)
	pbStPost(t, s, Input{Type: "ack", Job: "J-1", Attempt: 1, After: []string{assign.ID}, Body: map[string]any{}}, pbStWorker)
}

func pbStJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// pbStFold is the fold of s as JSON, after checking that J-1 runs and that the home
// is home.
func pbStFold(t *testing.T, s Store, home string) string {
	t.Helper()
	state, err := s.State()
	if err != nil {
		t.Fatal(err)
	}
	if j := state.Jobs["J-1"]; j == nil || j.Status != contract.Running {
		t.Fatalf("J-1 on the board: %+v", j)
	}
	got, err := s.Home()
	if err != nil {
		t.Fatal(err)
	}
	if got != home {
		t.Fatalf("home %q, want %q", got, home)
	}
	return pbStJSON(t, state)
}

// The facade (brain-board.md Q3, internal/feed/fold.go:84-184): on a board the posts
// are files in board/, and fold.lock, state.json and needs-you.json are in the
// machine folder with the board lock, seq and names cache. The task folder gets no
// feed/.
func TestPbStBoardCachesInMachineFolder(t *testing.T) {
	s := pbStStore(t, pbStHomeA)
	pbStRunning(t, s)
	state, err := s.SaveState()
	if err != nil {
		t.Fatal(err)
	}
	if j := state.Jobs["J-1"]; j == nil || j.Status != contract.Running {
		t.Fatalf("J-1 after SaveState: %+v", j)
	}
	cache := filepath.Join(s.Machine.Path, "tasks", pbStTask)
	for _, name := range []string{"state.json", "needs-you.json", "fold.lock", "board.lock", "board.seq", "board.cache.json"} {
		if _, err := os.Stat(filepath.Join(cache, name)); err != nil {
			t.Errorf("%s is not in the machine folder: %v", name, err)
		}
	}
	data, err := os.ReadFile(filepath.Join(cache, "state.json"))
	if err != nil || string(data) != pbStJSON(t, state)+"\n" {
		t.Errorf("state.json is not the fold: %v\n%s", err, data)
	}
	ents, err := os.ReadDir(s.TaskDir)
	if err != nil {
		t.Fatal(err)
	}
	var top []string
	for _, e := range ents {
		top = append(top, e.Name())
	}
	if !slices.Equal(top, []string{"board", "task.md"}) {
		t.Errorf("the task folder holds %v, want only board and task.md", top)
	}
	names, err := filepath.Glob(filepath.Join(s.TaskDir, "board", "*.md"))
	if err != nil || len(names) != 3 {
		t.Fatalf("board posts %v: %v", names, err)
	}
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, e := range read.Records {
		if e.Verdict != "counted" {
			t.Errorf("%s %s reads %s %s", e.Type, e.ID, e.Verdict, e.Reason)
		}
		types = append(types, e.Type)
	}
	if !slices.Equal(types, []string{"job", "control", "ack"}) {
		t.Errorf("read order %v, want job, control, ack by seq", types)
	}
}

// Row 4 (V9, decision point 4): the fold reports the current home from the posts, and
// a task.md whose home: names another machine does not change it.
func TestPbStBoardHomeFromPostsNotTaskMd(t *testing.T) {
	s := pbStStore(t, pbStHomeB)
	pbStRunning(t, s)
	want := pbStFold(t, s, pbStHomeA)
	pbStTaskMd(t, s.TaskDir, pbStHomeA)
	if got := pbStFold(t, s, pbStHomeA); got != want {
		t.Fatalf("task.md home: changed the fold\n%s\n%s", want, got)
	}
	// The owner's task-move of gen 1 to B moves the home; task.md still names A.
	pbStPost(t, s, Input{Type: "task-move", Body: map[string]any{"from": pbStHomeA, "to": pbStHomeB}}, Writer{Author: Author{ID: "owner", Role: "owner"}, Gen: 1})
	moved := pbStFold(t, s, pbStHomeB)
	pbStTaskMd(t, s.TaskDir, pbStHomeA)
	if got := pbStFold(t, s, pbStHomeB); got != moved {
		t.Fatalf("task.md home: changed the fold after the move\n%s\n%s", moved, got)
	}
}

// Row 10 (V15): editing or deleting the markdown body of posts leaves the fold
// unchanged, on this machine's names cache, on a machine folder that reads the files
// as they are now, and after the slow pass finds the edits.
func TestPbStBoardBodyEditLeavesFold(t *testing.T) {
	s := pbStStore(t, pbStHomeA)
	pbStRunning(t, s)
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	want := pbStFold(t, s, pbStHomeA)
	statePath := filepath.Join(s.Machine.Path, "tasks", pbStTask, "state.json")
	stateBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	names, err := filepath.Glob(filepath.Join(s.TaskDir, "board", "*.md"))
	if err != nil || len(names) != 3 {
		t.Fatalf("board posts %v: %v", names, err)
	}
	for i, name := range names {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		_, md, err := postfile.Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		head := data[:len(data)-len(md)]
		switch i {
		case 0: // an edit inside the body
			data = append(append([]byte{}, head...), []byte("Edited by hand in Obsidian.\n\n"+string(md))...)
		case 1: // the body replaced
			data = append(append([]byte{}, head...), []byte("Rewritten.\n")...)
		case 2: // the body deleted
			data = append([]byte{}, head...)
		}
		if err := os.WriteFile(name, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if got := pbStFold(t, s, pbStHomeA); got != want {
		t.Fatalf("fold changed on the names cache\n%s\n%s", want, got)
	}
	fresh := s
	fresh.Machine = pbStMachine(t)
	if got := pbStFold(t, fresh, pbStHomeA); got != want {
		t.Fatalf("fold changed reading the edited files\n%s\n%s", want, got)
	}
	d, err := s.boardDir()
	if err != nil {
		t.Fatal(err)
	}
	findings, err := d.Check()
	if err != nil || len(findings) != 3 {
		t.Fatalf("Check: %v %v, want three edited posts", findings, err)
	}
	if got := pbStFold(t, s, pbStHomeA); got != want {
		t.Fatalf("fold changed after Check\n%s\n%s", want, got)
	}
	if _, err := s.SaveState(); err != nil {
		t.Fatal(err)
	}
	if again, err := os.ReadFile(statePath); err != nil || string(again) != string(stateBytes) {
		t.Fatalf("state.json changed: %v\n%s\n%s", err, stateBytes, again)
	}
	after, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if pbStJSON(t, after.Records) != pbStJSON(t, read.Records) {
		t.Fatalf("read changed\n%s\n%s", pbStJSON(t, read.Records), pbStJSON(t, after.Records))
	}
}
