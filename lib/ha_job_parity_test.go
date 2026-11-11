package lib

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The worker's Job and intent calls (app internal/kernel/worker: agentbuild.go,
// supervisor.go, intents.go) run through lib.Run and through the built binary. Each
// test compares exit code, stdout and stderr of the two paths.

// haJbEnviron is the one environment of every call in this file, lib and binary alike.
func haJbEnviron(h home, extra ...string) []string {
	env := h.environ("GRID_HOME="+h.grid,
		"CLAUDE_CONFIG_DIR="+filepath.Join(h.root, "claude"),
		"CODEX_HOME="+filepath.Join(h.root, "codex"),
		"PATH="+os.Getenv("PATH"))
	return append(env, extra...)
}

type haJbResult struct {
	code int
	out  string
	err  string
}

// haJbCaller makes one call in dir and returns what a person would see.
type haJbCaller func(h home, dir string, args ...string) haJbResult

func haJbLib(h home, dir string, args ...string) haJbResult {
	code, out, errs := h.run(dir, haJbEnviron(h), args...)
	return haJbResult{code, out, errs}
}

func haJbBinary(bin string) haJbCaller {
	return func(h home, dir string, args ...string) haJbResult {
		code, out, errs := binary(bin, dir, haJbEnviron(h), args...)
		return haJbResult{code, out, errs}
	}
}

var (
	haJbULID = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)
	haJbTime = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})`)
)

// haJbNorm replaces the home's root path, the given secrets, times and ULIDs with fixed
// tokens, so the output of two homes can be compared.
func haJbNorm(s string, h home, secrets ...string) string {
	s = strings.ReplaceAll(s, h.root, "<ROOT>")
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "<SECRET>")
		}
	}
	s = haJbTime.ReplaceAllString(s, "<TIME>")
	return haJbULID.ReplaceAllString(s, "<ULID>")
}

// haJbTaskID is the id of the home's task (the folder is named after it).
func haJbTaskID(t *testing.T, h home) string {
	t.Helper()
	code, out, errs := h.run(h.task, haJbEnviron(h), "task", "show", "--json")
	if code != 0 {
		t.Fatalf("task show: %d %s", code, errs)
	}
	var got struct{ ID string }
	if err := json.Unmarshal([]byte(out), &got); err != nil || got.ID == "" {
		t.Fatalf("task show output: %v %s", err, out)
	}
	return got.ID
}

// haJbMachine is the machine id of the home, as the plugin's feed tests read it.
func haJbMachine(t *testing.T, h home) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(h.grid, "machine"))
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(b))
}

func haJbSame(t *testing.T, label string, args []string, lib, bin haJbResult) {
	t.Helper()
	if lib.code != bin.code || lib.out != bin.out || lib.err != bin.err {
		t.Errorf("%s %v: lib and binary differ\nlib:    exit %d stdout %q stderr %q\nbinary: exit %d stdout %q stderr %q",
			label, args, lib.code, lib.out, lib.err, bin.code, bin.out, bin.err)
	}
}

// Row 1: the reads the worker keeps the stdout of even when they fail (KeepFailedJSON).
func TestHaJbShowRender(t *testing.T) {
	bin := haJbBinary(buildBinary(t))
	h := newHome(t, "Show and render")
	id := haJbTaskID(t, h)
	for _, args := range [][]string{
		{"job", "show", id, "J-1", "--json"},
		{"job", "render", id, "J-1", "--json"},
		{"job", "show", id, "J-1", "--json"}, // again, after render made the Job's folder
	} {
		lib, bn := haJbLib(h, h.task, args...), bin(h, h.task, args...)
		t.Logf("%v: exit %d stdout %.160q stderr %q", args, lib.code, lib.out, lib.err)
		haJbSame(t, "read", args, lib, bn)
	}
}

// haJbIntentFlow is what the worker does with one intent (intents.go): a task move
// makes it, the worker lists the pending ones, claims it, and claims it again.
type haJbFlow struct {
	h        home
	id       string
	intentID string
	token    string
	machine  string
	steps    []haJbResult
	call     haJbCaller
}

func haJbClaim(t *testing.T, f *haJbFlow) {
	t.Helper()
	f.id = haJbTaskID(t, f.h)
	f.machine = haJbMachine(t, f.h)
	step := func(args ...string) haJbResult {
		r := f.call(f.h, f.h.task, args...)
		f.steps = append(f.steps, r)
		return r
	}
	if r := step("task", "move", f.id, "--to", "other-machine", "--json"); r.code != 0 {
		t.Fatalf("task move: %d %s", r.code, r.err)
	}
	r := step("intent", "list", "--pending", "--task", f.id, "--json")
	var rows []struct{ ID string }
	if err := json.Unmarshal([]byte(r.out), &rows); r.code != 0 || err != nil || len(rows) != 1 {
		t.Fatalf("intent list: %d %v %s %s", r.code, err, r.out, r.err)
	}
	f.intentID = rows[0].ID
	claim := []string{"intent", "claim", f.intentID, "--who", "member@" + f.machine, "--task", f.id, "--json"}
	r = step(claim...)
	var got struct{ Token string }
	if err := json.Unmarshal([]byte(r.out), &got); r.code != 0 || err != nil || got.Token == "" {
		t.Fatalf("intent claim: %d %v %s %s", r.code, err, r.out, r.err)
	}
	f.token = got.Token
	if r = step(claim...); r.code != 5 {
		t.Fatalf("second claim: exit %d, want 5: %s %s", r.code, r.out, r.err)
	}
}

func haJbFlowCompare(t *testing.T, lib, bin *haJbFlow, want []int) {
	t.Helper()
	if len(lib.steps) != len(bin.steps) || len(lib.steps) != len(want) {
		t.Fatalf("steps: lib %d binary %d want %d", len(lib.steps), len(bin.steps), len(want))
	}
	for i := range lib.steps {
		l, b := lib.steps[i], bin.steps[i]
		if l.code != want[i] || b.code != want[i] {
			t.Errorf("step %d: exit lib %d binary %d, want %d\nlib: %q %q\nbinary: %q %q", i, l.code, b.code, want[i], l.out, l.err, b.out, b.err)
		}
		lo, le := haJbNorm(l.out, lib.h, lib.token), haJbNorm(l.err, lib.h, lib.token)
		bo, be := haJbNorm(b.out, bin.h, bin.token), haJbNorm(b.err, bin.h, bin.token)
		if lo != bo || le != be {
			t.Errorf("step %d: normalised output differs\nlib:    %q %q\nbinary: %q %q", i, lo, le, bo, be)
		}
	}
}

func haJbTwins(t *testing.T, title string) (lib, bin *haJbFlow) {
	t.Helper()
	b := haJbBinary(buildBinary(t))
	lib = &haJbFlow{h: newHome(t, title), call: haJbLib}
	bin = &haJbFlow{h: newHome(t, title), call: b}
	return lib, bin
}

// Row 2: task move, intent list --pending, intent claim, and the claim again (exit 5).
// The claim argv and `--who member@<machine>` are the worker's (intents.go); the machine
// is read from the home's machine file as TestFeedPost... in internal/cli/feed_test.go does.
func TestHaJbIntentClaim(t *testing.T) {
	lib, bin := haJbTwins(t, "Intent claim")
	haJbClaim(t, lib)
	haJbClaim(t, bin)
	haJbFlowCompare(t, lib, bin, []int{0, 0, 0, 5})
	// The claim the binary wrote is read back through lib, and the other way round.
	// Both reads run on the same home, so the record is byte-equal as written.
	readBin := haJbBinary(buildBinary(t))
	for _, p := range []struct {
		name string
		f    *haJbFlow
	}{{"lib-written", lib}, {"binary-written", bin}} {
		args := []string{"intent", "list", "--task", p.f.id, "--json"}
		l := haJbLib(p.f.h, p.f.h.task, args...)
		b := readBin(p.f.h, p.f.h.task, args...)
		haJbSame(t, p.name, args, l, b)
		if !strings.Contains(l.out, p.f.token) {
			t.Errorf("%s: the claim token is not in the list: %s", p.name, l.out)
		}
	}
}

// Row 3: renew, done, and done again with the claim's token. The brief expects 0, 0, 5;
// the binary answers 0, 0, 0 for the same-token repeat, because Store.Done returns the
// stored completion when token and error are the same (internal/intent/intent.go:430-435).
// Exit 5 (ErrDone) is a done with a different error after the intent is done, so a
// fourth step is that call. Both paths are held to the same four codes.
func TestHaJbIntentDoneTwice(t *testing.T) {
	lib, bin := haJbTwins(t, "Intent done")
	for _, f := range []*haJbFlow{lib, bin} {
		haJbClaim(t, f)
		f.steps = f.steps[:0]
		for _, extra := range [][]string{{}, {}, {}, {"--error"}} {
			verb := "done"
			if len(f.steps) == 0 {
				verb = "renew"
			}
			args := append([]string{"intent", verb, f.intentID, "--token", f.token, "--task", f.id, "--json"}, extra...)
			f.steps = append(f.steps, f.call(f.h, f.h.task, args...))
		}
	}
	haJbFlowCompare(t, lib, bin, []int{0, 0, 0, 5})
	// Each home's finished intent is read through the other path, byte-equal.
	for _, f := range []*haJbFlow{lib, bin} {
		args := []string{"intent", "list", "--task", f.id, "--json"}
		haJbSame(t, "read back", args, haJbLib(f.h, f.h.task, args...), haJbBinary(buildBinary(t))(f.h, f.h.task, args...))
	}
}

// Row 4: a post from a stale lease is exit 3 with the same stderr on both paths. The argv
// is supervisor.go:100 (send) with --lease-epoch 9.9; the body is a worker-exit.
func TestHaJbStalePost(t *testing.T) {
	bin := haJbBinary(buildBinary(t))
	hl, hb := newHome(t, "Stale post"), newHome(t, "Stale post")
	args := []string{"job", "post", "--type", "worker-exit", "--id", "exit-1", "--job", "J-1", "--attempt", "1",
		"--lease", "J-1", "--lease-epoch", "9.9", "--body", `{"reason":"cancelled"}`, "--json"}
	l, b := haJbLib(hl, hl.task, args...), bin(hb, hb.task, args...)
	if l.code != 3 || b.code != 3 {
		t.Fatalf("exit lib %d binary %d, want 3\nlib: %q %q\nbinary: %q %q", l.code, b.code, l.out, l.err, b.out, b.err)
	}
	if haJbNorm(l.err, hl) != haJbNorm(b.err, hb) || l.err != b.err {
		t.Errorf("stderr differs\nlib:    %q\nbinary: %q", l.err, b.err)
	}
	if haJbNorm(l.out, hl) != haJbNorm(b.out, hb) {
		t.Errorf("stdout differs\nlib:    %q\nbinary: %q", l.out, b.out)
	}
}

// Row 5: usage post with the supervisor.go:55 argv,
//
//	args := []string{"usage", "post", "--task", j.Task, "--job", j.ID, "--result", result, "--json"}
//
// on an attempt log that holds a result line.
func TestHaJbUsagePost(t *testing.T) {
	bin := haJbBinary(buildBinary(t))
	line := `{"type":"result","session_id":"session-1","total_cost_usd":0.25,"duration_ms":90000,"usage":{"input_tokens":10,"output_tokens":5}}` + "\n"
	run := func(h home, call haJbCaller) []haJbResult {
		logPath := filepath.Join(h.root, "attempt.log")
		if err := os.WriteFile(logPath, []byte(line), 0o644); err != nil {
			t.Fatal(err)
		}
		args := []string{"usage", "post", "--task", haJbTaskID(t, h), "--job", "J-1", "--result", logPath, "--json"}
		// The second call books nothing new: the CLI keys the ledger on the session.
		return []haJbResult{call(h, h.task, args...), call(h, h.task, args...)}
	}
	hl, hb := newHome(t, "Usage post"), newHome(t, "Usage post")
	l, b := run(hl, haJbLib), run(hb, bin)
	for i := range l {
		if l[i].code != 0 || b[i].code != 0 {
			t.Errorf("call %d: exit lib %d binary %d, want 0\nlib: %q\nbinary: %q", i, l[i].code, b[i].code, l[i].err, b[i].err)
		}
		if haJbNorm(l[i].out, hl) != haJbNorm(b[i].out, hb) || haJbNorm(l[i].err, hl) != haJbNorm(b[i].err, hb) {
			t.Errorf("call %d: normalised output differs\nlib:    %q %q\nbinary: %q %q", i, l[i].out, l[i].err, b[i].out, b[i].err)
		}
	}
	// The ledger the binary wrote is read through lib, and the other way round.
	for _, h := range []home{hl, hb} {
		args := []string{"usage", "show", "--task", haJbTaskID(t, h), "--json"}
		haJbSame(t, "usage show", args, haJbLib(h, h.task, args...), bin(h, h.task, args...))
	}
}
