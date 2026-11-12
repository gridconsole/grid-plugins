package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/brain"
	"github.com/gridconsole/grid-plugins/internal/fslock"
	"github.com/gridconsole/grid-plugins/internal/projectenv"
)

// bwEnvHangCtx ends at t.Deadline() less one tenth of the time left, at most one
// minute, so a hung wait fails from the test before the go test timeout panic.
// Without a test deadline it never ends. It is a wait's only deadline.
func bwEnvHangCtx(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	if deadline, ok := t.Deadline(); ok {
		margin := time.Until(deadline) / 10
		if margin > time.Minute {
			margin = time.Minute
		}
		ctx, cancel = context.WithDeadline(context.Background(), deadline.Add(-margin))
	}
	t.Cleanup(cancel)
	return ctx
}

// bwEnvResult is what one command wrote, kept apart from h.out and h.errb, which
// a goroutine must not share.
type bwEnvResult struct {
	code        int
	out, errOut string
}

// bwEnvRun runs grid in h's home on a copy of h.env with buffers of its own.
func bwEnvRun(h *harness, args ...string) bwEnvResult {
	env := h.env
	var out, errb bytes.Buffer
	env.Stdout, env.Stderr, env.Cwd = &out, &errb, h.tmp
	code := Run(args, env)
	return bwEnvResult{code, out.String(), errb.String()}
}

// bwEnvStore is project alpha's brain in h's home.
func bwEnvStore(h *harness) brain.Store {
	return brain.Store{GridHome: h.gridHome, Project: "alpha"}
}

// bwEnvSeed stores the entry dev with a name only, under the owner's harness.
func bwEnvSeed(t *testing.T, h *harness) {
	t.Helper()
	if err := projectenv.Save(bwEnvStore(h), projectenv.Entry{ID: "dev", Name: "Seed"}); err != nil {
		t.Fatal(err)
	}
}

func bwEnvStored(t *testing.T, h *harness) projectenv.Entry {
	t.Helper()
	b, err := bwEnvStore(h).ReadConfig("project", "environments", "dev")
	if err != nil {
		t.Fatal(err)
	}
	e, err := projectenv.Parse("dev", b)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// bwEnvLockPath is project alpha's brain lock file.
func bwEnvLockPath(h *harness) string {
	return filepath.Join(h.gridHome, "projects", "alpha", ".brain.lock")
}

// bwEnvIsLock says whether a block event is on alpha's brain lock. The file is
// opened through a root, so only the last element and the project folder are
// compared.
func bwEnvIsLock(path string) bool {
	return filepath.Base(path) == ".brain.lock" && strings.Contains(filepath.ToSlash(path), "/projects/alpha/")
}

// Two edits of dev, each with one flag, both queue on the held brain lock. When it
// is released, they run one after the other and the entry holds both flags. When
// the entry is read before the lock, both edits start from the seed and the second
// write drops the first one's field.
func TestBwEnvCliHeldLockKeepsBoth(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	bwEnvSeed(t, h)

	f, err := fslock.OpenNoFollow(bwEnvLockPath(h), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if err := fslock.LockExclusive(f); err != nil {
		f.Close()
		t.Fatal(err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			fslock.Unlock(f)
			f.Close()
		})
	}
	t.Cleanup(release)

	queued := make(chan struct{}, 2)
	var waits atomic.Int32
	t.Cleanup(fslock.OnBlockForTest(func(path string) {
		if !bwEnvIsLock(path) {
			return
		}
		waits.Add(1)
		select {
		case queued <- struct{}{}:
		default:
		}
	}))

	edits := [][]string{
		{"project", "env", "edit", "alpha", "--id", "dev", "--name", "Builds", "--json"},
		{"project", "env", "edit", "alpha", "--id", "dev", "--how", "Run in CI", "--json"},
	}
	done := make(chan bwEnvResult, len(edits))
	for _, args := range edits {
		go func() { done <- bwEnvRun(h, args...) }()
	}

	hang := bwEnvHangCtx(t)
	for range edits {
		select {
		case r := <-done:
			t.Fatalf("an edit finished with exit %d while the brain lock was held, and with no block event: %s", r.code, r.errOut)
		case <-queued:
		case <-hang.Done():
			t.Fatalf("only %d of 2 edits queued on the brain lock", waits.Load())
		}
	}
	release()
	for range edits {
		select {
		case r := <-done:
			if r.code != 0 {
				t.Fatalf("an edit exited %d: %s", r.code, r.errOut)
			}
		case <-hang.Done():
			t.Fatal("the edits did not finish after the brain lock was released")
		}
	}
	if got := bwEnvStored(t, h); got.Name != "Builds" || got.How != "Run in CI" {
		t.Fatalf("stored entry = %+v, want Name Builds and How Run in CI", got)
	}
}

// Seven edits at once, one flag each: every exit is 0 and every value is stored.
func TestBwEnvCliEditsKeepEveryField(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	bwEnvSeed(t, h)
	flags := [][2]string{{"--name", "Builds"}, {"--kind", "cloud"}, {"--where", "the office"}, {"--runs", "app"},
		{"--test", "make test"}, {"--how", "Run in CI"}, {"--from", "template"}}
	results := make([]bwEnvResult, len(flags))
	var wg sync.WaitGroup
	for i, fv := range flags {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = bwEnvRun(h, "project", "env", "edit", "alpha", "--id", "dev", fv[0], fv[1], "--json")
		}()
	}
	wg.Wait()
	for i, r := range results {
		if r.code != 0 {
			t.Fatalf("edit %s exited %d: %s", flags[i][0], r.code, r.errOut)
		}
	}
	want := projectenv.Entry{ID: "dev", Name: "Builds", Kind: "cloud", Where: "the office", Runs: "app", Test: "make test",
		How: "Run in CI", From: "template"}
	if got := bwEnvStored(t, h); got.Name != want.Name || got.Kind != want.Kind || got.Where != want.Where ||
		got.Runs != want.Runs || got.Test != want.Test || got.How != want.How || got.From != want.From {
		t.Fatalf("stored entry = %+v, want all seven values %+v", got, want)
	}
}

// Eight adds of one id at once: one wins, seven are told to use edit, and the
// stored name is the winner's.
func TestBwEnvCliAddOnce(t *testing.T) {
	h := lnEnvCliHarness(t, "owner", "bo")
	if err := os.MkdirAll(filepath.Join(h.gridHome, "projects", "alpha"), 0o700); err != nil {
		t.Fatal(err)
	}
	const n = 8
	results := make([]bwEnvResult, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = bwEnvRun(h, "project", "env", "add", "alpha", "--id", "dev", "--name", "N"+string(rune('0'+i)), "--kind", "local", "--json")
		}()
	}
	wg.Wait()
	winner, wins := -1, 0
	for i, r := range results {
		if r.code == 0 {
			winner, wins = i, wins+1
			continue
		}
		if !strings.Contains(r.errOut, "environment dev already exists; use edit") {
			t.Fatalf("add %d exited %d with %q, want the already exists text", i, r.code, r.errOut)
		}
	}
	if wins != 1 {
		t.Fatalf("%d adds exited 0, want exactly one", wins)
	}
	if got := bwEnvStored(t, h); got.Name != "N"+string(rune('0'+winner)) {
		t.Fatalf("stored name = %q, want the winner's N%d", got.Name, winner)
	}
}
