package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/task"
)

func ncSummaryFile(fixture task.Task) string { return filepath.Join(fixture.Path, "summary.md") }

func ncSummaryRead(t *testing.T, fixture task.Task) (string, bool) {
	t.Helper()
	data, err := os.ReadFile(ncSummaryFile(fixture))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data), true
}

func TestNcSummaryWritesAndAnswers(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	if code := h.run(h.home, "task", "summary", fixture.ID, "--for", "customer", "--text", "  Plain words.  "); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	if got, _ := ncSummaryRead(t, fixture); got != "## Owner\n\n## Developer\n\n## Customer\n\nPlain words.\n" {
		t.Fatalf("summary.md = %q", got)
	}
	if code := h.run(h.home, "task", "summary", fixture.ID, "--for", "OWNER", "--text", "Owner words.", "--json"); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var event task.Event
	if err := json.Unmarshal(h.out.Bytes(), &event); err != nil || event.What != "summary" || event.Note != "summary · owner" || event.Task != fixture.ID {
		t.Fatalf("receipt = %+v, %v", event, err)
	}
	if got, _ := ncSummaryRead(t, fixture); got != "## Owner\n\nOwner words.\n\n## Developer\n\n## Customer\n\nPlain words.\n" {
		t.Fatalf("summary.md = %q", got)
	}
	// The task is also reachable by its folder name and path.
	if code := h.run(h.home, "task", "summary", fixture.Path, "--for", "developer", "--text", "Dev words."); code != 0 {
		t.Fatalf("path operand exit %d: %s", code, h.errb)
	}
	if code := h.run(h.home, "task", "history", fixture.ID); code != 0 || strings.Count(h.out.String(), "summary · ") != 3 {
		t.Fatalf("history exit %d: %s", code, h.out)
	}
}

func TestNcSummaryRefusals(t *testing.T) {
	h := newHarness(t)
	fixture := taskFixture(t, h)
	// Absent first, then unchanged after a real write.
	for _, written := range []bool{false, true} {
		if written {
			if code := h.run(h.home, "task", "summary", fixture.ID, "--for", "owner", "--text", "Kept."); code != 0 {
				t.Fatalf("exit %d: %s", code, h.errb)
			}
		}
		was, _ := ncSummaryRead(t, fixture)
		for _, tc := range []struct {
			name string
			args []string
			code int
			want string
		}{
			{"unknown reader", []string{"task", "summary", fixture.ID, "--for", "boss", "--text", "x"}, 2, "--for must be owner, developer or customer"},
			{"no reader", []string{"task", "summary", fixture.ID, "--text", "x"}, 2, "--for must be owner, developer or customer"},
			{"blank text", []string{"task", "summary", fixture.ID, "--for", "owner", "--text", "   "}, 2, "a summary needs words"},
			{"no text", []string{"task", "summary", fixture.ID, "--for", "owner"}, 2, "a summary needs words"},
			{"heading line", []string{"task", "summary", fixture.ID, "--for", "owner", "--text", "a\n## Customer\nb"}, 2, "would open a new section"},
			{"control character", []string{"task", "summary", fixture.ID, "--for", "owner", "--text", "a\x1bb"}, 2, "forbidden control"},
			{"unknown flag", []string{"task", "summary", fixture.ID, "--for", "owner", "--text", "x", "--bogus"}, 2, ""},
			{"no task", []string{"task", "summary", "--for", "owner", "--text", "x"}, 2, "Usage:"},
			{"unknown task", []string{"task", "summary", "T-99", "--for", "owner", "--text", "x"}, 1, "grid task summary: scope: task T-99 not found"},
		} {
			if code := h.run(h.home, tc.args...); code != tc.code || !strings.Contains(h.errb.String(), tc.want) {
				t.Fatalf("%s (written=%t): exit %d, stderr %q; want %d and %q", tc.name, written, code, h.errb, tc.code, tc.want)
			}
			if now, ok := ncSummaryRead(t, fixture); now != was || ok != written {
				t.Fatalf("%s (written=%t): summary.md = %q (present %t), want %q", tc.name, written, now, ok, was)
			}
		}
	}
}

func TestNcSummaryGuestRefused(t *testing.T) {
	h := newHarness(t)
	h.record(project.Record{Name: "example", ID: "01EXAMPLE"})
	fixture := taskFixtureProject(t, h, "example")
	// An invited guest sees the task, so the role rule answers; an uninvited one
	// gets the unknown-task answer the other task writers give.
	h.write(filepath.Join(h.gridHome, "projects", "example", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	h.env.Getenv = func(key string) string {
		return map[string]string{"GRID_ROLE": "guest", "GRID_MEMBER": "gus"}[key]
	}
	if code := h.run(h.home, "task", "summary", fixture.ID, "--for", "owner", "--text", "x"); code != 1 ||
		h.errb.String() != "grid task summary: scope: role cannot write task summaries\n" {
		t.Fatalf("guest: exit %d, stderr %q, stdout %q", code, h.errb, h.out)
	}
	if _, ok := ncSummaryRead(t, fixture); ok {
		t.Fatal("a guest refusal wrote summary.md")
	}
	// The member role may write.
	h.env.Getenv = func(key string) string { return map[string]string{"GRID_ROLE": "member"}[key] }
	if code := h.run(h.home, "task", "summary", fixture.ID, "--for", "owner", "--text", "x"); code != 0 {
		t.Fatalf("member: exit %d: %s", code, h.errb)
	}
}

func TestNcSummaryInProcess(t *testing.T) {
	ok, why := hsIpAnswer(hsIpTable, []string{"task", "summary", "T-01", "--for", "owner", "--text", "x"})
	if !ok {
		t.Fatalf("task summary must run in process: %s", why)
	}
}

func TestNcSummaryUsageLine(t *testing.T) {
	want := "grid task summary <task> --for owner|developer|customer --text <words> [--json]"
	if !strings.Contains(taskUsage, want) {
		t.Fatalf("taskUsage lacks %q", want)
	}
}
