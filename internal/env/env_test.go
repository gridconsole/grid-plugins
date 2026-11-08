//go:build linux || darwin

package env

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/ports"
)

// Build the immutable helper once per test process, including repeated -count runs.
var serverFixture struct {
	once   sync.Once
	dir    string
	binary string
	err    error
	output []byte
}

func TestMain(m *testing.M) {
	code := m.Run()
	if serverFixture.dir != "" {
		if err := os.RemoveAll(serverFixture.dir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
		}
	}
	os.Exit(code)
}

func devServer(t *testing.T) string {
	t.Helper()
	serverFixture.once.Do(func() {
		serverFixture.dir, serverFixture.err = os.MkdirTemp("", "grid-env-server-")
		if serverFixture.err != nil {
			return
		}
		serverFixture.binary = filepath.Join(serverFixture.dir, "server")
		serverFixture.output, serverFixture.err = exec.Command("go", "build", "-o", serverFixture.binary, "./testdata/server").CombinedOutput()
	})
	if serverFixture.err != nil {
		t.Fatalf("build helper: %v %s", serverFixture.err, serverFixture.output)
	}
	return serverFixture.binary
}

// Every helper process holds a connection until exit. EOF observes shutdown without
// depending on when the OS reaps an orphaned zombie (which kill(pid, 0) sees).
func serverLifetime(t *testing.T, dir string) *net.TCPListener {
	t.Helper()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	if err := os.WriteFile(filepath.Join(dir, "server.lifetime"), []byte(listener.Addr().String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return listener
}

func assertStopped(t *testing.T, lifetime *net.TCPListener, processes int) {
	t.Helper()
	// Share one bound across accepting and reading every process's connection.
	deadline := time.Now().Add(3 * time.Second)
	if err := lifetime.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < processes; i++ {
		conn, err := lifetime.AcceptTCP()
		if err != nil {
			t.Fatalf("missing process lifetime witness: %v", err)
		}
		defer conn.Close()
		if err := conn.SetReadDeadline(deadline); err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(conn)
		if err != nil {
			t.Fatalf("server processes survived shutdown: %v", err)
		}
		if string(data) != "started\n" {
			t.Fatalf("process lifetime witness: %q", data)
		}
	}
}

func TestDevServerLifecycle(t *testing.T) {
	binary := devServer(t)
	for _, mode := range []string{"healthy", "legacy", "tree", "term", "stubborn", "unhealthy", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			dir := t.TempDir()
			lifetime := serverLifetime(t, dir)
			m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "tasks", "T-1"), Dir: dir, Grace: 100 * time.Millisecond}
			// Avoid assuming the standard dev range is unused on the test machine.
			port := freeBlock(t)
			c := Config{Command: "exec '" + strings.ReplaceAll(binary, "'", "'\"'\"'") + "' " + mode, HealthURL: "http://127.0.0.1:{port}/health", Timeout: "3s", PortRange: []int{port, port + 9}}
			t.Cleanup(func() {
				if err := m.Down("J-1"); err != nil {
					t.Error(err)
				}
			})
			r, err := m.Up(context.Background(), "J-1", c)
			if mode == "unhealthy" || mode == "redirect" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected health timeout: %v", err)
				}
				assertEmpty(t, m.Ports)
				assertStopped(t, lifetime, 1)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if r.PID <= 0 || r.PIDStart == "" || !r.Group || !ports.Alive(r.PID, true) {
				t.Fatalf("server not recorded: %+v", r)
			}
			if mode == "tree" {
				data, err := os.ReadFile(filepath.Join(dir, "server.pid"))
				if err != nil {
					t.Fatal(err)
				}
				child, _ := strconv.Atoi(string(data))
				if child == r.PID {
					t.Fatal("test did not start a descendant")
				}
			}
			// A second up returns the same healthy server, and release cannot orphan it.
			again, err := m.Up(context.Background(), "J-1", c)
			if err != nil || again != r {
				t.Fatalf("repeat up: %+v %v", again, err)
			}
			if err := m.Ports.Release(m.TaskDir, "J-1"); err == nil {
				t.Fatal("released live server")
			}
			if mode == "legacy" {
				r.PIDStart = ""
				data, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				for port := r.Start; port <= r.End; port++ {
					if err := os.WriteFile(filepath.Join(home, "ports", strconv.Itoa(port)+".json"), data, 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			started := time.Now()
			if err := m.Down("J-1"); err != nil {
				t.Fatal(err)
			}
			if elapsed := time.Since(started); mode == "stubborn" && elapsed < 2*m.Grace {
				t.Fatalf("stubborn server stopped after %s, want at least two grace periods (%s)", elapsed, 2*m.Grace)
			}
			processes := 1
			if mode == "tree" {
				processes = 2
			}
			assertStopped(t, lifetime, processes)
			assertEmpty(t, m.Ports)
			// TestStopSignalLadder and TestStopRechecksIdentityBeforeEscalation
			// in internal/ports/process_unix_test.go cover the removed signal-log
			// assertion. A loaded OS may not schedule a handler before SIGKILL.
		})
	}
}

// freeBlock keeps a listening sentinel just beyond the block until test cleanup.
// Concurrent test processes cannot select the same block, and using ports below
// the usual ephemeral range avoids racing the health client's outbound sockets.
func freeBlock(t *testing.T) int {
	t.Helper()
	for i := 0; i < 100; i++ {
		start := 10000 + rand.IntN(1600)*(ports.BlockSize+1)
		ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(start+ports.BlockSize)))
		if err != nil {
			continue
		}
		if (ports.Range{Start: start, End: start + ports.BlockSize - 1}).Available() {
			t.Cleanup(func() { _ = ln.Close() })
			return start
		}
		ln.Close()
	}
	t.Fatal("no free ten-port block")
	return 0
}

func assertEmpty(t *testing.T, s ports.Store) {
	t.Helper()
	records, err := s.List()
	if err != nil || len(records) != 0 {
		t.Fatalf("reservations remain: %+v %v", records, err)
	}
}

func TestBadConfigAndFailedStart(t *testing.T) {
	home := t.TempDir()
	m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "task"), Dir: t.TempDir(), Grace: 10 * time.Millisecond}
	for _, c := range []Config{
		{}, {Command: "exit 0", HealthURL: "http://example.com"},
		{Command: "exit 0", HealthURL: "http://127.0.0.1:{port}", Timeout: "-1s"},
		{Command: "exit 0", HealthURL: "http://127.0.0.1:{port}", PortRange: []int{1, 2}},
	} {
		if _, err := m.Up(context.Background(), "J-1", c); err == nil {
			t.Fatalf("accepted %+v", c)
		}
	}
	port := freeBlock(t)
	c := Config{Command: "exit 7", HealthURL: "http://127.0.0.1:{port}", Timeout: "1s", PortRange: []int{port, port + 9}}
	if _, err := m.Up(context.Background(), "J-1", c); err == nil || !strings.Contains(err.Error(), "exited") {
		t.Fatalf("failed command: %v", err)
	}
	assertEmpty(t, m.Ports)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := m.Up(ctx, "J-1", c); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestUpRefusesExistingListener(t *testing.T) {
	home := t.TempDir()
	m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "tasks", "T-1"), Dir: t.TempDir()}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := min(listener.Addr().(*net.TCPAddr).Port, 65526)
	c := Config{Command: "touch started", HealthURL: "http://127.0.0.1:{port}", PortRange: []int{port, port + 9}}
	if _, err := m.Up(context.Background(), "J-1", c); err == nil || !strings.Contains(err.Error(), "listener") {
		t.Fatalf("expected port conflict: %v", err)
	}
	if _, err := os.Stat(filepath.Join(m.Dir, "started")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("started command with occupied block")
	}
	assertEmpty(t, m.Ports)
}

// The parent `env up` is killed after the server is healthy, so nothing runs
// grid env down. gc must stop the server once the Job's lease has expired.
func TestGCStopsOrphanedServerAfterParentKilledAndLeaseExpiry(t *testing.T) {
	const holder = "worker@01K9J000000000000000000002"
	if home := os.Getenv("GRID_ENV_UP_HOME"); home != "" {
		start, _ := strconv.Atoi(os.Getenv("GRID_ENV_UP_PORT"))
		m := Manager{Ports: ports.Store{GridHome: home}, TaskDir: filepath.Join(home, "tasks", "T-1"), Dir: os.Getenv("GRID_ENV_UP_DIR")}
		c := Config{Command: os.Getenv("GRID_ENV_UP_COMMAND"), HealthURL: "http://127.0.0.1:{port}/health", Timeout: "10s", PortRange: []int{start, start + 9}}
		if _, err := m.Up(context.Background(), "J-1", c); err != nil {
			t.Fatal(err)
		}
		fmt.Fprintln(os.Stdout, "ready")
		select {}
	}
	binary := devServer(t)
	home, dir := t.TempDir(), t.TempDir()
	lifetime := serverLifetime(t, dir)
	task := filepath.Join(home, "tasks", "T-1")
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(task, "feed", "lease", "J-1.json"), `{"lease":"J-1","holder":"`+holder+`","fence":2,"gen":0,"attempt":1,"since":"2026-11-08T10:00:00.000Z","pid":null,"pid_start":null}`)
	write(filepath.Join(task, "feed", ".live", holder+".2.json"), `{"lease":"J-1","holder":"`+holder+`","fence":2,"attempt":1,"pid":1234,"pid_start":"2026-11-08T10:00:00.000Z","beat_at":"2026-11-08T10:00:00.000Z","output_at":"2026-11-08T10:00:00.000Z","until":"2026-11-08T10:10:00.000Z"}`)
	start := freeBlock(t)
	parent := exec.Command(os.Args[0], "-test.run=^TestGCStopsOrphanedServerAfterParentKilledAndLeaseExpiry$")
	parent.Env = append(os.Environ(), "GRID_ENV_UP_HOME="+home, "GRID_ENV_UP_DIR="+dir, "GRID_ENV_UP_PORT="+strconv.Itoa(start), "GRID_ENV_UP_COMMAND=exec '"+binary+"' healthy")
	ready, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	reaped := false
	defer func() {
		if !reaped {
			_ = parent.Process.Kill()
			_ = parent.Wait()
		}
	}()
	at := func(clock string) ports.Store {
		now, _ := time.Parse(time.RFC3339, clock)
		return ports.Store{GridHome: home, Now: func() time.Time { return now }, Grace: 100 * time.Millisecond}
	}
	// A recorded PID precedes health readiness; wait for Up to finish before
	// killing its parent so this tests a detached, healthy server.
	started := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(ready).ReadString('\n')
		if err == nil && line != "ready\n" {
			err = fmt.Errorf("unexpected readiness message %q", line)
		}
		started <- err
	}()
	select {
	case err := <-started:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("env up never became healthy")
	}
	records, err := at("2026-11-08T10:05:00Z").List()
	if err != nil || len(records) != 1 || records[0].PID == 0 || !records[0].Group {
		t.Fatalf("server not recorded: %+v %v", records, err)
	}
	rec := records[0]
	if err := parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = parent.Wait()
	reaped = true
	t.Cleanup(func() { _ = ports.Stop(rec.PID, 100*time.Millisecond) })
	if !ports.Alive(rec.PID, true) {
		t.Fatal("server died with its parent")
	}
	if removed, err := at("2026-11-08T10:05:00Z").GC(); err != nil || len(removed) != 0 || !ports.Alive(rec.PID, true) {
		t.Fatalf("gc touched a server under a live lease: %v %v", removed, err)
	}
	if removed, err := at("2026-11-08T11:00:00Z").GC(); err != nil || len(removed) != 1 {
		t.Fatalf("gc after expiry: %v %v", removed, err)
	}
	assertStopped(t, lifetime, 1)
	assertEmpty(t, ports.Store{GridHome: home})
}

func TestHealthWaitReleasesLockAndConfirmsReservation(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(strconv.FormatBool(existing), func(t *testing.T) {
			m := Manager{Ports: ports.Store{GridHome: t.TempDir()}, Dir: t.TempDir(), Grace: 10 * time.Millisecond}
			m.TaskDir = filepath.Join(m.Ports.GridHome, "tasks", "T-1")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			var block atomic.Bool
			block.Store(!existing)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if block.Load() {
					once.Do(func() { close(entered) })
					select {
					case <-release:
					case <-r.Context().Done():
						return
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			defer close(release)
			port := freeBlock(t)
			c := Config{Command: "exec sleep 60", HealthURL: server.URL + "/?port={port}", Timeout: "5s", PortRange: []int{port, port + 9}}
			t.Cleanup(func() {
				if err := m.Down("J-1"); err != nil {
					t.Error(err)
				}
			})
			if existing {
				if _, err := m.Up(ctx, "J-1", c); err != nil {
					t.Fatal(err)
				}
				block.Store(true)
			}
			result := make(chan error, 1)
			go func() { _, err := m.Up(ctx, "J-1", c); result <- err }()
			defer func() {
				cancel()
				select {
				case <-result:
				case <-time.After(6 * time.Second):
					t.Error("up did not finish")
				}
			}()
			select {
			case <-entered:
			case err := <-result:
				result <- err
				t.Fatalf("up ended before health wait: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("health wait did not start")
			}
			// List and down must finish while the health handler is still blocked.
			operations := make(chan error, 1)
			var replacement ports.Record
			go func() {
				records, err := m.Ports.List()
				if err == nil && len(records) != 1 {
					err = errors.New("missing starting server reservation")
				}
				if err == nil {
					err = m.Down("J-1")
				}
				if err == nil {
					replacement, err = m.Ports.Claim(m.TaskDir, "J-1", os.Getpid(), ports.Range{Start: port, End: port + 9})
				}
				operations <- err
			}()
			select {
			case err := <-operations:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				cancel()
				<-operations
				t.Fatal("allocator lock held across health wait")
			}
			cancel()
			select {
			case err := <-result:
				result <- err // let the deferred drain cover every exit path
				if err == nil || !strings.Contains(err.Error(), "reservation changed") {
					t.Fatalf("replacement not detected: %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("up did not confirm reservation")
			}
			records, err := m.Ports.List()
			if err != nil || len(records) != 1 || records[0] != replacement {
				t.Fatalf("up removed replacement: %+v %v", records, err)
			}
		})
	}
}

func TestDevCommandEnvironmentAllowlist(t *testing.T) {
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "AWS_SECRET_ACCESS_KEY", "GRID_PRIVATE_TOKEN", "BASH_ENV", "ENV"} {
		t.Setenv(key, "must-not-reach-child")
	}
	t.Setenv("PORT", "parent-port")
	t.Setenv("GRID_JOB", "parent-job")
	t.Setenv("LANG", "C")
	m := Manager{Ports: ports.Store{GridHome: t.TempDir()}, Dir: t.TempDir(), Grace: 10 * time.Millisecond}
	m.TaskDir = filepath.Join(m.Ports.GridHome, "tasks", "T-1")
	port := freeBlock(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := os.Stat(filepath.Join(m.Dir, "child.ready")); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	defer func() {
		if err := m.Down("J-1"); err != nil {
			t.Error(err)
		}
	}()
	c := Config{Command: "env > child.env; touch child.ready; exec sleep 60", HealthURL: server.URL + "/?port={port}", Timeout: "3s", PortRange: []int{port, port + 9}}
	if _, err := m.Up(context.Background(), "J-1", c); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(m.Dir, "child.env"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "must-not-reach-child") || strings.Contains(string(data), "parent-port") || strings.Contains(string(data), "parent-job") {
		t.Fatalf("parent secrets or port settings leaked: %s", data)
	}
	for _, entry := range []string{"LANG=C", "PORT=" + strconv.Itoa(port), "GRID_PORT=" + strconv.Itoa(port), "GRID_JOB=J-1", "PATH=" + os.Getenv("PATH")} {
		if !strings.Contains("\n"+string(data), "\n"+entry+"\n") {
			t.Errorf("child missing %q", entry)
		}
	}
}

func TestDownDoesNotSignalReusedPID(t *testing.T) {
	m := Manager{Ports: ports.Store{GridHome: t.TempDir()}, Dir: t.TempDir(), Grace: 10 * time.Millisecond}
	m.TaskDir = filepath.Join(m.Ports.GridHome, "tasks", "T-1")
	port := freeBlock(t)
	r, done, err := m.start("J-1", Config{Command: "exec sleep 60", PortRange: []int{port, port + 9}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := ports.StopRecord(r, m.grace()); err != nil {
			t.Error(err)
		}
	}()
	stale := r
	stale.PIDStart = "previous-process-start"
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	for p := stale.Start; p <= stale.End; p++ {
		if err := os.WriteFile(filepath.Join(m.Ports.GridHome, "ports", strconv.Itoa(p)+".json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Down("J-1"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		t.Fatal("down signalled reused PID")
	case <-time.After(50 * time.Millisecond):
	}
	assertEmpty(t, m.Ports)
}
