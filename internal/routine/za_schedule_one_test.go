package routine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/gridhome"
)

type zaSchedBackend struct {
	name    string
	backend Backend
}

func zaSchedBackends() []zaSchedBackend {
	return []zaSchedBackend{{"linux", Systemd{}}, {"darwin", Launchd{UID: 501}}, {"windows", Schtasks{}}}
}

// zaSchedFixture is a project scope with two routines of this machine and a
// scheduler whose OS calls are recorded, never run.
type zaSchedFixture struct {
	store Store
	s     Scheduler
	calls *fakeOS
}

func zaSchedNewFixture(t *testing.T, backend Backend, now time.Time) zaSchedFixture {
	t.Helper()
	home := gridhome.Real(t.TempDir())
	gh := filepath.Join(home, "data & space")
	store := Store{GridHome: gh, Project: "example", Now: func() time.Time { return now }}
	if _, err := store.Add("daily", "0 9 * * 1-5", "grid version"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Add("hourly", "1h", "grid version"); err != nil {
		t.Fatal(err)
	}
	calls := &fakeOS{}
	s := Scheduler{Home: home, GridHome: gh, Binary: filepath.Join(home, "bin", "grid"), Backend: backend, Runner: calls}
	return zaSchedFixture{store: store, s: s, calls: calls}
}

func zaSchedPlan(t *testing.T, f zaSchedFixture, name string) TimerPlan {
	t.Helper()
	p, err := f.s.plan(f.store.Project, name)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// zaSchedRecords lists the ownership record files of the machine.
func zaSchedRecords(t *testing.T, f zaSchedFixture) []string {
	t.Helper()
	path, err := f.s.statePath("probe")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func zaSchedMentions(calls []OSCommand, label string) bool {
	for _, c := range calls {
		b, _ := json.Marshal(c)
		if strings.Contains(string(b), label) {
			return true
		}
	}
	return false
}

func zaSchedRows(t *testing.T, store Store) map[string]Entry {
	t.Helper()
	entries, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	rows := map[string]Entry{}
	for _, e := range entries {
		rows[e.Name] = e
	}
	return rows
}

func zaSchedEdit(t *testing.T, store Store, name string, edit func(*Entry)) {
	t.Helper()
	e, err := store.Get(name)
	if err != nil {
		t.Fatal(err)
	}
	edit(&e)
	if err := store.save(e); err != nil {
		t.Fatal(err)
	}
}

func zaSchedAt(value string) time.Time {
	at, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return at
}

func zaSchedWantNext(t *testing.T, rows map[string]Entry, name, want string) {
	t.Helper()
	got := rows[name].Next
	if got == nil || !got.Equal(zaSchedAt(want)) || got.Location() != time.UTC {
		t.Fatalf("%s next: %v, want %s", name, got, want)
	}
}

func TestZaSchedScheduleOneTouchesOnlyItsRoutine(t *testing.T) {
	for _, tc := range zaSchedBackends() {
		t.Run(tc.name, func(t *testing.T) {
			f := zaSchedNewFixture(t, tc.backend, zaSchedAt("2026-11-16T08:59:30Z"))
			daily, hourly := zaSchedPlan(t, f, "daily"), zaSchedPlan(t, f, "hourly")
			if len(daily.Install) == 0 || len(daily.Remove) == 0 {
				t.Fatalf("plan has no OS commands: %+v", daily)
			}
			if err := f.s.ScheduleOne(context.Background(), f.store, "daily"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(f.calls.calls, daily.Install) {
				t.Fatalf("install commands: %v, want daily's %v", f.calls.calls, daily.Install)
			}
			if !zaSchedMentions(f.calls.calls, daily.Label) || zaSchedMentions(f.calls.calls, hourly.Label) {
				t.Fatalf("install commands name the wrong label: %v", f.calls.calls)
			}
			if got := zaSchedRecords(t, f); !reflect.DeepEqual(got, []string{daily.Label + ".json"}) {
				t.Fatalf("ownership records after schedule: %v", got)
			}
			state, err := f.s.readState(daily)
			if err != nil || !state.Installed || state.Name != "daily" || state.Project != "example" {
				t.Fatalf("daily record: %+v %v", state, err)
			}
			entry, err := f.store.Get("daily")
			if err != nil {
				t.Fatal(err)
			}
			if hash, _ := entry.Fingerprint(); state.Hash != hash {
				t.Fatalf("record hash %q is not the entry fingerprint %q", state.Hash, hash)
			}
			for _, file := range daily.Files {
				if _, err := os.Stat(file.Path); err != nil {
					t.Fatalf("daily timer file missing: %v", err)
				}
			}
			for _, file := range hourly.Files {
				if _, err := os.Stat(file.Path); !os.IsNotExist(err) {
					t.Fatalf("hourly timer file written: %s %v", file.Path, err)
				}
			}

			installed := len(f.calls.calls)
			if err := f.s.Unschedule(context.Background(), f.store.Project, "daily"); err != nil {
				t.Fatal(err)
			}
			want := append(append([]OSCommand{}, daily.Remove...), daily.AfterRemove...)
			if got := f.calls.calls[installed:]; !reflect.DeepEqual(got, want) {
				t.Fatalf("remove commands: %v, want daily's %v", got, want)
			}
			if zaSchedMentions(f.calls.calls, hourly.Label) {
				t.Fatalf("hourly was named in an OS call: %v", f.calls.calls)
			}
			if got := zaSchedRecords(t, f); len(got) != 0 {
				t.Fatalf("ownership records after unschedule: %v", got)
			}
			for _, file := range daily.Files {
				if _, err := os.Stat(file.Path); !os.IsNotExist(err) {
					t.Fatalf("daily timer file remains: %s %v", file.Path, err)
				}
			}
			if _, err := f.store.Get("daily"); err != nil {
				t.Fatalf("unschedule removed the routine itself: %v", err)
			}
		})
	}
}

func TestZaSchedUnscheduleOneLeavesTheOtherScheduled(t *testing.T) {
	for _, tc := range zaSchedBackends() {
		t.Run(tc.name, func(t *testing.T) {
			f := zaSchedNewFixture(t, tc.backend, zaSchedAt("2026-11-16T08:59:30Z"))
			daily, hourly := zaSchedPlan(t, f, "daily"), zaSchedPlan(t, f, "hourly")
			for _, name := range []string{"daily", "hourly"} {
				if err := f.s.ScheduleOne(context.Background(), f.store, name); err != nil {
					t.Fatal(err)
				}
			}
			hourlyPath, err := f.s.statePath(hourly.Label)
			if err != nil {
				t.Fatal(err)
			}
			before, err := os.ReadFile(hourlyPath)
			if err != nil {
				t.Fatal(err)
			}
			calls := len(f.calls.calls)
			if err := f.s.Unschedule(context.Background(), f.store.Project, "daily"); err != nil {
				t.Fatal(err)
			}
			if zaSchedMentions(f.calls.calls[calls:], hourly.Label) {
				t.Fatalf("unschedule of daily ran hourly's commands: %v", f.calls.calls[calls:])
			}
			after, err := os.ReadFile(hourlyPath)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("hourly record changed: %q %q %v", before, after, err)
			}
			rows := zaSchedRows(t, f.store)
			if rows["daily"].Scheduled || !rows["hourly"].Scheduled {
				t.Fatalf("scheduled flags: daily %v hourly %v", rows["daily"].Scheduled, rows["hourly"].Scheduled)
			}
			if _, err := os.Stat(daily.Files[0].Path); !os.IsNotExist(err) {
				t.Fatalf("daily timer file remains: %v", err)
			}
		})
	}
}

func TestZaSchedNamedVerbsRefuseOrIgnoreUnknownNames(t *testing.T) {
	for _, tc := range zaSchedBackends() {
		t.Run(tc.name, func(t *testing.T) {
			f := zaSchedNewFixture(t, tc.backend, zaSchedAt("2026-11-16T08:59:30Z"))
			err := f.s.ScheduleOne(context.Background(), f.store, "nosuch")
			// The store's own not-found error, the one Get gives.
			if _, getErr := f.store.Get("nosuch"); err == nil || getErr == nil || err.Error() != getErr.Error() || !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("schedule of an unknown routine: %v, store.Get says %v", err, getErr)
			}
			if err := f.s.Unschedule(context.Background(), f.store.Project, "nosuch"); err != nil {
				t.Fatalf("unschedule of a name with no record: %v", err)
			}
			if len(f.calls.calls) != 0 || len(zaSchedRecords(t, f)) != 0 {
				t.Fatalf("an unknown name reached the OS or wrote a record: %v %v", f.calls.calls, zaSchedRecords(t, f))
			}
			if _, err := f.s.PreviewOne(f.store, "nosuch", false); err == nil {
				t.Fatal("preview of an unknown routine did not fail")
			}
		})
	}
}

func TestZaSchedPreviewOneMatchesPreviewForThatName(t *testing.T) {
	for _, tc := range zaSchedBackends() {
		t.Run(tc.name, func(t *testing.T) {
			f := zaSchedNewFixture(t, tc.backend, zaSchedAt("2026-11-16T08:59:30Z"))
			for _, remove := range []bool{false, true} {
				all, err := f.s.Preview(f.store, remove)
				if err != nil {
					t.Fatal(err)
				}
				one, err := f.s.PreviewOne(f.store, "daily", remove)
				if err != nil {
					t.Fatal(err)
				}
				var want []TimerPlan
				for _, p := range all {
					if p.Name == "daily" {
						want = append(want, p)
					}
				}
				if len(want) != 1 || !reflect.DeepEqual(one, want) {
					t.Fatalf("remove=%v: preview of daily %+v, want %+v", remove, one, want)
				}
			}
			if len(f.calls.calls) != 0 || len(zaSchedRecords(t, f)) != 0 {
				t.Fatal("preview wrote state or ran a command")
			}
		})
	}
}

func TestZaSchedListSaysScheduledAndNext(t *testing.T) {
	now := zaSchedAt("2026-11-16T08:59:30Z") // a Monday
	clock := now
	f := zaSchedNewFixture(t, Systemd{}, now)
	f.store.Now = func() time.Time { return clock }
	if _, err := f.store.Add("idle", "30 3 * * *", "grid version"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"daily", "hourly"} {
		if err := f.s.ScheduleOne(context.Background(), f.store, name); err != nil {
			t.Fatal(err)
		}
	}
	rows := zaSchedRows(t, f.store)
	if !rows["daily"].Scheduled || !rows["hourly"].Scheduled {
		t.Fatalf("scheduled routines not marked: %+v", rows)
	}
	zaSchedWantNext(t, rows, "daily", "2026-11-16T09:00:00Z")
	// A duration routine that never ran is due now.
	zaSchedWantNext(t, rows, "hourly", "2026-11-16T08:59:30Z")
	if rows["idle"].Scheduled || rows["idle"].Next != nil {
		t.Fatalf("unscheduled routine: scheduled %v next %v", rows["idle"].Scheduled, rows["idle"].Next)
	}
	raw, err := json.Marshal([]Entry{rows["daily"], rows["idle"]})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"scheduled":true,"next":"2026-11-16T09:00:00Z"`, `"scheduled":false,"next":null`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("list JSON lacks %s: %s", want, raw)
		}
	}

	// A duration counts from last_run; last_run is not part of the fingerprint.
	zaSchedEdit(t, f.store, "hourly", func(e *Entry) { e.LastRun, e.Outcome = zaSchedAt("2026-11-16T08:10:00Z"), "success" })
	zaSchedWantNext(t, zaSchedRows(t, f.store), "hourly", "2026-11-16T09:10:00Z")

	// Friday 09:00 has run: the next minute that matches is Monday.
	clock = zaSchedAt("2026-11-20T09:00:30Z")
	zaSchedEdit(t, f.store, "daily", func(e *Entry) { e.LastRun, e.Outcome = zaSchedAt("2026-11-20T09:00:00Z"), "success" })
	rows = zaSchedRows(t, f.store)
	if !rows["daily"].Scheduled {
		t.Fatal("a run changed the fingerprint")
	}
	zaSchedWantNext(t, rows, "daily", "2026-11-23T09:00:00Z")

	// A routine of another machine is never scheduled here, whatever record exists.
	zaSchedEdit(t, f.store, "hourly", func(e *Entry) { e.Machine = "01ARZ3NDEKTSV4RRFFQ69G5FAV" })
	if got := zaSchedRows(t, f.store)["hourly"]; got.Scheduled || got.Next != nil {
		t.Fatalf("routine of another machine: scheduled %v next %v", got.Scheduled, got.Next)
	}

	// Changing the command or interval on disk ends the approval, as for timer runs.
	zaSchedEdit(t, f.store, "daily", func(e *Entry) { e.Every = "0 10 * * 1-5" })
	if got := zaSchedRows(t, f.store)["daily"]; got.Scheduled || got.Next != nil {
		t.Fatalf("changed routine: scheduled %v next %v", got.Scheduled, got.Next)
	}
	if err := f.s.ScheduleOne(context.Background(), f.store, "daily"); err != nil {
		t.Fatal(err)
	}
	rows = zaSchedRows(t, f.store)
	if !rows["daily"].Scheduled {
		t.Fatal("schedule did not approve the changed routine")
	}
	// It is still Friday 09:00:30, so the new 10:00 minute is today's.
	zaSchedWantNext(t, rows, "daily", "2026-11-20T10:00:00Z")
}

func TestZaSchedRunReportsItsOwnSchedule(t *testing.T) {
	f := zaSchedNewFixture(t, Systemd{}, zaSchedAt("2026-11-16T08:59:30Z"))
	if err := f.s.ScheduleOne(context.Background(), f.store, "daily"); err != nil {
		t.Fatal(err)
	}
	runner := executorFunc(func(context.Context, Store, Entry) error { return nil })
	result, err := f.store.Execute(context.Background(), "daily", false, runner, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Scheduled || result.Next == nil {
		t.Fatalf("run of a scheduled routine reports scheduled %v next %v", result.Scheduled, result.Next)
	}
	zaSchedWantNext(t, map[string]Entry{"daily": result.Entry}, "daily", "2026-11-16T09:00:00Z")
	result, err = f.store.Execute(context.Background(), "hourly", false, runner, nil)
	if err != nil || result.Scheduled || result.Next != nil {
		t.Fatalf("run of an unscheduled routine: %+v %v", result.Entry, err)
	}
}

func TestZaSchedNextWalksMinutesLikeDue(t *testing.T) {
	for _, tc := range []struct {
		name, every      string
		after, last, now string
		want             string // empty: no minute within 366 days
	}{
		{"weekday morning before it", "0 9 * * 1-5", "2026-11-16T08:59:30Z", "", "", "2026-11-16T09:00:00Z"},
		{"friday after the run", "0 9 * * 1-5", "2026-11-20T09:00:30Z", "2026-11-20T09:00:00Z", "", "2026-11-23T09:00:00Z"},
		{"the minute that is now is not after now", "0 9 * * *", "2026-11-16T09:00:30Z", "", "", "2026-11-17T09:00:00Z"},
		{"on the minute is still after", "0 9 * * *", "2026-11-16T09:00:00Z", "", "", "2026-11-17T09:00:00Z"},
		{"a run in a later minute skips that minute", "* * * * *", "2026-11-16T09:00:30Z", "2026-11-16T09:01:10Z", "", "2026-11-16T09:02:00Z"},
		{"day of month or day of week", "0 9 15 * 1", "2026-11-11T12:00:00Z", "", "", "2026-11-15T09:00:00Z"},
		{"day of week beats a later day of month", "0 9 20 * 1", "2026-11-11T12:00:00Z", "", "", "2026-11-16T09:00:00Z"},
		{"star with a day of month", "0 9 20 * *", "2026-11-11T12:00:00Z", "", "", "2026-11-20T09:00:00Z"},
		{"month and year roll over", "30 23 31 12 *", "2026-11-11T12:00:00Z", "", "", "2026-12-31T23:30:00Z"},
		{"sunday as seven", "0 0 * * 7", "2026-11-16T00:00:00Z", "", "", "2026-11-22T00:00:00Z"},
		{"no such day", "0 0 31 2 *", "2026-11-11T12:00:00Z", "", "", ""},
		{"leap day is more than a year away", "0 0 29 2 *", "2026-03-01T00:00:00Z", "", "", ""},
		{"duration from last run", "1h", "2026-11-16T08:59:30Z", "2026-11-16T08:10:00Z", "", "2026-11-16T09:10:00Z"},
		{"duration never run is due now", "1h", "2026-11-16T08:59:30Z", "", "", "2026-11-16T08:59:30Z"},
		{"duration overdue is due now", "30m", "2026-11-16T08:59:30Z", "2026-11-16T06:00:00Z", "", "2026-11-16T08:59:30Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interval, err := ParseInterval(tc.every)
			if err != nil {
				t.Fatal(err)
			}
			var last time.Time
			if tc.last != "" {
				last = zaSchedAt(tc.last)
			}
			got, ok := interval.Next(zaSchedAt(tc.after), last)
			if tc.want == "" {
				if ok {
					t.Fatalf("expected no minute, got %s", got)
				}
				return
			}
			if !ok || !got.Equal(zaSchedAt(tc.want)) || got.Location() != time.UTC {
				t.Fatalf("next: %s %v, want %s", got, ok, tc.want)
			}
			if interval.Duration == 0 && !interval.Due(got, last) {
				t.Fatalf("Next gave %s, which Due refuses", got)
			}
		})
	}
}
