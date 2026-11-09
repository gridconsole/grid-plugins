package brain

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const hashA = "sha256:0123456789abcdef"

// The first lines are the format before versions existed, with and without the
// trailing space an empty reason leaves; the rest are the current format, two
// of them sharing a stamp, and lines a hand edit could leave behind.
const mixedLog = "# Brain log\n\n" +
	"- 2025-12-01T10:00:00.000Z | accepted | alpha | job:T-01/J-01 |\n" +
	"- 2025-12-02T10:00:00.000Z | rejected | beta | session:abc | Already covered.\n" +
	"just some words\n" +
	"- not-a-time | accepted | gamma | job:x | \n" +
	"- 2025-12-03T10:00:00.000Z | accepted | delta | job:T-02/J-01 | \n" +
	"- 2025-12-04T10:00:00.000Z | accepted | one | job:T-03/J-01 | merge-into brain:project/one | decisions | v1 | " + hashA + "\n" +
	"- 2025-12-04T10:00:00.000Z | accepted | one | job:T-03/J-02 |  | decisions | v2 | sha256:fedcba9876543210\n" +
	"- 2025-12-05T10:00:00.000Z | accepted | bad | job:T-03/J-03 |  | decisions | v1 | sha256:xyz\n" +
	"- 2025-12-05T10:00:00.000Z | accepted | short | job:T-03/J-03 |  | decisions\n"

func names(changes []Change) []string {
	var out []string
	for _, c := range changes {
		out = append(out, c.Name+"/"+c.OriginID)
	}
	return out
}

func TestChangesReadsOldAndNewLines(t *testing.T) {
	s := testStore(t)
	putFile(t, s, "projects/example/knowledge/log.md", mixedLog)
	all, err := s.Changes("project", time.Time{}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Newest first; of the two lines with one stamp the later one leads.
	want := []string{"one/T-03/J-02", "one/T-03/J-01", "delta/T-02/J-01", "beta/abc", "alpha/T-01/J-01"}
	if !reflect.DeepEqual(names(all), want) {
		t.Fatalf("changes: %v, want %v", names(all), want)
	}
	if got := all[1]; got != (Change{At: "2025-12-04T10:00:00.000Z", Status: "accepted", Topic: "decisions", Name: "one", Version: 1, Hash: hashA,
		OriginKind: "job", OriginID: "T-03/J-01", Reason: "merge-into brain:project/one"}) {
		t.Fatalf("new line: %+v", got)
	}
	if got := all[3]; got != (Change{At: "2025-12-02T10:00:00.000Z", Status: "rejected", Name: "beta", OriginKind: "session", OriginID: "abc", Reason: "Already covered."}) {
		t.Fatalf("old line: %+v", got)
	}

	since := time.Date(2025, 12, 2, 10, 0, 0, 0, time.UTC)
	if got, _ := s.Changes("project", since, 0); len(got) != 4 || got[3].Name != "beta" {
		t.Fatalf("since keeps the stamp it names: %v", names(got))
	}
	if got, _ := s.Changes("project", since, 2); !reflect.DeepEqual(names(got), want[:2]) {
		t.Fatalf("limit: %v", names(got))
	}
	if got, err := s.Changes("project", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC), 0); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("nothing since 2030: %v %v", got, err)
	}

	// A new decision lands after the old lines and leads the list.
	n := sample()
	accepted(t, s, n)
	got, err := s.Changes("project", time.Time{}, 1)
	if err != nil || len(got) != 1 || got[0].Name != "checks" || got[0].Version != 1 || got[0].Topic != "conventions" {
		t.Fatalf("latest change: %+v %v", got, err)
	}
}

func TestChangesAndVersionsTolerateNothingThere(t *testing.T) {
	for _, s := range []Store{testStore(t), {GridHome: filepath.Join(t.TempDir(), "missing"), Project: "example"}} {
		for _, level := range []string{"home", "project"} {
			if got, err := s.Changes(level, time.Time{}, 5); err != nil || got == nil || len(got) != 0 {
				t.Fatalf("%s changes: %v %v", level, got, err)
			}
			if got, err := s.Versions(level, "decisions", "none"); err != nil || got == nil || len(got) != 0 {
				t.Fatalf("%s versions: %v %v", level, got, err)
			}
		}
	}
	s := testStore(t)
	if _, err := s.Changes("elsewhere", time.Time{}, 0); err == nil {
		t.Fatal("unknown level accepted")
	}
	if _, err := s.Versions("project", "../x", "y"); err == nil {
		t.Fatal("traversal accepted")
	}
}

func TestChangesStayWithinTheirLevel(t *testing.T) {
	s := testStore(t)
	n := sample()
	n.Level, n.Topic, n.Name = "home", "decisions", "personal"
	accepted(t, s, n)
	accepted(t, s, sample())
	home, err := s.Changes("home", time.Time{}, 0)
	if err != nil || len(home) != 1 || home[0].Name != "personal" {
		t.Fatalf("home changes: %+v %v", home, err)
	}
	project, err := s.Changes("project", time.Time{}, 0)
	if err != nil || len(project) != 1 || project[0].Name != "checks" {
		t.Fatalf("project changes: %+v %v", project, err)
	}
	if v, err := s.Versions("home", "decisions", "personal"); err != nil || len(v) != 1 || v[0].Path != "knowledge/versions/decisions/personal/1.md" {
		t.Fatalf("home versions: %+v %v", v, err)
	}
	if v, err := s.Versions("project", "conventions", "checks"); err != nil || len(v) != 1 || v[0].Path != "projects/example/knowledge/versions/conventions/checks/1.md" {
		t.Fatalf("project versions: %+v %v", v, err)
	}
}
