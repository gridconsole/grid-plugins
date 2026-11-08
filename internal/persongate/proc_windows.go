//go:build windows

package persongate

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

var queryProcess = syscall.NewLazyDLL("ntdll.dll").NewProc("NtQueryInformationProcess")

func processDetails(pid int) (uint64, []string, error) {
	h, err := syscall.OpenProcess(0x1000, false, uint32(pid)) // QUERY_LIMITED_INFORMATION
	if err != nil {
		return 0, nil, err
	}
	defer syscall.CloseHandle(h)
	var created, exited, kernel, user syscall.Filetime
	if err := syscall.GetProcessTimes(h, &created, &exited, &kernel, &user); err != nil {
		return 0, nil, err
	}
	// ProcessCommandLineInformation returns a UNICODE_STRING and its backing text.
	buf := make([]byte, 65536)
	var size uint32
	status, _, _ := queryProcess.Call(uintptr(h), 60, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), uintptr(unsafe.Pointer(&size)))
	if status != 0 {
		return 0, nil, errors.New("cannot read process arguments")
	}
	type unicodeString struct {
		Length, MaximumLength uint16
		Buffer                *uint16
	}
	u := (*unicodeString)(unsafe.Pointer(&buf[0]))
	args := strings.Fields(syscall.UTF16ToString(unsafe.Slice(u.Buffer, int(u.Length)/2)))
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), args, nil
}
func (SystemProcs) Snapshot() (map[int]Process, error) {
	h, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(h)
	all := map[int]Process{}
	e := syscall.ProcessEntry32{Size: uint32(unsafe.Sizeof(syscall.ProcessEntry32{}))}
	err = syscall.Process32First(h, &e)
	for err == nil {
		p := Process{PID: int(e.ProcessID), PPID: int(e.ParentProcessID), Exe: syscall.UTF16ToString(e.ExeFile[:])}
		all[p.PID] = p
		err = syscall.Process32Next(h, &e)
	}
	if err != syscall.ERROR_NO_MORE_FILES {
		return nil, err
	}
	tree := map[int]Process{}
	var childTime uint64
	for pid := os.Getpid(); pid > 0; {
		if _, ok := tree[pid]; ok {
			break
		}
		p, ok := all[pid]
		if !ok {
			break
		}
		start, args, err := processDetails(pid)
		if err != nil {
			return nil, err
		}
		if childTime != 0 && start > childTime {
			break
		} // dead/reused parent is gone
		p.Args = args
		tree[pid] = p
		childTime = start
		pid = p.PPID
	}
	return tree, nil
}
