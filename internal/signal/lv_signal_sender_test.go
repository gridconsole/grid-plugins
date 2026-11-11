package signal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
)

// lvSigHeader returns the front matter lines of the note at path, without the fences.
func lvSigHeader(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	head, _, ok := strings.Cut(strings.TrimPrefix(string(data), "---\n"), "\n---\n")
	if !ok {
		t.Fatalf("no closing front matter in %q", data)
	}
	return strings.Split(head, "\n")
}

// lvSigLines returns the sender and recipient lines of a header, in file order.
func lvSigLines(header []string) []string {
	var got []string
	for _, l := range header {
		if strings.HasPrefix(l, "sender:") || strings.HasPrefix(l, "recipient:") {
			got = append(got, l)
		}
	}
	return got
}

func lvSigWant(t *testing.T, path string, want ...string) {
	t.Helper()
	got := lvSigLines(lvSigHeader(t, path))
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("sender/recipient lines %q, want %q", got, want)
	}
}

var lvSigBoth = []string{"sender: Marieke", "recipient: the vertex address"}

func lvSigAddBoth(t *testing.T, s Store, sourceID string) (Result, string) {
	t.Helper()
	r, err := s.Add(Input{Source: "mail", SourceID: sourceID, Title: "T", Sender: "Marieke", Recipient: "the vertex address", Body: "Body"})
	if err != nil {
		t.Fatal(err)
	}
	path, err := s.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	return r, path
}

func TestLvSigAddWritesBothKeys(t *testing.T) {
	s := fixture(t)
	r, path := lvSigAddBoth(t, s, "m-1")
	if r.Result != "written" {
		t.Fatalf("result %q, want written", r.Result)
	}
	lvSigWant(t, path, lvSigBoth...)
	n, err := read(path)
	if err != nil || n.Sender != "Marieke" || n.Recipient != "the vertex address" {
		t.Fatalf("read back %+v %v", n, err)
	}
}

// Absent fields write no key, so a note added without them keeps the exact key
// set and order the signal-note golden holds.
func TestLvSigAbsentFieldsWriteNoKey(t *testing.T) {
	s := fixture(t)
	_, path := actNote(t, s, "lv-absent")
	var keys []string
	for _, l := range lvSigHeader(t, path) {
		keys = append(keys, l[:strings.Index(l, ":")])
	}
	want := "id source source_id title status captured updated tier body_kind"
	if strings.Join(keys, " ") != want {
		t.Fatalf("keys %v, want %s", keys, want)
	}
}

func TestLvSigValidate(t *testing.T) {
	long := strings.Repeat("x", 513)
	fit := strings.Repeat("é", 256) // 512 bytes
	for _, tc := range []struct {
		name string
		in   Input
		want string
	}{
		{"sender-newline", Input{Source: "mail", Title: "T", Sender: "Mar\nieke"}, "sender must be one line"},
		{"recipient-newline", Input{Source: "mail", Title: "T", Recipient: "a\nb"}, "recipient must be one line"},
		{"sender-cr", Input{Source: "mail", Title: "T", Sender: "a\rb"}, "sender must be one line"},
		{"recipient-line-separator", Input{Source: "mail", Title: "T", Recipient: "a b"}, "recipient must be one line"},
		{"sender-header-injection", Input{Source: "mail", Title: "T", Sender: "x\nstatus: done"}, "sender must be one line"},
		{"sender-long", Input{Source: "mail", Title: "T", Sender: long}, "sender must be UTF-8 text of at most 512 bytes"},
		{"recipient-long", Input{Source: "mail", Title: "T", Recipient: long}, "recipient must be UTF-8 text of at most 512 bytes"},
		{"sender-bad-utf8", Input{Source: "mail", Title: "T", Sender: "a\xffb"}, "sender must be UTF-8 text of at most 512 bytes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixture(t)
			err := Validate(tc.in)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("Validate: %v, want %q", err, tc.want)
			}
			if _, err := s.Add(tc.in); err == nil || err.Error() != tc.want {
				t.Fatalf("Add: %v, want %q", err, tc.want)
			}
			if entries, err := os.ReadDir(filepath.Join(s.GridHome, "projects", s.Project, "notes")); err == nil && len(entries) != 0 {
				t.Fatal("a refused add wrote a note")
			}
		})
	}
	if err := Validate(Input{Source: "mail", Title: "T", Sender: fit, Recipient: fit}); err != nil {
		t.Fatalf("512 bytes must be accepted: %v", err)
	}
}

// A value the YAML dump could read back as another type must still read back as text.
func TestLvSigRoundTripsLookalikes(t *testing.T) {
	for i, v := range []string{"true", "123", "null", "~", "yes", "1e3", "0x1F", "a: b", "# c", " lead", "trail ", `"q"`, "it's", "- dash", "[x]", "{y}", "Zoë 🙂"} {
		s := fixture(t)
		r, err := s.Add(Input{Source: "mail", SourceID: "lk-" + string(rune('a'+i)), Title: "T", Sender: v, Recipient: v})
		if err != nil {
			t.Fatalf("%q: %v", v, err)
		}
		path, _ := s.path(r.ID)
		n, err := read(path)
		if err != nil || n.Sender != v || n.Recipient != v {
			t.Fatalf("%q read back as %q / %q (%v)\n%s", v, n.Sender, n.Recipient, err, strings.Join(lvSigHeader(t, path), "\n"))
		}
	}
}

// A second add for the same source id is a duplicate and leaves the first sender alone.
func TestLvSigDuplicateKeepsFirstSender(t *testing.T) {
	s := fixture(t)
	_, path := lvSigAddBoth(t, s, "dup")
	r, err := s.Add(Input{Source: "mail", SourceID: "dup", Title: "T", Sender: "Other", Recipient: "elsewhere", Body: "Body"})
	if err != nil || r.Result != "duplicate" {
		t.Fatalf("%+v %v", r, err)
	}
	lvSigWant(t, path, lvSigBoth...)
}

// Apply re-encodes the note from the struct; the keys must ride along through
// every act, and a move patches the header in place.
func TestLvSigKeysSurviveActs(t *testing.T) {
	s := fixture(t)
	r, path := lvSigAddBoth(t, s, "life")
	if err := os.MkdirAll(filepath.Join(s.GridHome, "projects", s.Project, "tasks", "T-7"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, o := range []ActOptions{
		{Act: "snooze", Until: "2030-01-02T09:00"},
		{Act: "wake"},
		{Act: "note"},
		{Act: "reopen"},
		{Act: "add", Task: "T-7"},
		{Act: "reopen"},
		{Act: "dismiss"},
	} {
		if _, err := s.Apply(r.ID, o); err != nil {
			t.Fatalf("%s: %v", o.Act, err)
		}
		lvSigWant(t, path, lvSigBoth...)
		if n, err := read(path); err != nil || n.Sender != "Marieke" || n.Recipient != "the vertex address" {
			t.Fatalf("%s: read back %+v %v", o.Act, n, err)
		}
	}
}

func TestLvSigKeysSurviveMove(t *testing.T) {
	s := fixture(t)
	if err := project.Write(s.GridHome, project.Record{Name: "second"}); err != nil {
		t.Fatal(err)
	}
	r, _ := lvSigAddBoth(t, s, "mv")
	if _, err := s.Move(r.ID, "example", "second"); err != nil {
		t.Fatal(err)
	}
	moved := s
	moved.Project = "second"
	path, err := moved.path(r.ID)
	if err != nil {
		t.Fatal(err)
	}
	lvSigWant(t, path, lvSigBoth...)
	// The move changed new to placed; a snooze after it still keeps both lines.
	if _, err := moved.Apply(r.ID, ActOptions{Act: "snooze", Until: "2030-01-02T09:00"}); err != nil {
		t.Fatal(err)
	}
	lvSigWant(t, path, lvSigBoth...)
}
