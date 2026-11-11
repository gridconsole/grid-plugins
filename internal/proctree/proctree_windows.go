//go:build windows

package proctree

import (
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"unsafe"
)

// Values from the Windows SDK headers.
const (
	// JobObjectInfoClass for SetInformationJobObject: JobObjectExtendedLimitInformation.
	jobObjectExtendedLimitInformation = 9
	// JOBOBJECT_BASIC_LIMIT_INFORMATION.LimitFlags: the job's processes end when
	// the last handle to the job closes. It is the only limit this package sets;
	// no memory, CPU, process-count or time limit is set on the job.
	jobObjectLimitKillOnJobClose = 0x2000
	// Process access rights AssignProcessToJobObject needs on the process handle.
	processSetQuota  = 0x0100
	processTerminate = 0x0001

	// Size of JOBOBJECT_EXTENDED_LIMIT_INFORMATION on a 64-bit system (amd64 and
	// arm64): 64 (basic) + 48 (IO_COUNTERS) + 4*8. The struct below has that
	// size only there; on a 32-bit build attach refuses to go on and the command
	// runs as it would under cmd.Run.
	extendedLimitInformationSize = 144

	// Exit code the job's processes get when the job is terminated.
	jobTerminateExitCode = 1
)

// ioCounters is IO_COUNTERS: six ULONGLONGs, 48 bytes.
type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

// basicLimitInformation is JOBOBJECT_BASIC_LIMIT_INFORMATION: 64 bytes on amd64
// and arm64 (LimitFlags and ActiveProcessLimit are each followed by 4 bytes of
// padding that the compiler inserts, because the next field is pointer-sized).
type basicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

// extendedLimitInformation is JOBOBJECT_EXTENDED_LIMIT_INFORMATION: 144 bytes
// on amd64 and arm64.
type extendedLimitInformation struct {
	BasicLimitInformation basicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObject          = kernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = kernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = kernel32.NewProc("AssignProcessToJobObject")
	procTerminateJobObject       = kernel32.NewProc("TerminateJobObject")
)

func newOps(cmd *exec.Cmd) ops {
	return ops{
		start:  cmd.Start,
		attach: func() (uintptr, error) { return attachJob(cmd) },
		wait:   cmd.Wait,
		end:    endJob,
	}
}

// attachJob creates a job whose only flag is KILL_ON_JOB_CLOSE and assigns the
// started process to it. Before Wait the process handle that os/exec holds is
// still open, so the pid cannot have been reused when it is opened here.
//
// Assignment can fail: on a host before Windows 8, where a process already in a
// job cannot join a second one, or when the process has already exited. Then the
// error is returned with no handle held, and the caller goes on without a job.
func attachJob(cmd *exec.Cmd) (uintptr, error) {
	if cmd.Process == nil {
		return 0, errors.New("proctree: process not started")
	}
	var info extendedLimitInformation
	if unsafe.Sizeof(info) != extendedLimitInformationSize {
		return 0, fmt.Errorf("proctree: job limit struct is %d bytes, want %d", unsafe.Sizeof(info), extendedLimitInformationSize)
	}
	job, _, err := procCreateJobObject.Call(0, 0)
	if job == 0 {
		return 0, fmt.Errorf("proctree: CreateJobObject: %w", err)
	}
	info.BasicLimitInformation.LimitFlags = jobObjectLimitKillOnJobClose
	r, _, err := procSetInformationJobObject.Call(job, jobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r == 0 {
		closeHandle(job)
		return 0, fmt.Errorf("proctree: SetInformationJobObject: %w", err)
	}
	process, err := syscall.OpenProcess(processSetQuota|processTerminate, false, uint32(cmd.Process.Pid))
	if err != nil {
		closeHandle(job)
		return 0, fmt.Errorf("proctree: OpenProcess: %w", err)
	}
	defer func() { _ = syscall.CloseHandle(process) }()
	r, _, err = procAssignProcessToJobObject.Call(job, uintptr(process))
	if r == 0 {
		closeHandle(job)
		return 0, fmt.Errorf("proctree: AssignProcessToJobObject: %w", err)
	}
	return job, nil
}

// endJob ends every process still in the job and closes the handle. Closing it
// alone would end them too (KILL_ON_JOB_CLOSE); the explicit call makes the end
// happen here, before Run returns.
func endJob(job uintptr) {
	_, _, _ = procTerminateJobObject.Call(job, jobTerminateExitCode)
	closeHandle(job)
}

func closeHandle(h uintptr) { _ = syscall.CloseHandle(syscall.Handle(h)) }
