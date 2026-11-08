//go:build darwin

package persongate

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func (SystemProcs) Snapshot() (map[int]Process, error) {
	raw, err := exec.Command("/bin/ps", "-A", "-o", "pid=,ppid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	tree := map[int]Process{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) < 3 {
			return nil, fmt.Errorf("invalid ps process")
		}
		pid, e := strconv.Atoi(fields[0])
		if e != nil {
			return nil, e
		}
		parent, e := strconv.Atoi(fields[1])
		if e != nil {
			return nil, e
		}
		rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), fields[0]))
		rest = strings.TrimSpace(strings.TrimPrefix(rest, fields[1]))
		tree[pid] = Process{PID: pid, PPID: parent, Exe: rest}
	}
	raw, err = exec.Command("/bin/ps", "-A", "-o", "pid=,args=").Output()
	if err != nil {
		return nil, err
	}
	joined := map[int]Process{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		pid, e := strconv.Atoi(fields[0])
		if e != nil {
			return nil, e
		}
		if p, ok := tree[pid]; ok {
			p.Args = fields[1:]
			joined[pid] = p
		}
	}
	return joined, nil
}
