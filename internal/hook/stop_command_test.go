package hook

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/feed"
)

func stopReminder(t *testing.T, f *eventFixture) string {
	t.Helper()
	_, log, code := f.run(t, "Stop", stopInput("Stop"))
	if code != 2 {
		t.Fatalf("no result: code %d, want 2: %s", code, log)
	}
	if strings.Count(strings.TrimSpace(log), "\n") != 0 || !strings.HasSuffix(log, "\n") {
		t.Fatalf("want exactly one line: %q", log)
	}
	return log
}

func TestStopReminderNamesResultCommandWithGridTask(t *testing.T) {
	f := eventSetup(t)
	id, err := feed.TaskID(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	f.env["GRID_JOB"] = "J-1"
	f.env["GRID_TASK"] = id
	log := stopReminder(t, f)
	want := "grid job result --task " + id + " --job J-1 --body"
	if !strings.Contains(log, want) {
		t.Fatalf("line lacks %q: %q", want, log)
	}
}

func TestStopReminderNamesResultCommandWithTaskFolder(t *testing.T) {
	f := eventSetup(t)
	f.env["GRID_JOB"] = "J-1"
	log := stopReminder(t, f)
	want := "grid job result --task " + f.dir + " --job J-1 --body"
	if !strings.Contains(log, want) {
		t.Fatalf("line lacks %q: %q", want, log)
	}
}

func TestStopReminderSaysBlockedJobPostsBlocker(t *testing.T) {
	f := eventSetup(t)
	f.env["GRID_JOB"] = "J-1"
	if log := stopReminder(t, f); !strings.Contains(log, "blocker") {
		t.Fatalf("line does not name the blocker: %q", log)
	}
}

func TestStopReminderCommandMatchesRunJobSkill(t *testing.T) {
	raw, err := os.ReadFile("../../skills/run-job/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	var skill string
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(string(raw), -1) {
		if strings.HasPrefix(m[1], "grid job result") && strings.Contains(m[1], "--task") {
			skill = m[1]
			break
		}
	}
	if skill == "" {
		t.Fatal("no `grid job result --task ...` command in the run-job skill")
	}
	f := eventSetup(t)
	id, err := feed.TaskID(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	f.env["GRID_JOB"] = "J-1"
	f.env["GRID_TASK"] = id
	log := stopReminder(t, f)
	want := strings.NewReplacer("<task>", id, "<job>", "J-1").Replace(skill)
	if !strings.Contains(log, want) {
		t.Fatalf("line lacks the skill command %q: %q", want, log)
	}
}
