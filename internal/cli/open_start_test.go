package cli

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/fslock"
)

// workerStatusJSON is worker-status.json as the app's grid-worker writes it.
const workerStatusJSON = `{"pid":4242,"version":"0.0.0-test","started_at":"2026-11-08T09:00:00Z","uptime_seconds":10,"address":"127.0.0.1:43111","token":"not-for-the-list"}`

// startFixture is an open fixture plus a folder of fake grid-server and grid-worker
// scripts. Each start appends a line to <markers>/<name>.started, which the fake worker
// below waits for; the tests count starts from the run's stderr (koSoStarted). A worker that runs
// holds an empty worker.lock with an OS lock and has written worker-status.json: the
// test process plays that part (a script cannot hold the lock), starting once the fake
// grid-worker has been run. The fake server writes app-url with the address of a
// listener the test holds.
type startFixture struct {
	*harness
	project, id string
	bin         string // folder of fake programs
	markers     string
	addr        string // where the fake server "listens"
}

func newStartFixture(t *testing.T) *startFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake programs are shell scripts")
	}
	h, project, id := openFixture(t, true)
	f := &startFixture{harness: h, project: project, id: id,
		bin: t.TempDir(), markers: t.TempDir()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f.addr = ln.Addr().String()
	t.Setenv("FAKE_MARKERS", f.markers)
	t.Setenv("FAKE_ADDR", f.addr)

	h.write(filepath.Join(h.gridHome, "machine"), "01BX5ZZKBKACTAV9WEVGEMMVRZ\n")
	f.program("grid-worker", "exit 0\n")
	// The worker the fake program stands for: once it has been run, the status file
	// is there and the lock is held until the test ends.
	stop, done := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(stop); <-done })
	go func() {
		defer close(done)
		marker := filepath.Join(f.markers, "grid-worker.started")
		for {
			if _, err := os.Stat(marker); err == nil {
				break
			}
			select {
			case <-stop:
				return
			case <-time.After(5 * time.Millisecond):
			}
		}
		lock, err := f.holdWorkerFiles()
		if err != nil {
			t.Errorf("fake worker: %v", err)
			return
		}
		<-stop
		lock.Close()
	}()
	// The sleep keeps the first start in flight while a second grid open arrives.
	f.program("grid-server", `sleep 0.3
printf 'http://%s\n' "$FAKE_ADDR" > "$GRID_HOME/app-url"
`)
	h.env.AppStart = &AppStart{Dirs: []string{f.bin}, Wait: 10 * time.Second}
	return f
}

// program writes a fake program that records its start, then runs body.
func (f *startFixture) program(name, body string) {
	f.t.Helper()
	script := "#!/bin/sh\necho started >> \"$FAKE_MARKERS/" + name + ".started\"\n" + body
	path := filepath.Join(f.bin, name)
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		f.t.Fatal(err)
	}
}

// running makes the worker and the server look as they do when the app is up.
func (f *startFixture) running() {
	f.t.Helper()
	f.write(filepath.Join(f.gridHome, AppURLFile), "http://"+f.addr+"\n")
	lock, err := f.holdWorkerFiles()
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { lock.Close() })
}

// holdWorkerFiles makes the Grid home look as it does while the app's worker runs: an
// empty worker.lock held with an OS lock (the caller closes the file to let go) and
// worker-status.json.
func (f *startFixture) holdWorkerFiles() (*os.File, error) {
	lock, err := os.OpenFile(filepath.Join(f.gridHome, "worker.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := fslock.LockExclusive(lock); err != nil {
		lock.Close()
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(f.gridHome, "worker-status.json"), []byte(workerStatusJSON), 0o600); err != nil {
		lock.Close()
		return nil, err
	}
	return lock, nil
}

// koSoStarted counts the lines of a grid open stderr that say the named program was
// started: "grid open: started grid-worker and grid-server." names both. Open prints
// the line after it has started every program it starts and before it returns, so no
// wait is needed once the run is over.
func koSoStarted(stderr, name string) int {
	n := 0
	for _, line := range strings.Split(stderr, "\n") {
		list, ok := strings.CutPrefix(line, "grid open: started ")
		if !ok {
			continue
		}
		for _, program := range strings.Split(strings.TrimSuffix(list, "."), " and ") {
			if program == name {
				n++
			}
		}
	}
	return n
}

// starts counts how often the stderr of the given grid open runs says it started a
// program. With no stderr given it reads the fixture's own run.
func (f *startFixture) starts(name string, stderr ...string) int {
	if len(stderr) == 0 {
		stderr = []string{f.errb.String()}
	}
	n := 0
	for _, s := range stderr {
		n += koSoStarted(s, name)
	}
	return n
}

// counts reports the worker and server start counts of the given runs' stderr (the
// fixture's own run when none is given).
func (f *startFixture) counts(stderr ...string) (worker, server int) {
	return f.starts("grid-worker", stderr...), f.starts("grid-server", stderr...)
}

func (f *startFixture) want() string {
	return "http://" + f.addr + "/#/t/" + f.project + "/" + f.id + "/overview"
}

func TestOpenStartsWorkerAndServerWhenNotRunning(t *testing.T) {
	f := newStartFixture(t)
	var opened []string
	f.env.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
	if w, s := f.counts(); w != 1 || s != 1 {
		t.Fatalf("starts: worker %d, server %d, want 1 and 1", w, s)
	}
	if len(opened) != 1 || opened[0] != f.want() {
		t.Fatalf("opened %v, want %s", opened, f.want())
	}
	if !strings.Contains(f.errb.String(), "started grid-worker and grid-server") {
		t.Errorf("stderr %q does not say what was started", f.errb)
	}
	if _, err := os.Stat(filepath.Join(f.gridHome, "logs", "grid-server.log")); err != nil {
		t.Errorf("no server log: %v", err)
	}
}

func TestOpenStartsNothingWhenTheAppRuns(t *testing.T) {
	f := newStartFixture(t)
	f.running()
	var opened []string
	f.env.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
	if w, s := f.counts(); w != 0 || s != 0 {
		t.Fatalf("started with the app running: worker %d, server %d", w, s)
	}
	if len(opened) != 1 || opened[0] != f.want() {
		t.Fatalf("opened %v, want %s", opened, f.want())
	}
	if f.errb.Len() != 0 {
		t.Errorf("stderr %q, want nothing", f.errb)
	}
}

func TestOpenParallelStartsEachProgramOnce(t *testing.T) {
	f := newStartFixture(t)
	var mu sync.Mutex
	var opened []string
	var errs [2]bytes.Buffer
	var codes [2]int
	release := make(chan struct{})
	var wg sync.WaitGroup
	for i := range codes {
		e := f.env
		e.Cwd = f.home
		e.Stdout, e.Stderr = &bytes.Buffer{}, &errs[i]
		e.OpenURL = func(u string) error { mu.Lock(); opened = append(opened, u); mu.Unlock(); return nil }
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-release
			codes[i] = runOpen([]string{"T-01"}, e)
		}(i)
	}
	close(release)
	wg.Wait()
	for i, c := range codes {
		if c != 0 {
			t.Fatalf("open %d: exit %d %s", i, c, errs[i].String())
		}
	}
	if w, s := f.counts(errs[0].String(), errs[1].String()); w != 1 || s != 1 {
		t.Fatalf("two opens at once: worker started %d times, server %d, want 1 and 1", w, s)
	}
	if len(opened) != 2 || opened[0] != f.want() || opened[1] != f.want() {
		t.Fatalf("opened %v, want %s twice", opened, f.want())
	}
}

func TestOpenReplacesAStaleAppURL(t *testing.T) {
	f := newStartFixture(t)
	f.running()
	// A server that is gone: the file still holds its address, nothing listens there.
	gone, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := gone.Addr().String()
	gone.Close()
	f.write(filepath.Join(f.gridHome, AppURLFile), "http://"+dead+"\n")
	var opened []string
	f.env.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
	if w, s := f.counts(); s != 1 || w != 0 {
		t.Fatalf("starts: server %d, worker %d, want 1 and 0 (the worker ran)", s, w)
	}
	if len(opened) != 1 || opened[0] != f.want() {
		t.Fatalf("opened %v, want %s", opened, f.want())
	}
}

func TestOpenMissingProgramExitsThreeAndStartsNothing(t *testing.T) {
	f := newStartFixture(t)
	if err := os.Remove(filepath.Join(f.bin, "grid-server")); err != nil {
		t.Fatal(err)
	}
	f.env.OpenURL = func(u string) error { t.Fatalf("opened %s with a program missing", u); return nil }
	if code := f.run(f.home, "open", "T-01"); code != 3 {
		t.Fatalf("exit %d, want 3: %s", code, f.errb)
	}
	line := f.errb.String()
	if strings.Count(line, "\n") != 1 || !strings.Contains(line, "grid-server") || !strings.Contains(line, f.bin) || strings.Contains(line, "grid-worker") {
		t.Errorf("stderr %q: want one line naming grid-server and %s", line, f.bin)
	}
	if f.out.Len() != 0 {
		t.Errorf("stdout %q, want nothing", f.out)
	}
	if w := f.starts("grid-worker"); w != 0 {
		t.Errorf("the worker was started although the server is missing")
	}
}

func TestOpenGridAppURLStartsNothing(t *testing.T) {
	f := newStartFixture(t)
	t.Setenv("GRID_APP_URL", "https://grid.example.test")
	f.env.AppStart = &AppStart{Dirs: []string{t.TempDir()}} // would exit 3 if it tried
	var opened []string
	f.env.OpenURL = func(u string) error { opened = append(opened, u); return nil }
	if code := f.run(f.home, "open", "T-01"); code != 0 {
		t.Fatalf("open: %d %s", code, f.errb)
	}
	if len(opened) != 1 || !strings.HasPrefix(opened[0], "https://grid.example.test/#/t/") {
		t.Fatalf("opened %v", opened)
	}
}

func TestOpenServerThatWritesNoAddressFails(t *testing.T) {
	f := newStartFixture(t)
	f.running()
	if err := os.Remove(filepath.Join(f.gridHome, AppURLFile)); err != nil {
		t.Fatal(err)
	}
	f.program("grid-server", "exit 0\n")
	f.env.AppStart.Wait = 300 * time.Millisecond
	f.env.OpenURL = func(u string) error { t.Fatalf("opened %s with no address", u); return nil }
	if code := f.run(f.home, "open", "T-01"); code != 1 {
		t.Fatalf("exit %d, want 1: %s", code, f.errb)
	}
	if !strings.Contains(f.errb.String(), "grid-server did not write app-url") {
		t.Errorf("stderr %q", f.errb)
	}
}
