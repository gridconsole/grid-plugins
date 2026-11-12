package cli

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/routine"
)

// zaSchedCLIProject is a harness with a registered project holding two routines;
// only `daily` is scheduled, through the recording runner and never the real one.
func zaSchedCLIProject(t *testing.T) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	folder := filepath.Join(h.root, "work", "example")
	h.mkdir(folder)
	if code := h.run(folder, "project", "add"); code != 0 {
		t.Fatal(h.errb.String())
	}
	for _, args := range [][]string{
		{"routine", "add", "daily", "--every", "0 9 * * 1-5", "--run", "grid version", "--project", "example"},
		{"routine", "add", "hourly", "--every", "1h", "--run", "grid version", "--project", "example"},
	} {
		if code := h.run(folder, args...); code != 0 {
			t.Fatalf("%v: %s", args, h.errb)
		}
	}
	store := routine.Store{GridHome: h.gridHome, Project: "example"}
	scheduler := routine.Scheduler{Home: h.home, GridHome: h.gridHome, Binary: filepath.Join(h.root, "grid"), Backend: routine.Systemd{}, Runner: routineOS{}}
	if err := scheduler.ScheduleOne(context.Background(), store, "daily"); err != nil {
		t.Fatal(err)
	}
	h.out.Reset()
	h.errb.Reset()
	return h, folder
}

func zaSchedCLIRows(t *testing.T, h *harness, folder string) map[string]map[string]any {
	t.Helper()
	h.out.Reset()
	if code := h.run(folder, "routine", "list", "--project", "example", "--json"); code != 0 {
		t.Fatalf("list: exit %d: %s", code, h.errb)
	}
	var rows []map[string]any
	if err := json.Unmarshal(h.out.Bytes(), &rows); err != nil {
		t.Fatalf("list output: %s %v", h.out, err)
	}
	byName := map[string]map[string]any{}
	for _, row := range rows {
		byName[row["name"].(string)] = row
	}
	return byName
}

func TestZaSchedCLIListRowsSayScheduledAndNext(t *testing.T) {
	h, folder := zaSchedCLIProject(t)
	rows := zaSchedCLIRows(t, h, folder)
	daily, hourly := rows["daily"], rows["hourly"]
	next, isString := daily["next"].(string)
	if daily["scheduled"] != true || !isString {
		t.Fatalf("scheduled routine row: %v", daily)
	}
	at, err := time.Parse(time.RFC3339, next)
	if err != nil || !at.After(time.Now().Add(-time.Minute)) || at.Location() != time.UTC {
		t.Fatalf("next is not an RFC3339 UTC time ahead of now: %q %v", next, err)
	}
	if value, present := hourly["next"]; hourly["scheduled"] != false || !present || value != nil {
		t.Fatalf("unscheduled routine row: %v", hourly)
	}
	if !strings.Contains(h.out.String(), `"scheduled":false,"next":null`) || !strings.Contains(h.out.String(), `"scheduled":true,"next":"`) {
		t.Fatalf("raw rows: %s", h.out)
	}

	h.out.Reset()
	if code := h.run(folder, "routine", "list", "--project", "example"); code != 0 {
		t.Fatal(h.errb.String())
	}
	for _, want := range []string{"scheduled: true\nnext: " + next + "\n", "scheduled: false\nnext: none\n"} {
		if !strings.Contains(h.out.String(), want) {
			t.Fatalf("text list lacks %q: %s", want, h.out)
		}
	}
}

func TestZaSchedCLINamedUnscheduleOfUnknownNameRemovesNothing(t *testing.T) {
	h, folder := zaSchedCLIProject(t)
	if code := h.run(folder, "routine", "unschedule", "nosuch", "--project", "example", "--json"); code != 0 {
		t.Fatalf("unschedule nosuch: exit %d: %s", code, h.errb)
	}
	if !strings.Contains(h.out.String(), `"name":"nosuch"`) || !strings.Contains(h.out.String(), `"result":"unschedule"`) {
		t.Fatalf("reply: %s", h.out)
	}
	if rows := zaSchedCLIRows(t, h, folder); rows["daily"]["scheduled"] != true {
		t.Fatalf("unschedule of another name removed daily's timer: %v", rows["daily"])
	}
}

func TestZaSchedCLIScheduleOfUnknownNameFails(t *testing.T) {
	h, folder := zaSchedCLIProject(t)
	if code := h.run(folder, "routine", "schedule", "nosuch", "--project", "example"); code != 1 {
		t.Fatalf("schedule nosuch: exit %d: %s", code, h.errb)
	}
	if rows := zaSchedCLIRows(t, h, folder); rows["hourly"]["scheduled"] != false || rows["daily"]["scheduled"] != true {
		t.Fatalf("a refused schedule changed the rows: %v", rows)
	}
}

func TestZaSchedCLITooManyNamesExitTwo(t *testing.T) {
	h := newHarness(t)
	for _, args := range [][]string{
		{"routine", "schedule", "a", "b"},
		{"routine", "unschedule", "a", "b"},
		{"routine", "schedule", "a", "b", "--json"},
	} {
		h.errb.Reset()
		if code := h.run(h.root, args...); code != 2 || !strings.Contains(h.errb.String(), "at most 1 argument") {
			t.Fatalf("%v: exit %d: %s", args, code, h.errb)
		}
	}
	// The other verbs keep their counts.
	for _, args := range [][]string{{"routine", "remove"}, {"routine", "list", "extra"}} {
		if code := h.run(h.root, args...); code != 2 {
			t.Fatalf("%v: exit %d", args, code)
		}
	}
}

func TestZaSchedCLIUnscheduleWithoutNameKeepsItsReply(t *testing.T) {
	h := newHarness(t)
	// No timer was ever written here, so the all-routines form reaches no OS call.
	if code := h.run(h.root, "routine", "unschedule", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); got != `{"result":"unschedule","project":""}`+"\n" {
		t.Fatalf("reply changed: %q", got)
	}
	h.out.Reset()
	if code := h.run(h.root, "routine", "unschedule", "nosuch", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if got := h.out.String(); got != `{"result":"unschedule","project":"","name":"nosuch"}`+"\n" {
		t.Fatalf("named reply: %q", got)
	}
}
