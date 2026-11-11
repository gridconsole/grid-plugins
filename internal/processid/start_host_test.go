//go:build unix

package processid

import (
	"errors"
	"os"
	"testing"
)

// fakeHost answers reads from a map and records every path asked for.
func fakeHost(goos string, files map[string]string, lstart func(int) (string, error)) (host, *[]string) {
	var reads []string
	return host{
		goos: goos,
		readFile: func(path string) ([]byte, error) {
			reads = append(reads, path)
			if s, ok := files[path]; ok {
				return []byte(s), nil
			}
			return nil, os.ErrNotExist
		},
		lstart: lstart,
	}, &reads
}

func noPS(t *testing.T) func(int) (string, error) {
	return func(pid int) (string, error) {
		t.Errorf("ps run for pid %d", pid)
		return "", errors.New("unexpected")
	}
}

func TestStartLinuxReadsProcAndBootID(t *testing.T) {
	h, _ := fakeHost("linux", map[string]string{
		"/proc/42/stat": statLine("42", "grid", "222"),
		BootIDPath:      "boot-a\n",
	}, noPS(t))
	if got := h.start(42); got != "proc:boot-a:222" {
		t.Fatalf("got %q", got)
	}
	// Same pid after the process is replaced: the token differs.
	h, _ = fakeHost("linux", map[string]string{
		"/proc/42/stat": statLine("42", "grid", "999"),
		BootIDPath:      "boot-a\n",
	}, noPS(t))
	if got := h.start(42); got != "proc:boot-a:999" {
		t.Fatalf("reused pid: got %q", got)
	}
	// No boot ID, no token.
	h, _ = fakeHost("linux", map[string]string{"/proc/42/stat": statLine("42", "grid", "222")}, noPS(t))
	if got := h.start(42); got != "" {
		t.Fatalf("no boot id: got %q", got)
	}
}

func TestStartDarwinUsesPSOnly(t *testing.T) {
	var asked []int
	h, reads := fakeHost("darwin", map[string]string{
		"/proc/42/stat": statLine("42", "grid", "222"),
		BootIDPath:      "boot-a\n",
	}, func(pid int) (string, error) {
		asked = append(asked, pid)
		return "Tue Nov  3 09:15:30 2026\n", nil
	})
	if got := h.start(42); got != "Tue Nov  3 09:15:30 2026" {
		t.Fatalf("got %q", got)
	}
	if len(asked) != 1 || asked[0] != 42 {
		t.Fatalf("ps asked for %v", asked)
	}
	if len(*reads) != 0 {
		t.Fatalf("darwin read files: %v", *reads)
	}
	// A pid that has gone: ps prints nothing and exits 1.
	h, _ = fakeHost("darwin", nil, func(int) (string, error) { return "", errors.New("exit status 1") })
	if got := h.start(42); got != "" {
		t.Fatalf("gone pid: got %q", got)
	}
	// ps exits 0 with no text: no identity either.
	h, _ = fakeHost("darwin", nil, func(int) (string, error) { return "\n", nil })
	if got := h.start(42); got != "" {
		t.Fatalf("blank ps: got %q", got)
	}
}

func TestStartRejectsNonPositivePID(t *testing.T) {
	for _, goos := range []string{"linux", "darwin"} {
		h, reads := fakeHost(goos, nil, noPS(t))
		for _, pid := range []int{0, -1} {
			if got := h.start(pid); got != "" || len(*reads) != 0 {
				t.Errorf("%s pid %d: %q, reads %v", goos, pid, got, *reads)
			}
		}
	}
}
