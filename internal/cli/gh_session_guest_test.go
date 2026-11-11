package cli

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	contractdata "github.com/gridconsole/grid-plugins/contract"
	"github.com/gridconsole/grid-plugins/internal/intent"
	"github.com/gridconsole/grid-plugins/internal/project"
	"github.com/gridconsole/grid-plugins/internal/session"
)

// The session subcommands a guest may still run (reads) and the seven it may not
// (writes: each one queues an intent through runIntentWrite).
var (
	ghSessionReads  = []string{"list", "show"}
	ghSessionWrites = []string{"start", "stop", "send", "pause", "resume", "handback", "switch"}
)

// ghSessionCase is one write command with the intent it queues; the same args as
// TestSessionIntentCommands.
type ghSessionCase struct {
	args                     []string
	verb, target, key, value string
}

var ghSessionCases = []ghSessionCase{
	{[]string{"start"}, "start", "T-01-example", "", ""},
	{[]string{"start", "--agent", "planner"}, "start", "T-01-example", "agent", "planner"},
	{[]string{"stop", "--target", "session-example"}, "stop", "session-example", "", ""},
	{[]string{"send", "a message with spaces"}, "send", "T-01-example", "text", "a message with spaces"},
	{[]string{"send", "--text", "--literal text"}, "send", "T-01-example", "text", "--literal text"},
	{[]string{"pause"}, "pause", "T-01-example", "", ""},
	{[]string{"resume"}, "resume", "T-01-example", "", ""},
	{[]string{"handback"}, "handback", "T-01-example", "", ""},
	{[]string{"switch", "builder"}, "switch", "T-01-example", "agent", "builder"},
}

// ghSession is a harness with one task folder in a registered project that
// invites the guest "gus", and the given role and lease in its environment; no
// other variable is set. The guest sees the task, so a refusal is the role's.
func ghSession(t *testing.T, role, lease string) (*harness, string) {
	t.Helper()
	h := newHarness(t)
	task := livenessProject(h)
	h.record(project.Record{Name: "example", ID: "EXAMPLE"})
	h.write(filepath.Join(h.gridHome, "projects", "example", "project-roles.json"), `{"people":[{"id":"gus","perm":"Client"}]}`)
	ghSessionEnv(h, role, lease)
	return h, task
}

func ghSessionEnv(h *harness, role, lease string) {
	h.env.Getenv = func(k string) string {
		switch k {
		case "GRID_ROLE":
			return role
		case "GRID_LEASE":
			return lease
		case "GRID_MEMBER":
			return "gus"
		}
		return ""
	}
}

// ghSessionContract reads the session subcommands from the contract inventory and
// fails on one this file does not classify as a read or a write.
func ghSessionContract(t *testing.T) (reads, writes []string) {
	t.Helper()
	var doc contractCommandDocument
	if err := json.Unmarshal(contractdata.Commands, &doc); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range doc.Commands {
		if c.Name != "session" {
			continue
		}
		found = true
		for _, sub := range c.Subcommands {
			switch {
			case slices.Contains(ghSessionReads, sub.Name):
				reads = append(reads, sub.Name)
			case slices.Contains(ghSessionWrites, sub.Name):
				writes = append(writes, sub.Name)
			default:
				t.Errorf("session %s is neither a classified read nor a classified write: classify it here and in guestSessionWrite", sub.Name)
			}
		}
	}
	if !found {
		t.Fatal("the contract inventory has no session command")
	}
	for _, want := range ghSessionWrites {
		if !slices.Contains(writes, want) {
			t.Errorf("the contract inventory lacks session %s", want)
		}
	}
	return reads, writes
}

type ghSessionAnswer struct {
	code        int
	out, errOut string
}

func ghSessionRun(h *harness, cwd string, args ...string) ghSessionAnswer {
	code := h.run(cwd, args...)
	return ghSessionAnswer{code, h.out.String(), h.errb.String()}
}

func ghSessionRefusal(verb string) string {
	return "grid session " + verb + ": role \"guest\" may not " + verb + " a session; a guest answers and comments\n"
}

func TestGhSessionGuest(t *testing.T) {
	reads, writes := ghSessionContract(t)

	// Every write subcommand of the contract is refused for a guest, leaves the
	// task folder as it was and queues no intent.
	t.Run("walk", func(t *testing.T) {
		for _, verb := range writes {
			t.Run(verb, func(t *testing.T) {
				if !guestWriteVerb("session", []string{verb}) {
					t.Fatalf("guestWriteVerb does not name session %s", verb)
				}
				var args []string
				for _, tc := range ghSessionCases {
					if tc.verb == verb {
						args = tc.args
						break
					}
				}
				if args == nil {
					t.Fatalf("no args for session %s", verb)
				}
				h, task := ghSession(t, "guest", "")
				before := guestSnapshot(t, task)
				got := ghSessionRun(h, task, append(append([]string{"session"}, args...), "--who", "member")...)
				after := guestSnapshot(t, task)
				if got.code != 1 || got.out != "" || got.errOut != ghSessionRefusal(verb) {
					t.Fatalf("guest session %s: %+v", verb, got)
				}
				if after != before {
					t.Fatalf("guest session %s changed the task folder:\n%s\n---\n%s", verb, before, after)
				}
				rows, err := (intent.Store{TaskDir: task}).List(false)
				if err != nil || len(rows) != 0 {
					t.Fatalf("guest session %s queued an intent: %v %v", verb, rows, err)
				}
			})
		}
		for _, verb := range reads {
			if guestWriteVerb("session", []string{verb}) {
				t.Errorf("guestWriteVerb names the read session %s", verb)
			}
		}
	})

	// The refusal comes before any task is resolved: a task the guest cannot see
	// gets the same answer as one it can.
	t.Run("early", func(t *testing.T) {
		for _, verb := range writes {
			t.Run(verb, func(t *testing.T) {
				var args []string
				for _, tc := range ghSessionCases {
					if tc.verb == verb {
						args = tc.args
						break
					}
				}
				h, task := ghSession(t, "guest", "")
				visible := ghSessionRun(h, h.home, append(append([]string{"session"}, args...), "--task", task)...)
				unknown := ghSessionRun(h, h.home, append(append([]string{"session"}, args...), "--task", "T-999")...)
				if visible != unknown {
					t.Fatalf("session %s: visible %+v, unknown %+v", verb, visible, unknown)
				}
				if want := (ghSessionAnswer{1, "", ghSessionRefusal(verb)}); unknown != want {
					t.Fatalf("session %s: %+v, want %+v", verb, unknown, want)
				}
			})
		}
	})

	// An owner, a member and an unset role (the default member) keep every write.
	t.Run("roles", func(t *testing.T) {
		for _, role := range []string{"owner", "member", ""} {
			for _, tc := range ghSessionCases {
				t.Run(role+"/"+strings.Join(tc.args, " "), func(t *testing.T) {
					h, task := ghSession(t, role, "")
					ghSessionQueues(t, h, task, tc)
				})
			}
		}
	})

	// A guest holding the task lease is an agent: that path is as it was.
	t.Run("lease", func(t *testing.T) {
		for _, tc := range ghSessionCases {
			t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
				h, task := ghSession(t, "guest", "task")
				ghSessionQueues(t, h, task, tc)
			})
		}
	})

	// list and show read: a guest gets what a member gets.
	t.Run("reads", func(t *testing.T) {
		h, task := ghSession(t, "member", "")
		record := session.Record{Key: session.Key{SessionID: "session-1", MachineID: "machine-1"}, PID: 42, PIDStart: "start-1"}
		if _, err := (session.Store{TaskDir: task}).Start(record); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			args []string
			ok   bool
		}{
			{[]string{"session", "list", "--json"}, true},
			{[]string{"session", "list"}, true},
			{[]string{"session", "show", "session-1"}, true},
			{[]string{"session", "show", "session-none"}, false},
		} {
			t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
				ghSessionEnv(h, "member", "")
				member := ghSessionRun(h, task, tc.args...)
				ghSessionEnv(h, "guest", "")
				guest := ghSessionRun(h, task, tc.args...)
				if guest != member {
					t.Fatalf("guest %+v, member %+v", guest, member)
				}
				if tc.ok && member.code != 0 {
					t.Fatalf("member read failed: %+v", member)
				}
				if guest.errOut != "" && strings.Contains(guest.errOut, "role") {
					t.Fatalf("guest read refused on role: %+v", guest)
				}
			})
		}
	})
}

// ghSessionQueues runs one write command and checks the intent it queues.
func ghSessionQueues(t *testing.T, h *harness, task string, tc ghSessionCase) {
	t.Helper()
	args := append([]string{"session"}, tc.args...)
	args = append(args, "--who", "member", "--json")
	if code := h.run(task, args...); code != 0 {
		t.Fatalf("exit %d: %s", code, h.errb)
	}
	var r intent.Record
	if err := json.Unmarshal(h.out.Bytes(), &r); err != nil {
		t.Fatal(err)
	}
	if r.Verb != tc.verb || r.Target != tc.target || r.Who != "member" || r.Args[tc.key] != tc.value {
		t.Fatalf("record: %+v", r)
	}
	rows, err := (intent.Store{TaskDir: task}).List(false)
	if err != nil || len(rows) != 1 || rows[0].ID != r.ID {
		t.Fatalf("queued: %v %v", rows, err)
	}
}
