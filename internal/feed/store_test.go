package feed

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const testTask = "01ARZ3NDEKTSV4RRFFQ69G5FAV"
const testMachine = "01ARZ3NDEKTSV4RRFFQ69G5FAW"

func testWriter() Writer {
	return Writer{Author: Author{Member: "member", Role: "member"}, Worker: "member@" + testMachine}
}
func testStore(t *testing.T) Store { t.Helper(); return Store{TaskDir: t.TempDir(), TaskID: testTask} }
func message(text string) Input {
	return Input{Type: "post", Body: map[string]any{"ref": "task", "kind": "message", "text": text}}
}
func mustPost(t *testing.T, s Store, in Input, w Writer) Record {
	t.Helper()
	r, err := s.Post(in, w)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func testLease(t *testing.T, s Store, w Writer, attempt *int) {
	t.Helper()
	dir := filepath.Join(s.Dir(), "lease")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(Lease{Lease: w.Lease, Holder: &w.Worker, Fence: w.Fence, Gen: w.Gen, Attempt: attempt})
	if err := os.WriteFile(filepath.Join(dir, w.Lease+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestAppendAndRead(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	r := mustPost(t, s, message("example"), w)
	if r.Seq != 1 || r.TypeV != 1 {
		t.Fatalf("record: %+v", r)
	}
	in := message("example")
	in.ID = r.ID
	retry := mustPost(t, s, in, w)
	if retry.ID != r.ID || retry.Seq != 1 {
		t.Fatal("retry changed record")
	}
	mustPost(t, s, Input{Type: "question", Job: "J-1", Body: map[string]any{"question": "Proceed?"}}, w)
	for _, tc := range []struct {
		name string
		f    Filter
		n    int
		bad  bool
	}{
		{"all", Filter{}, 2, false}, {"type", Filter{Type: "post"}, 1, false}, {"job", Filter{Job: "J-1"}, 1, false}, {"since id", Filter{Since: r.ID}, 1, false}, {"since time", Filter{Since: "2000-01-01T00:00:00Z"}, 2, false}, {"invalid since", Filter{Since: "invalid"}, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			read, err := s.Read(tc.f)
			if (err != nil) != tc.bad {
				t.Fatalf("error: %v", err)
			}
			if err == nil && len(read.Records) != tc.n {
				t.Fatalf("got %d records", len(read.Records))
			}
		})
	}
	data, err := os.ReadFile(filepath.Join(s.Dir(), writerFile(w.Worker, 0)))
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(completeLines(data)[0], &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 17 || fields["lease"] != nil || fields["epoch"] != nil {
		t.Fatalf("envelope: %v", fields)
	}
}

func TestTailRepairAndLargeBody(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	first := mustPost(t, s, message("first"), w)
	file := filepath.Join(s.Dir(), writerFile(w.Worker, 0))
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteString(`{"torn":`)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(Filter{})
	if err != nil || len(read.Records) != 1 {
		t.Fatalf("torn read: %+v %v", read, err)
	}
	second := mustPost(t, s, Input{Type: "knowledge", Body: map[string]any{"title": "Large", "text": strings.Repeat("x", MaxLine*2)}}, w)
	if second.Seq != first.Seq+1 {
		t.Fatal("bad seq")
	}
	read, err = s.Read(Filter{})
	if err != nil || len(read.Records) != 2 || read.Bad[filepath.Base(file)] != 1 {
		t.Fatalf("repair: %+v %v", read, err)
	}
	if str(read.Records[1].Body["text"]) != str(second.Body["text"]) {
		t.Fatal("body did not round trip")
	}
	data, _ := os.ReadFile(file)
	for _, line := range completeLines(data) {
		if len(line)+1 > MaxLine {
			t.Fatal("oversized line")
		}
	}
}

func TestConcurrentGoroutines(t *testing.T) {
	s := testStore(t)
	const workers = 20
	const each = 10
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < each; j++ {
				if _, err := s.Post(message(fmt.Sprintf("%d/%d", i, j)), testWriter()); err != nil {
					errs <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	checkAppends(t, s, workers*each)
}

func TestConcurrentProcesses(t *testing.T) {
	if dir := os.Getenv("GRID_FEED_TEST_CHILD"); dir != "" {
		s := Store{TaskDir: dir, TaskID: testTask}
		for i := 0; i < 12; i++ {
			if _, err := s.Post(message(fmt.Sprintf("%d/%d", os.Getpid(), i)), testWriter()); err != nil {
				t.Fatal(err)
			}
		}
		return
	}
	s := testStore(t)
	cmds := make([]*exec.Cmd, 4)
	for i := range cmds {
		cmds[i] = exec.Command(os.Args[0], "-test.run=^TestConcurrentProcesses$")
		cmds[i].Env = append(os.Environ(), "GRID_FEED_TEST_CHILD="+s.TaskDir)
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	}
	checkAppends(t, s, 48)
}
func checkAppends(t *testing.T, s Store, want int) {
	t.Helper()
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Records) != want || len(read.Bad) != 0 {
		t.Fatalf("records=%d bad=%v want=%d", len(read.Records), read.Bad, want)
	}
	ids := map[string]bool{}
	texts := map[string]bool{}
	for i, e := range read.Records {
		if e.Seq != int64(i+1) || ids[e.ID] || texts[str(e.Body["text"])] {
			t.Fatalf("lost order or duplicate at %d", i)
		}
		ids[e.ID] = true
		texts[str(e.Body["text"])] = true
	}
}

func TestShortWriteRepairsBeforeRetry(t *testing.T) {
	s := testStore(t)
	w := testWriter()
	first := mustPost(t, s, message("first"), w)
	r, err := s.record(message("retry"), w)
	if err != nil {
		t.Fatal(err)
	}
	r.Seq = first.Seq + 1
	line, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	line = append(line, '\n')
	path := filepath.Join(s.Dir(), writerFile(w.Worker, 0))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := lock(f, true); err != nil {
		t.Fatal(err)
	}
	defer unlock(f)
	calls := 0
	err = appendRecord(f, path, line, func(file *os.File, data []byte) (int, error) {
		calls++
		if calls == 1 {
			return writeOnce(file, data[:12])
		}
		return writeOnce(file, data)
	})
	if err != nil {
		t.Fatal(err)
	}
	read, err := s.Read(Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(read.Records) != 2 || read.Bad[filepath.Base(path)] != 1 || read.Records[1].ID != r.ID {
		t.Fatalf("retry: calls=%d records=%+v bad=%v", calls, read.Records, read.Bad)
	}
}
