package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/worker"
)

// startFixture is an open fixture plus a folder of fake grid-server and grid-worker
// scripts. Each start appends a line to <markers>/<name>.started. The fake worker copies
// the presence files a real one writes; the fake server writes app-url with the address of
// a listener the test holds.
type startFixture struct {
	*harness
	project, id string
	bin         string // folder of fake programs
	markers     string
	presence    string // folder holding the worker.json and worker.lock the fake worker copies
	addr        string // where the fake server "listens"
}

func newStartFixture(t *testing.T) *startFixture {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake programs are shell scripts")
	}
	h, project, id := openFixture(t, true)
	f := &startFixture{harness: h, project: project, id: id,
		bin: t.TempDir(), markers: t.TempDir(), presence: t.TempDir()}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f.addr = ln.Addr().String()
	t.Setenv("FAKE_MARKERS", f.markers)
	t.Setenv("FAKE_PRESENCE", f.presence)
	t.Setenv("FAKE_ADDR", f.addr)

	// The worker's presence files name this machine by its ULID, as the app writes them.
	machine := "01BX5ZZKBKACTAV9WEVGEMMVRZ"
	h.write(filepath.Join(h.gridHome, "machine"), machine+"\n")
	now := time.Now().UTC()
	status, _ := json.Marshal(worker.Status{Version: 1, Machine: machine, PID: 4242, StartedAt: now.Add(-10 * time.Second), UpdatedAt: now.Add(-time.Second), State: "running"})
	lock, _ := json.Marshal(worker.Lock{Version: 1, Machine: machine, PID: 4242, StartedAt: now.Add(-10 * time.Second)})
	h.write(filepath.Join(f.presence, "worker.json"), string(status))
	h.write(filepath.Join(f.presence, "worker.lock"), string(lock))

	f.program("grid-worker", `cp "$FAKE_PRESENCE/worker.json" "$GRID_HOME/worker.json"
cp "$FAKE_PRESENCE/worker.lock" "$GRID_HOME/worker.lock"
`)
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
	for _, n := range []string{"worker.json", "worker.lock"} {
		b, err := os.ReadFile(filepath.Join(f.presence, n))
		if err != nil {
			f.t.Fatal(err)
		}
		f.write(filepath.Join(f.gridHome, n), string(b))
	}
}

// starts counts how often a fake program ran. A detached program is not waited for, so
// this waits until the first run shows (when want says one is due), then a little longer
// for a second run that should not come.
func (f *startFixture) starts(name string, want int) int {
	count := func() int {
		b, err := os.ReadFile(filepath.Join(f.markers, name+".started"))
		if err != nil {
			return 0
		}
		return strings.Count(string(b), "started\n")
	}
	for deadline := time.Now().Add(5 * time.Second); count() < want && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(300 * time.Millisecond)
	return count()
}

// startedOnce reports the worker and server start counts for a run that should start the given numbers.
func (f *startFixture) counts(wantWorker, wantServer int) (worker, server int) {
	return f.starts("grid-worker", wantWorker), f.starts("grid-server", wantServer)
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
	if w, s := f.counts(1, 1); w != 1 || s != 1 {
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
	if w, s := f.counts(0, 0); w != 0 || s != 0 {
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
	if w, s := f.counts(1, 1); w != 1 || s != 1 {
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
	if w, s := f.counts(0, 1); s != 1 || w != 0 {
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
	if w := f.starts("grid-worker", 0); w != 0 {
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
