package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func mustWrite(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func wantError(t *testing.T, err error, text string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), text) {
		t.Fatalf("error = %v, want one containing %q", err, text)
	}
}

func TestActivitiesOnFreshHomeCreatesNothing(t *testing.T) {
	home := t.TempDir()
	list, err := Store{TaskDir: home}.Activities()
	if err != nil {
		t.Fatal(err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("Activities() = %#v, want an empty non-nil slice", list)
	}
	if _, err := os.Lstat(filepath.Join(home, "sessions")); !os.IsNotExist(err) {
		t.Fatalf("sessions folder after a read: err = %v, want not exist", err)
	}
}

func TestActivitiesSkipsStraysAndFailsOnABadFile(t *testing.T) {
	home := t.TempDir()
	store := Store{TaskDir: home, Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }}
	for _, id := range []string{"s-one", "s-two"} {
		if err := store.RecordUsage(id, "tool-1", Usage{Input: 3}); err != nil {
			t.Fatal(err)
		}
	}
	hooks := filepath.Join(home, "sessions", ".hooks")
	if err := os.Mkdir(filepath.Join(hooks, "stray"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(hooks, "notes.txt"), "not an activity")
	list, err := store.Activities()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, a := range list {
		got[a.SessionID] = true
	}
	if len(list) != 2 || !got["s-one"] || !got["s-two"] {
		t.Fatalf("Activities() = %#v, want exactly s-one and s-two", list)
	}
	mustWrite(t, filepath.Join(hooks, "broken.json"), "{not json")
	if list, err := store.Activities(); err == nil {
		t.Fatalf("Activities() with a bad file = %#v, nil; want an error", list)
	}
}

func TestActivityReadsAndRefusesBadIdentity(t *testing.T) {
	home := t.TempDir()
	store := Store{TaskDir: home}
	for _, id := range []string{"../x", ""} {
		_, err := store.Activity(id)
		wantError(t, err, "invalid session id")
	}
	// No hooks folder at all, then a hooks folder without the file.
	a, err := store.Activity("unknown")
	if err != nil || a.SessionID != "unknown" || a.Usage != nil || !a.Last.IsZero() {
		t.Fatalf("Activity(unknown) without a folder = %#v, %v", a, err)
	}
	hooks := filepath.Join(home, "sessions", ".hooks")
	mustWrite(t, filepath.Join(hooks, "other.json"), `{"session_id":"other"}`)
	a, err = store.Activity("unknown")
	if err != nil || a.SessionID != "unknown" || a.Usage != nil || !a.Last.IsZero() {
		t.Fatalf("Activity(unknown) without a file = %#v, %v", a, err)
	}
	mustWrite(t, filepath.Join(hooks, "a.json"), `{"session_id":"b"}`)
	_, err = store.Activity("a")
	wantError(t, err, "activity identity mismatch")
	mustWrite(t, filepath.Join(hooks, "bad.json"), `{"session_id":`)
	if _, err = store.Activity("bad"); err == nil {
		t.Fatal("Activity(bad json) returned no error")
	}
}

func TestActivityFailsWhenHooksPathIsAFile(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, "sessions", ".hooks"), "a file where a folder belongs")
	store := Store{TaskDir: home}
	_, err := store.Activity("s-1")
	wantError(t, err, "real directories")
	_, err = store.Activities()
	wantError(t, err, "real directories")
}

func TestEndKeepsTheFirstEndAndRefusesNonEndStates(t *testing.T) {
	f := newFixture(t, false)
	k := f.record.Key
	if _, err := f.store.Sleep(k); err != nil {
		t.Fatal(err)
	}
	f.now = f.now.Add(time.Minute)
	first, err := f.store.End(k, Finished)
	if err != nil {
		t.Fatal(err)
	}
	if first.State != Finished || first.Ended == nil || !first.Ended.Equal(f.now) || first.Slept != nil {
		t.Fatalf("End(Finished) = %+v, want Finished, Ended = %s, Slept nil", first, f.now)
	}
	f.now = f.now.Add(time.Hour)
	again, err := f.store.End(k, Errored)
	if err != nil {
		t.Fatal(err)
	}
	if again.State != Finished || again.Ended == nil || !again.Ended.Equal(*first.Ended) {
		t.Fatalf("second End = %+v, want the first record unchanged", again)
	}
	shown, err := f.store.Show(k)
	if err != nil || shown.State != Finished || !shown.Ended.Equal(*first.Ended) {
		t.Fatalf("Show after second End = %+v, %v", shown, err)
	}
	for _, state := range []State{Running, Sleeping} {
		_, err := f.store.End(k, state)
		wantError(t, err, "invalid session end state")
	}
	for _, state := range []State{Errored, StoppedByYou} {
		g := newFixture(t, false)
		r, err := g.store.End(g.record.Key, state)
		if err != nil || r.State != state || r.Ended == nil {
			t.Fatalf("End(%q) = %+v, %v", state, r, err)
		}
	}
}

func TestShowRoundTripsAndRefusesBadRecords(t *testing.T) {
	f := newFixture(t, false)
	got, err := f.store.Show(f.record.Key)
	if err != nil {
		t.Fatal(err)
	}
	if got.Key != f.record.Key || got.PID != 42 || got.PIDStart != "start-1" || got.State != LivenessUnknown || got.Transcript != "transcript.jsonl" {
		t.Fatalf("Show = %+v, want the started record", got)
	}
	if _, err := f.store.Show(Key{"../x", "machine-1"}); err == nil {
		t.Fatal("Show accepted an unsafe session id")
	}
	if _, err := f.store.Show(Key{"absent", "machine-1"}); !os.IsNotExist(err) {
		t.Fatalf("Show(absent) error = %v, want not exist", err)
	}
	dir := filepath.Join(f.store.TaskDir, "sessions", "machine-1")
	raw, err := os.ReadFile(filepath.Join(dir, "session-1.json"))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "session-2.json"), string(raw))
	_, err = f.store.Show(Key{"session-2", "machine-1"})
	wantError(t, err, "session record identity does not match its path")
	mustWrite(t, filepath.Join(dir, "session-3.json"), "{")
	if _, err = f.store.Show(Key{"session-3", "machine-1"}); err == nil || !strings.Contains(err.Error(), "read session session-3") {
		t.Fatalf("Show(bad json) error = %v", err)
	}
	edit := func(id string, change func(*Record)) {
		r := got
		r.Key.SessionID = id
		change(&r)
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		mustWrite(t, filepath.Join(dir, id+".json"), string(b))
	}
	edit("no-pid", func(r *Record) { r.PID = 0 })
	_, err = f.store.Show(Key{"no-pid", "machine-1"})
	wantError(t, err, "session needs a pid and its start time")
	edit("no-start", func(r *Record) { r.PIDStart = "" })
	_, err = f.store.Show(Key{"no-start", "machine-1"})
	wantError(t, err, "session needs a pid and its start time")
	edit("zombie", func(r *Record) { r.State = "Zombie" })
	_, err = f.store.Show(Key{"zombie", "machine-1"})
	wantError(t, err, "invalid session state")
}

func TestValidateTenureJobFields(t *testing.T) {
	ok := Record{PID: 1, PIDStart: "s"}
	if err := validateTenure(ok); err != nil {
		t.Fatal(err)
	}
	ok.Lease = "J-1"
	wantError(t, validateTenure(ok), "Job liveness needs a holder, fence and attempt")
	ok.Holder, ok.Fence, ok.Attempt = "member@machine-1", 1, 1
	if err := validateTenure(ok); err != nil {
		t.Fatal(err)
	}
}

func TestListSkipsStraysAndFailsOnABadRecord(t *testing.T) {
	empty := Store{TaskDir: t.TempDir()}
	none, err := empty.List()
	if err != nil || none == nil || len(none) != 0 {
		t.Fatalf("List without a sessions folder = %#v, %v", none, err)
	}
	if _, err := os.Lstat(filepath.Join(empty.TaskDir, "sessions")); !os.IsNotExist(err) {
		t.Fatalf("List created the sessions folder: %v", err)
	}

	f := newFixture(t, false)
	other, err := f.store.Start(Record{Key: Key{"session-9", "machine-2"}, PID: 7, PIDStart: "start-9"})
	if err != nil {
		t.Fatal(err)
	}
	sessions := filepath.Join(f.store.TaskDir, "sessions")
	// Skipped: a machine folder that is not a safe component (its record would
	// fail Show), the hooks folder, a non-.json entry and a sub-folder.
	mustWrite(t, filepath.Join(sessions, "bad name", "x.json"), "{")
	mustWrite(t, filepath.Join(sessions, "machine-1", "x.tmp"), "{")
	if err := os.Mkdir(filepath.Join(sessions, "machine-1", "sub.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(sessions, ".hooks", "h.json"), "{")
	list, err := f.store.List()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[Key]bool{}
	for _, r := range list {
		seen[r.Key] = true
	}
	if len(list) != 2 || !seen[f.record.Key] || !seen[other.Key] {
		t.Fatalf("List = %+v, want session-1/machine-1 and session-9/machine-2", list)
	}
	mustWrite(t, filepath.Join(sessions, "machine-2", "broken.json"), "{")
	if list, err := f.store.List(); err == nil {
		t.Fatalf("List with a malformed record = %+v, nil; want an error", list)
	}
}

func TestListFailsWhenSessionsPathIsAFile(t *testing.T) {
	home := t.TempDir()
	mustWrite(t, filepath.Join(home, "sessions"), "a file")
	_, err := Store{TaskDir: home}.List()
	wantError(t, err, "real directories")
}

func TestAtomicJSONRefusesNonRegularAndBadValues(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "folder.json"), 0o700); err != nil {
		t.Fatal(err)
	}
	wantError(t, atomicJSON(filepath.Join(dir, "folder.json"), Activity{}), "must be regular")
	// A parent that is a file makes Lstat fail with something other than not-exist.
	mustWrite(t, filepath.Join(dir, "file"), "x")
	if err := atomicJSON(filepath.Join(dir, "file", "a.json"), Activity{}); err == nil {
		t.Fatal("atomicJSON below a file returned no error")
	}
	// A value JSON cannot encode leaves no file and no temp file behind.
	if err := atomicJSON(filepath.Join(dir, "chan.json"), make(chan int)); err == nil {
		t.Fatal("atomicJSON of a channel returned no error")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "folder.json" && e.Name() != "file" {
			t.Fatalf("atomicJSON left %q behind", e.Name())
		}
	}
	good := filepath.Join(dir, "good.json")
	if err := atomicJSON(good, Activity{SessionID: "g"}); err != nil {
		t.Fatal(err)
	}
	if err := atomicJSON(good, Activity{SessionID: "h"}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(good)
	if err != nil || !strings.Contains(string(b), `"session_id":"h"`) {
		t.Fatalf("replaced file = %q, %v", b, err)
	}
}
