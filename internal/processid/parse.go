package processid

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// LstartLayout is the line `ps -o lstart=` prints under LC_ALL=C. The day is
// space padded to two columns ("Tue Nov  3 09:15:30 2026").
const LstartLayout = "Mon Jan _2 15:04:05 2006"

// ErrNoProcess is what ParseLstart returns for empty output, which is how ps
// answers for a pid that has gone.
var ErrNoProcess = errors.New("ps printed no start time")

// ParseLstart reads the output of one `ps -o lstart= -p <pid>` run. ps prints
// the wall clock in the zone it ran with, so the caller names that zone. Empty
// output is ErrNoProcess; anything that is not one C locale line is an error.
func ParseLstart(out string, loc *time.Location) (time.Time, error) {
	s := strings.TrimSpace(out)
	if s == "" {
		return time.Time{}, ErrNoProcess
	}
	if strings.ContainsAny(s, "\r\n") {
		return time.Time{}, fmt.Errorf("ps printed more than one line: %q", s)
	}
	return time.ParseInLocation(LstartLayout, s, loc)
}

// BootIDPath is where Linux names the current boot. Process start ticks repeat
// after a reboot, so a Linux token carries the boot ID next to them.
const BootIDPath = "/proc/sys/kernel/random/boot_id"

// ProcStartTicks returns the start time field, the 22nd, of a /proc/<pid>/stat
// line. The command name sits in parentheses and may hold spaces and ')', so
// the fields are counted after the last ')'.
func ProcStartTicks(stat string) (string, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return "", false
	}
	f := strings.Fields(stat[i+1:])
	if len(f) < 20 || f[19] == "" {
		return "", false
	}
	for _, c := range f[19] {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	return f[19], true
}

const procPrefix = "proc:"

// ProcToken is the Linux start token, "proc:<boot id>:<start ticks>".
func ProcToken(boot, ticks string) string { return procPrefix + boot + ":" + ticks }

// ParseProcToken splits a recorded Linux token. Older hooks wrote
// "proc:<start ticks>" with no boot ID; boot is empty for those.
func ParseProcToken(token string) (boot, ticks string, ok bool) {
	if !strings.HasPrefix(token, procPrefix) {
		return "", "", false
	}
	rest := token[len(procPrefix):]
	if i := strings.LastIndexByte(rest, ':'); i >= 0 {
		boot, ticks = rest[:i], rest[i+1:]
	} else {
		ticks = rest
	}
	if ticks == "" {
		return "", "", false
	}
	return boot, ticks, true
}
