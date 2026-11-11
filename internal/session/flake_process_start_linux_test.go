//go:build linux

package session

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/gridconsole/grid-plugins/internal/contract"
	"github.com/gridconsole/grid-plugins/internal/processid"
)

// flakePSNoPS puts a ps that exits 1 at once as the only command on PATH, so a
// test fails if the code under test runs ps by name.
func flakePSNoPS(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

// ProcessStart and ProbeLocal answer for a live pid with no ps they can run.
func TestFlakePSLinuxRunsNoPS(t *testing.T) {
	flakePSNoPS(t)
	self := os.Getpid()
	start, err := ProcessStart(self)
	if err != nil {
		t.Fatalf("ProcessStart needed ps: %v", err)
	}
	if got := ProbeLocal(Live{PID: self, PIDStart: contract.FormatTime(start)}); got != Alive {
		t.Errorf("RFC3339 token: got %v, want Alive", got)
	}
	raw := start.UTC().Format(processid.LstartLayout)
	if got := ProbeLocal(Live{PID: self, PIDStart: raw}); got != Alive {
		t.Errorf("raw UTC lstart token %q: got %v, want Alive", raw, got)
	}
}

// A different start is still proven Dead with no ps to ask.
func TestFlakePSLinuxOldTokenIsDeadWithoutPS(t *testing.T) {
	flakePSNoPS(t)
	if got := ProbeLocal(Live{PID: os.Getpid(), PIDStart: "2000-01-01T00:00:00.000Z"}); got != Dead {
		t.Fatalf("got %v, want Dead", got)
	}
}

// lstartLocal on Linux is not processid.PSLstart, which runs /bin/ps by its
// absolute path: PATH cannot stand in for it, so the function itself is compared.
func TestFlakePSLinuxLocalIsNotPSLstart(t *testing.T) {
	got := reflect.ValueOf(localHost.lstartLocal).Pointer()
	if got == reflect.ValueOf(processid.PSLstart).Pointer() {
		t.Fatal("lstartLocal is processid.PSLstart, which forks /bin/ps")
	}
}

// flakePSRunPS runs /bin/ps with no deadline for one pid; env is its whole environment.
func flakePSRunPS(t *testing.T, pid int, env ...string) string {
	t.Helper()
	if _, err := os.Stat("/bin/ps"); err != nil {
		t.Skipf("no /bin/ps to compare with: %v", err)
	}
	cmd := exec.Command("/bin/ps", "-o", "lstart=", "-p", strconv.Itoa(pid))
	cmd.Env = env
	b, err := cmd.Output()
	if err != nil {
		t.Fatalf("ps -p %d: %v", pid, err)
	}
	return strings.TrimSpace(string(b))
}

// ps is what stored tokens were written with: the start read from /proc must equal it.
func TestFlakePSLinuxMatchesPS(t *testing.T) {
	for _, pid := range []int{os.Getpid(), 1} {
		utcText := flakePSRunPS(t, pid, "TZ=UTC", "LC_ALL=C")
		want, err := processid.ParseLstart(utcText, time.UTC)
		if err != nil {
			t.Fatalf("pid %d: parse ps text %q: %v", pid, utcText, err)
		}
		got, err := ProcessStart(pid)
		if err != nil {
			t.Fatalf("pid %d: ProcessStart: %v", pid, err)
		}
		if !got.Equal(want) {
			t.Errorf("pid %d: ProcessStart %v, ps %v", pid, got, want)
		}
		if text, err := localHost.lstartUTC(pid); err != nil || text != utcText {
			t.Errorf("pid %d: lstartUTC %q (%v), ps %q", pid, text, err, utcText)
		}
		// PSLstart runs ps with LC_ALL=C and no TZ, the zone of /etc/localtime.
		localText := flakePSRunPS(t, pid, "LC_ALL=C")
		// probePS trims what lstartLocal returns, as PSLstart's line ends in a newline.
		if text, err := localHost.lstartLocal(pid); err != nil || strings.TrimSpace(text) != localText {
			t.Errorf("pid %d: lstartLocal %q (%v), ps %q", pid, text, err, localText)
		}
	}
}

// flakePSFiles is a read-file function over fixed text; a path not in the map is missing.
func flakePSFiles(files map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		s, ok := files[path]
		if !ok {
			return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
		}
		return []byte(s), nil
	}
}

// flakePSStat builds a /proc/<pid>/stat line with the given command field and start ticks.
func flakePSStat(comm, ticks string) string {
	return "42 " + comm + " S 1 42 42 0 -1 4194304 100 0 0 0 1 1 0 0 20 0 1 0 " + ticks + " 1000 100 18446744073709551615"
}

const flakePSMachine = "cpu  1 2 3 4\ncpu0 1 2 3 4\nintr 9\nbtime 1700000000\nprocesses 77\n"

// flakePSTZif is a TZif file with one zone at a fixed offset and no transitions.
func flakePSTZif(offset int32, abbr string) []byte {
	var b bytes.Buffer
	b.WriteString("TZif")
	b.Write(make([]byte, 16)) // version 0 and the reserved bytes
	for _, n := range []uint32{0, 0, 0, 0, 1, uint32(len(abbr) + 1)} {
		_ = binary.Write(&b, binary.BigEndian, n)
	}
	_ = binary.Write(&b, binary.BigEndian, offset)
	b.Write([]byte{0, 0}) // not dst, abbreviation at 0
	b.WriteString(abbr)
	b.WriteByte(0)
	return b.Bytes()
}

// btime + ticks/100, in whole seconds, for command names that hold spaces and ") ".
func TestFlakePSLinuxProcStartFromFixedText(t *testing.T) {
	want := time.Unix(1700000000+123, 0).UTC()
	for _, tc := range []struct{ name, comm string }{
		{"plain", "(grid)"},
		{"spaces", "(grid x y)"},
		{"close paren and space", "(a) b (c) d)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rf := flakePSFiles(map[string]string{
				"/proc/42/stat": flakePSStat(tc.comm, "12399"), // 123.99 s: truncated, not rounded
				"/proc/stat":    flakePSMachine,
			})
			got, err := procStartFrom(rf, 42)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(want) || got.Location() != time.UTC || got.Nanosecond() != 0 {
				t.Fatalf("got %v, want %v", got, want)
			}
		})
	}
}

// Missing or malformed text is an error that names the pid, never a panic or a guessed time.
func TestFlakePSLinuxProcStartFromBadText(t *testing.T) {
	good := flakePSStat("(grid)", "500")
	for _, tc := range []struct {
		name  string
		files map[string]string
	}{
		{"pid missing", map[string]string{"/proc/stat": flakePSMachine}},
		{"btime missing", map[string]string{"/proc/42/stat": good, "/proc/stat": "cpu 1\nprocesses 77\n"}},
		{"btime empty", map[string]string{"/proc/42/stat": good, "/proc/stat": "btime\n"}},
		{"btime text", map[string]string{"/proc/42/stat": good, "/proc/stat": "btime soon\n"}},
		{"btime negative", map[string]string{"/proc/42/stat": good, "/proc/stat": "btime -5\n"}},
		{"btime and ticks overflow", map[string]string{"/proc/42/stat": flakePSStat("(grid)", "999999"), "/proc/stat": "btime 9223372036854775807\n"}},
		{"machine stat missing", map[string]string{"/proc/42/stat": good}},
		{"empty stat", map[string]string{"/proc/42/stat": "", "/proc/stat": flakePSMachine}},
		{"no close paren", map[string]string{"/proc/42/stat": "42 grid S 1", "/proc/stat": flakePSMachine}},
		{"too few fields", map[string]string{"/proc/42/stat": "42 (grid) S 1 42", "/proc/stat": flakePSMachine}},
		{"ticks text", map[string]string{"/proc/42/stat": flakePSStat("(grid)", "late"), "/proc/stat": flakePSMachine}},
		{"ticks past uint64", map[string]string{"/proc/42/stat": flakePSStat("(grid)", "99999999999999999999"), "/proc/stat": flakePSMachine}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := procStartFrom(flakePSFiles(tc.files), 42)
			if err == nil {
				t.Fatalf("got %v, want an error", got)
			}
			if !strings.Contains(err.Error(), "pid 42") {
				t.Errorf("error does not name the pid: %v", err)
			}
		})
	}
}

// A pid that exits between the kill and the read gives ESRCH from the read: an error with the cause kept.
func TestFlakePSLinuxProcStartFromGonePid(t *testing.T) {
	rf := func(path string) ([]byte, error) {
		return nil, &fs.PathError{Op: "read", Path: path, Err: syscall.ESRCH}
	}
	_, err := procStartFrom(rf, 42)
	if !errors.Is(err, syscall.ESRCH) || !strings.Contains(err.Error(), "pid 42") {
		t.Fatalf("got %v, want an ESRCH error naming pid 42", err)
	}
	if _, err := procStart(1 << 30); err == nil { // beyond pid_max
		t.Fatal("a pid that cannot exist has a start")
	}
}

// The raw pid checks of ProcessStart stand with no ps.
func TestFlakePSLinuxProcessStartRejectsBadPids(t *testing.T) {
	flakePSNoPS(t)
	for _, pid := range []int{0, -1} {
		if _, err := ProcessStart(pid); err == nil {
			t.Errorf("accepted pid %d", pid)
		}
	}
}

// lstartLocal prints in the zone of /etc/localtime (read from the file, not from TZ), lstartUTC in UTC.
func TestFlakePSLinuxLstartZones(t *testing.T) {
	t.Setenv("TZ", "Asia/Tokyo") // must not reach the answer: ps run with no TZ ignores it
	files := map[string]string{
		"/proc/42/stat": flakePSStat("(grid)", "12399"),
		"/proc/stat":    flakePSMachine,
	}
	// 1700000123 is Tue Nov 14 22:15:23 2023 UTC.
	for _, tc := range []struct {
		name      string
		localtime []byte
		want      string
	}{
		{"minus five", flakePSTZif(-5*3600, "EST"), "Tue Nov 14 17:15:23 2023"},
		{"plus nine is the next day", flakePSTZif(9*3600, "JST"), "Wed Nov 15 07:15:23 2023"},
		{"no file is UTC", nil, "Tue Nov 14 22:15:23 2023"},
		{"garbage is UTC", []byte("not a zone"), "Tue Nov 14 22:15:23 2023"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.localtime != nil {
				files["/etc/localtime"] = string(tc.localtime)
			} else {
				delete(files, "/etc/localtime")
			}
			got, err := lstartIn(flakePSFiles(files), 42, true)
			if err != nil || got != tc.want {
				t.Fatalf("lstartLocal %q (%v), want %q", got, err, tc.want)
			}
		})
	}
	got, err := lstartIn(flakePSFiles(files), 42, false)
	if want := "Tue Nov 14 22:15:23 2023"; err != nil || got != want {
		t.Fatalf("lstartUTC %q (%v), want %q", got, err, want)
	}
	// 1699400000 + 99999 s is Thu Nov 9 03:19:59 2023 UTC: a one digit day is padded with a space.
	files["/proc/42/stat"] = flakePSStat("(grid)", "9999999")
	files["/proc/stat"] = "btime 1699400000\n"
	if got, err := lstartIn(flakePSFiles(files), 42, false); err != nil || got != "Thu Nov  9 03:19:59 2023" {
		t.Fatalf("lstartUTC %q (%v), want a space padded day", got, err)
	}
}
