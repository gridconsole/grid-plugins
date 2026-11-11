//go:build linux

package session

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gridconsole/grid-plugins/internal/processid"
)

// userHZ is the unit of the start time in /proc/<pid>/stat. The kernel reports
// it in USER_HZ, which is 100 on every Linux target Go supports.
const userHZ = 100

// localtimePath is the zone ps prints in when it runs with no TZ.
const localtimePath = "/etc/localtime"

// procStart reads the process start time in UTC at one second precision from
// /proc, with no ps: ps takes a fork and a scan of /proc, and a busy box lets
// that outlast any deadline.
func procStart(pid int) (time.Time, error) { return procStartFrom(os.ReadFile, pid) }

// procStartFrom is procStart over a read-file function, so tests can feed fixed
// text. The start is the boot time (the btime line of /proc/stat) plus the
// start ticks (field 22 of /proc/<pid>/stat) over USER_HZ, the sum procps-ng
// prints as lstart.
func procStartFrom(readFile func(path string) ([]byte, error), pid int) (time.Time, error) {
	stat, err := readFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return time.Time{}, fmt.Errorf("read start of pid %d: %w", pid, err)
	}
	ticksText, ok := processid.ProcStartTicks(string(stat))
	if !ok {
		return time.Time{}, fmt.Errorf("read start of pid %d: no start time in /proc/%d/stat", pid, pid)
	}
	ticks, err := strconv.ParseUint(ticksText, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("read start of pid %d: start time %q: %w", pid, ticksText, err)
	}
	machine, err := readFile("/proc/stat")
	if err != nil {
		return time.Time{}, fmt.Errorf("read start of pid %d: %w", pid, err)
	}
	boot, ok := bootTime(string(machine))
	if !ok {
		return time.Time{}, fmt.Errorf("read start of pid %d: no boot time in /proc/stat", pid)
	}
	secs := ticks / userHZ
	if secs > uint64(math.MaxInt64-boot) {
		return time.Time{}, fmt.Errorf("read start of pid %d: start time %d ticks is out of range", pid, ticks)
	}
	return time.Unix(boot+int64(secs), 0).UTC(), nil
}

// bootTime is the btime line of /proc/stat, seconds since the epoch.
func bootTime(procStat string) (int64, bool) {
	for _, line := range strings.Split(procStat, "\n") {
		f := strings.Fields(line)
		if len(f) != 2 || f[0] != "btime" {
			continue
		}
		boot, err := strconv.ParseInt(f[1], 10, 64)
		return boot, err == nil && boot >= 0
	}
	return 0, false
}

// lstartIn prints the start of pid as ps prints it under LC_ALL=C: time.ANSIC
// with the day padded by a space, in UTC or in the system zone.
func lstartIn(readFile func(path string) ([]byte, error), pid int, system bool) (string, error) {
	start, err := procStartFrom(readFile, pid)
	if err != nil {
		return "", err
	}
	zone := time.UTC
	if system {
		zone = systemZone(readFile)
	}
	return start.In(zone).Format(processid.LstartLayout), nil
}

// systemZone is the zone ps prints in when it runs with LC_ALL=C and no TZ,
// which is the zone of /etc/localtime. time.Local is not used because Go reads
// the TZ variable of this process into it, and ps, run with the environment
// processid.PSLstart gives it, would not. A missing or unreadable file is UTC,
// as it is for the C library.
func systemZone(readFile func(path string) ([]byte, error)) *time.Location {
	data, err := readFile(localtimePath)
	if err != nil {
		return time.UTC
	}
	zone, err := time.LoadLocationFromTZData("Local", data)
	if err != nil {
		return time.UTC
	}
	return zone
}

// The Linux pair for localHost: ps's lstart text read from /proc, in UTC and in
// the system zone. Neither forks ps.
var (
	osLstartUTC   = func(pid int) (string, error) { return lstartIn(os.ReadFile, pid, false) }
	osLstartLocal = func(pid int) (string, error) { return lstartIn(os.ReadFile, pid, true) }
)
