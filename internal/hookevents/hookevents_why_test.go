package hookevents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A task folder is named T-NN or T-NN-words; the record names the T-NN.
func TestTaskOfNamesTheTaskID(t *testing.T) {
	for in, want := range map[string]string{
		"/x/projects/p/T-10-rate-limits": "T-10",
		"T-01":                           "T-01",
		"/x/T-7":                         "T-7",
		"/x/scratch":                     "scratch",
	} {
		if got := TaskOf(in); got != want {
			t.Errorf("TaskOf(%q) = %q, want %q", in, got, want)
		}
	}
}

// A line written before task and reason existed reads with both empty, and a
// record with neither writes exactly the four old keys.
func TestRecordTaskAndReasonAreOptional(t *testing.T) {
	home := t.TempDir()
	if err := os.Mkdir(filepath.Join(home, Dir), 0o700); err != nil {
		t.Fatal(err)
	}
	old := `{"rule":"history-log","event":"Stop","at":"2026-11-15T10:00:00Z","outcome":"ok"}` + "\n"
	if err := os.WriteFile(Path(home), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	Append(home, []Record{
		{Rule: "stop-check", Event: "Stop", At: "2026-11-16T09:00:00Z", Outcome: Blocked, Task: "T-10", Reason: "untraced narrowing"},
		{Rule: "session-start", Event: "SessionStart", At: "2026-11-16T09:01:00Z", Outcome: OK},
	})
	got, err := Read(home)
	if err != nil || len(got) != 3 {
		t.Fatalf("Read: %v, %v", got, err)
	}
	// newest first
	if got[1].Task != "T-10" || got[1].Reason != "untraced narrowing" || got[1].Outcome != Blocked {
		t.Errorf("refusal line read back as %+v", got[1])
	}
	if got[0].Task != "" || got[0].Reason != "" || got[2].Task != "" || got[2].Reason != "" {
		t.Errorf("lines without task and reason read with %+v and %+v", got[0], got[2])
	}
	raw, err := os.ReadFile(Path(home))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if want := `{"rule":"session-start","event":"SessionStart","at":"2026-11-16T09:01:00Z","outcome":"ok"}`; lines[2] != want {
		t.Errorf("a record without task and reason wrote %s, want %s", lines[2], want)
	}
	if want := `{"rule":"stop-check","event":"Stop","at":"2026-11-16T09:00:00Z","outcome":"blocked","task":"T-10","reason":"untraced narrowing"}`; lines[1] != want {
		t.Errorf("a refusal wrote %s, want %s", lines[1], want)
	}
}
