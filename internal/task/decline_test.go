package task

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gridconsole/grid-plugins/internal/contract"
)

func TestDeclinePolicy(t *testing.T) {
	for _, role := range []string{"", "owner", "member", "guest", "worker"} {
		want := role == "" || role == "owner" || role == "member"
		if DeclineAllowed(role) != want {
			t.Fatalf("DeclineAllowed(%q) != %t", role, want)
		}
	}
	original := DeclineAllowed
	t.Cleanup(func() { DeclineAllowed = original })
	DeclineAllowed = func(role string) bool { return role == "owner" }
	s, task := writeFixture(t)
	_, err := s.Decline(DeclineOptions{WriteOptions: WriteOptions{ID: task.ID}, Reason: "Later", Role: "member"})
	if err == nil || !strings.Contains(err.Error(), "scope") {
		t.Fatalf("custom policy: %v", err)
	}
	if _, err := os.Stat(filepath.Join(task.Path, ".task.lock")); !os.IsNotExist(err) {
		t.Fatalf("policy must run before lock: %v", err)
	}
}

func TestDeclineValidation(t *testing.T) {
	for _, reason := range []string{"", " \t\n ", "bad\x00reason"} {
		_, err := (Store{}).Decline(DeclineOptions{Reason: reason})
		if !errors.Is(err, ErrEditUsage) {
			t.Fatalf("reason %q: %v", reason, err)
		}
	}
}

func TestDeclinePlannerAndWho(t *testing.T) {
	for _, tc := range []struct {
		kind, who string
		rejected  bool
	}{
		{kind: "planner", rejected: true},
		{kind: "alias", rejected: true},
		{kind: "builder", who: "alex"},
		{who: ""},
	} {
		t.Run(tc.kind+tc.who, func(t *testing.T) {
			s, task := writeFixture(t)
			path := filepath.Join(task.Path, "task.md")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			text := strings.Replace(string(data), "state: queued", "kind: '"+tc.kind+"'\nstate: queued\nnext: 'Keep going'", 1)
			if tc.kind == "alias" {
				text = strings.Replace(text, "kind: 'alias'", "planner_kind: &planner planner\nkind: *planner", 1)
			}
			write(t, path, text)
			_, err = s.Decline(DeclineOptions{WriteOptions: WriteOptions{ID: task.ID, Who: contract.Author{ID: tc.who}}, Reason: "Later"})
			if tc.rejected {
				if !errors.Is(err, ErrEditRejected) {
					t.Fatalf("planner: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err = os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.who
			if want == "" {
				want = "you"
			}
			if !strings.Contains(string(data), "declined by "+want) || strings.Contains(string(data), "Keep going") {
				t.Fatalf("next = %s", data)
			}
		})
	}
}
