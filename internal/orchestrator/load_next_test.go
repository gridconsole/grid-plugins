package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/feed"
	"github.com/gridconsole/grid-plugins/internal/job"
)

func itemIDs(items []job.Job) []string {
	out := []string{}
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestLoadSortsItemsAndFallsBackToRecordTime(t *testing.T) {
	f := setup(t)
	f.job(t, "J-3", nil)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", nil)
	// J-4 carries its own created_at; the others have none.
	body := jobBody(t, "J-4")
	body["created_at"] = "2020-01-02T03:04:05.000Z"
	r4 := f.post(t, feed.Input{Type: "job", Body: body, Job: "J-4"}, f.s.Writer)

	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := itemIDs(snap.Items), []string{"J-1", "J-2", "J-3", "J-4"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Items = %v, want %v", got, want)
	}
	for _, id := range []string{"J-1", "J-2", "J-3", "J-4"} {
		if snap.Bodies[id] == nil || snap.Bodies[id]["id"] != id {
			t.Fatalf("Bodies[%s] = %v", id, snap.Bodies[id])
		}
	}
	// No created_at in the body: Created is the record's at.
	for _, id := range []string{"J-1", "J-2", "J-3"} {
		var rec feed.Record
		for _, r := range snap.Records {
			if r.Type == "job" && r.Job != nil && *r.Job == id {
				rec = r
			}
		}
		want, err := time.Parse(time.RFC3339Nano, rec.At)
		if err != nil || rec.At == "" {
			t.Fatalf("record at %q: %v", rec.At, err)
		}
		if got := snap.Created[id]; !got.Equal(want) {
			t.Fatalf("Created[%s] = %v, want record at %v", id, got, want)
		}
	}
	// A body created_at wins over the record time.
	if got := snap.Created["J-4"]; !got.Equal(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Fatalf("Created[J-4] = %v (record at %s)", got, r4.At)
	}
}

func TestLoadDropsRecordsThatAreNotCounted(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	// A record whose "re" target was never posted is held, not counted.
	held := f.post(t, feed.Input{Type: "control", Re: "01ARZ3NDEKTSV4RRFFQ69G5FAX", Body: map[string]any{"target": "J-1", "reason": "x"}}, f.s.Writer)

	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	verdict := ""
	for _, e := range r.Records {
		if e.ID == held.ID {
			verdict = e.Verdict
		}
	}
	if verdict == "" || verdict == "counted" {
		t.Fatalf("fixture record verdict = %q, want not counted", verdict)
	}

	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := snap.Records[held.ID]; ok {
		t.Fatalf("a %s record is in Records", verdict)
	}
	counted := 0
	for _, v := range snap.State.Verdicts {
		if v.Kind == "counted" {
			counted++
		}
	}
	if len(snap.Records) != counted || len(snap.Records) == 0 {
		t.Fatalf("Records %d, counted verdicts %d", len(snap.Records), counted)
	}
}

func TestLoadDropsAnIgnoredRecord(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	f.job(t, "J-2", nil)
	r, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	var j2 feed.Record
	for _, e := range r.Records {
		if e.Type == "job" && e.Job != nil && *e.Job == "J-2" {
			j2 = e.Record
		}
	}
	if j2.ID == "" {
		t.Fatal("J-2 record not found")
	}
	// A third Job record in the writer's own file whose body is empty: it fails
	// the body schema on read, so its verdict is ignored.
	bad := j2
	bad.ID, err = feed.NewID()
	if err != nil {
		t.Fatal(err)
	}
	bad.Seq++
	bad.Body = feed.Body{}
	line, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(filepath.Join(f.s.Feed.Dir(), fmt.Sprintf("%s.%d.jsonl", bad.Worker, bad.Fence)), os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.Write(append(line, '\n'))
	file.Close()
	if err != nil {
		t.Fatal(err)
	}

	snap, err := Load(f.s.Feed)
	if err != nil {
		t.Fatal(err)
	}
	// The feed read marks it ignored and keeps it out of the fold, so it has no
	// counted verdict.
	read, err := f.s.Feed.Read(feed.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	kind := ""
	for _, e := range read.Records {
		if e.ID == bad.ID {
			kind = e.Verdict
		}
	}
	if kind != "ignored" {
		t.Fatalf("verdict of the invalid record = %q, want ignored", kind)
	}
	if _, ok := snap.Records[bad.ID]; ok {
		t.Fatal("an ignored record is in Records")
	}
	if got, want := itemIDs(snap.Items), []string{"J-1", "J-2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Items = %v, want %v", got, want)
	}
}

func TestSplitMissingTargets(t *testing.T) {
	mk := func(id string, after ...string) job.Job {
		j := job.Job{ID: id}
		for _, a := range after {
			j.After = append(j.After, job.Dependency{Job: a})
		}
		return j
	}
	for _, tc := range []struct {
		name       string
		in         []job.Job
		keep, held []string
	}{
		{"missing target holds it and what follows", []job.Job{mk("J-1"), mk("J-2", "J-9"), mk("J-3", "J-2")}, []string{"J-1"}, []string{"J-2", "J-3"}},
		{"gate target passes", []job.Job{mk("J-1", "gate:plan")}, []string{"J-1"}, []string{}},
		{"chain of three behind one missing target", []job.Job{mk("J-1", "J-9"), mk("J-2", "J-1"), mk("J-3", "J-2")}, []string{}, []string{"J-1", "J-2", "J-3"}},
		{"one missing among several targets holds it", []job.Job{mk("J-1"), mk("J-2", "J-1", "J-9")}, []string{"J-1"}, []string{"J-2"}},
		{"empty", []job.Job{}, []string{}, []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keep, held := splitMissingTargets(tc.in)
			if got := itemIDs(keep); !reflect.DeepEqual(got, tc.keep) {
				t.Fatalf("keep = %v, want %v", got, tc.keep)
			}
			if got := itemIDs(held); !reflect.DeepEqual(got, tc.held) {
				t.Fatalf("held = %v, want %v", got, tc.held)
			}
		})
	}
	t.Run("empty input returns empty slices", func(t *testing.T) {
		keep, held := splitMissingTargets(nil)
		if len(keep) != 0 || len(held) != 0 {
			t.Fatalf("keep %v held %v", keep, held)
		}
	})
}

func TestNextWithoutWaitReturnsAfterOnePass(t *testing.T) {
	f := setup(t)
	start := time.Now()
	out, err := f.s.Next(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	if out.Assigned != 0 || out.Wake != nil || len(out.Spawns) != 0 {
		t.Fatalf("empty feed gave %+v", out)
	}
	if d := time.Since(start); d >= time.Second {
		t.Fatalf("wait=false took %v", d)
	}
}

func TestNextWaitReturnsAssignedJobBeforeTheTick(t *testing.T) {
	f := setup(t)
	f.job(t, "J-1", nil)
	pauses := 0
	f.s.pause = func(context.Context) error {
		pauses++
		return errors.New("TestNextWaitReturnsAssignedJobBeforeTheTick: Next waited between passes")
	}
	out, err := f.s.Next(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	if out.Assigned != 1 || len(out.Spawns) != 1 || out.Spawns[0].Job != "J-1" {
		t.Fatalf("Next = %+v", out)
	}
	if pauses != 0 {
		t.Fatalf("Next waited %d times, want 0", pauses)
	}
}

func TestNextWaitStopsWhenContextIsCancelled(t *testing.T) {
	f := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	out, err := f.s.Next(ctx, true)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, out = %+v", err, out)
	}
	if out.Assigned != 0 {
		t.Fatalf("Assigned = %d", out.Assigned)
	}
	if d := time.Since(start); d < 40*time.Millisecond || d >= time.Second {
		t.Fatalf("returned after %v, want about 50ms", d)
	}
}

func memoryPath(f *fixture) string { return filepath.Join(f.s.Feed.Dir(), ".orchestrator.json") }

func TestReadMemory(t *testing.T) {
	t.Run("missing file is empty", func(t *testing.T) {
		f := setup(t)
		if err := os.Remove(memoryPath(f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
		m, err := f.s.readMemory()
		if err != nil {
			t.Fatal(err)
		}
		if m.Jobs == nil || m.Delivered == nil || len(m.Jobs) != 0 || len(m.Delivered) != 0 || len(m.Wakes) != 0 || len(m.Pending) != 0 {
			t.Fatalf("%+v", m)
		}
	})
	t.Run("null jobs is invalid", func(t *testing.T) {
		f := setup(t)
		if err := os.WriteFile(memoryPath(f), []byte(`{"jobs":null,"delivered":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.s.readMemory(); err == nil || !strings.Contains(err.Error(), "invalid orchestrator state") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("null delivered is invalid", func(t *testing.T) {
		f := setup(t)
		if err := os.WriteFile(memoryPath(f), []byte(`{"jobs":{},"delivered":null}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := f.s.readMemory(); err == nil || !strings.Contains(err.Error(), "invalid orchestrator state") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("malformed json errors", func(t *testing.T) {
		f := setup(t)
		if err := os.WriteFile(memoryPath(f), []byte(`{"jobs":`), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := f.s.readMemory()
		if err == nil || strings.Contains(err.Error(), "invalid orchestrator state") {
			t.Fatalf("err = %v, want a JSON syntax error", err)
		}
	})
}

func TestWriteJSONSkipsAnUnchangedFile(t *testing.T) {
	f := setup(t)
	path := filepath.Join(t.TempDir(), "state.json")
	v := map[string]any{"a": 1, "b": []string{"x"}}
	if err := writeJSON(path, v); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(path, v); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("same value rewrote the file: mtime %v, want %v", info.ModTime(), old)
	}
	// A changed value does write.
	if err := writeJSON(path, map[string]any{"a": 2}); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.ModTime().Equal(old) {
		t.Fatal("changed value did not rewrite the file")
	}
	// The same through the memory file.
	m := memory{Jobs: map[string]*jobMemory{}, Delivered: map[string]bool{}, Wakes: []time.Time{}, Pending: []Wake{}}
	if err := f.s.saveMemory(m); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(memoryPath(f), old, old); err != nil {
		t.Fatal(err)
	}
	if err := f.s.saveMemory(m); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(memoryPath(f))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("saveMemory rewrote an unchanged file: %v", info.ModTime())
	}
}

func TestHostCutsAfterTheLastAt(t *testing.T) {
	for in, want := range map[string]string{"a@b@c": "c", "w": "w", "worker@machine": "machine", "": "", "a@": ""} {
		if got := host(in); got != want {
			t.Errorf("host(%q) = %q, want %q", in, got, want)
		}
		if got := workerHost(in); got != want {
			t.Errorf("workerHost(%q) = %q, want %q", in, got, want)
		}
	}
}
