package agent

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// iqAcHangCtx ends at the test's deadline less a tenth of the time left, and
// at most one minute from now, so a test that hangs fails with its own message
// before go test kills the binary. It has no deadline when the test has none.
func iqAcHangCtx(t *testing.T) context.Context {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		return context.Background()
	}
	left := time.Until(deadline)
	end := deadline.Add(-left / 10)
	if limit := time.Now().Add(time.Minute); end.After(limit) {
		end = limit
	}
	ctx, cancel := context.WithDeadline(context.Background(), end)
	t.Cleanup(cancel)
	return ctx
}

// iqAcProbeEnd swaps probeContext for the test. Each probe keeps the default
// budget, so its deadline can still be checked, and also ends when the
// returned function is called: the event a real budget would give. A probe
// that starts after the call ends at once. The test restores probeContext in
// t.Cleanup; it must not run in parallel.
func iqAcProbeEnd(t *testing.T) func() {
	t.Helper()
	orig := probeContext
	ended, end := context.WithCancel(context.Background())
	probeContext = func(ctx context.Context) (context.Context, context.CancelFunc) {
		budget, stopBudget := orig(ctx)
		child, cancel := context.WithCancel(budget)
		stop := context.AfterFunc(ended, cancel)
		if ended.Err() != nil {
			cancel() // AfterFunc would cancel on its own goroutine, later.
		}
		return child, func() { stop(); cancel(); stopBudget() }
	}
	t.Cleanup(func() {
		end()
		probeContext = orig
	})
	return end
}

// iqAcRunCheck runs Check in a goroutine and sends its reports on the returned
// channel. Its cleanup cancels the check and waits for it to return, so a test
// that fails early leaves no probe behind to read probeContext after the
// cleanup of iqAcProbeEnd (which runs after this one) has restored it.
func iqAcRunCheck(t *testing.T, env Env) <-chan []Report {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []Report, 1)
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		done <- Check(ctx, env)
	}()
	t.Cleanup(func() {
		cancel()
		<-exited
	})
	return done
}

// The default probeContext gives a probe the Wait budget, checked by order
// only: the deadline is no earlier than the budget started after `before`, and
// no later than the budget started now.
func TestIqAcDefaultBudget(t *testing.T) {
	before := time.Now()
	child, cancel := probeContext(context.Background())
	defer cancel()
	deadline, ok := child.Deadline()
	if !ok || deadline.Before(before.Add(Wait)) || deadline.After(time.Now().Add(Wait)) {
		t.Fatalf("default probe budget: deadline %v, %v", deadline, ok)
	}
}

// A probe that starts late (the Codex runner is held until the Claude probe has
// run) still has a deadline in order, and Check still returns both reports.
func TestIqAcLateEntry(t *testing.T) {
	hang := iqAcHangCtx(t)
	before := time.Now()
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseCodex := func() { releaseOnce.Do(func() { close(release) }) }
	t.Cleanup(releaseCodex)
	claudeEntered := make(chan struct{})
	var enteredOnce sync.Once
	env := fakeEnv(t, func(ctx context.Context, argv []string) (string, int) {
		deadline, ok := ctx.Deadline()
		if !ok || deadline.Before(before.Add(Wait)) || deadline.After(time.Now().Add(Wait)) {
			t.Errorf("wrong probe deadline: %v, %v", deadline, ok)
		}
		name := filepath.Base(argv[0])
		if name == "codex" {
			select {
			case <-release:
			case <-ctx.Done():
			}
		} else {
			enteredOnce.Do(func() { close(claudeEntered) })
		}
		if argv[1] == "--version" {
			if name == "claude" {
				return "2.1.236 (Claude Code)\n", 0
			}
			return "codex-cli v0.162.0\n", 0
		}
		return `{"authMethod":"oauth"}`, 0
	})
	done := iqAcRunCheck(t, env)
	select {
	case <-claudeEntered:
	case <-hang.Done():
		t.Fatal("claude probe did not run while codex was held")
	}
	if hang.Err() != nil {
		t.Fatalf("hang context ended before codex was released: %v", hang.Err())
	}
	releaseCodex()
	select {
	case reports := <-done:
		for i, want := range []struct{ agent, version string }{{"claude", "2.1.236"}, {"codex", "0.162.0"}} {
			r := reports[i]
			if r.Agent != want.agent || !r.Found || r.Version != want.version || r.SignedIn == nil || !*r.SignedIn || r.Fix != "" {
				t.Errorf("report %d: %+v", i, r)
			}
		}
	case <-hang.Done():
		t.Fatal("check did not return after codex was released")
	}
}
