//go:build linux

package persongate

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Snapshot reads the caller and its ancestry. Unrelated processes need not be
// readable. A disappearing parent ends the chain; other read errors fail closed.
func (SystemProcs) Snapshot() (map[int]Process, error) { return linuxSnapshot("/proc") }

func linuxSnapshot(procRoot string) (map[int]Process, error) {
	tree := map[int]Process{}
	for pid := os.Getpid(); pid > 0; {
		if _, ok := tree[pid]; ok {
			break
		}
		dir := filepath.Join(procRoot, strconv.Itoa(pid))
		raw, err := os.ReadFile(filepath.Join(dir, "stat"))
		if os.IsNotExist(err) && pid != os.Getpid() {
			break
		}
		if err != nil {
			return nil, err
		}
		end := strings.LastIndexByte(string(raw), ')')
		if end < 0 {
			return nil, fmt.Errorf("invalid process stat")
		}
		fields := strings.Fields(string(raw[end+1:]))
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid process parent")
		}
		parent, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, err
		}
		cmd, err := os.ReadFile(filepath.Join(dir, "cmdline"))
		if err != nil {
			return nil, err
		}
		args := strings.Split(strings.TrimRight(string(cmd), "\x00"), "\x00")
		exe, err := os.Readlink(filepath.Join(dir, "exe"))
		if err != nil {
			if len(args) == 0 || args[0] == "" {
				return nil, err
			}
			exe = args[0]
		}
		tree[pid] = Process{pid, parent, exe, args}
		pid = parent
	}
	return tree, nil
}
