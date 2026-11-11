// Package proctree runs a command and, once it is over, ends the processes it
// left behind.
//
// On Unix the callers end the command's process group after the run. On Windows
// taskkill /T cannot find the descendants of a process that has already exited,
// and its pid may belong to someone else by then, so Run puts the command in a
// job object right after it starts and ends the job after it has been waited
// for. Elsewhere Run is exactly cmd.Run.
//
// Known gap: the job is assigned after the process has been created, so a
// descendant the command starts before that assignment is outside the job. Go's
// os/exec cannot start a process suspended and resume it, and this package does
// not add an undocumented resume call.
package proctree

import "os/exec"

// ops are the four steps of a run. They are separate so the order can be tested
// on any system; newOps builds the real ones for the system in use.
type ops struct {
	// start begins the command.
	start func() error
	// attach puts the started command under the tree. It returns a handle that
	// end needs, or 0 when there is nothing to end. On error it returns 0 and
	// holds nothing.
	attach func() (uintptr, error)
	// wait returns when the command is over.
	wait func() error
	// end ends whatever is left of the tree and releases the handle.
	end func(handle uintptr)
}

// Run is cmd.Run with the tree ended after the command is over. Where no tree
// can be set up, because the system has none or because attaching failed for any
// reason, the command runs exactly as under cmd.Run; a failure to attach is
// never an error of the run, and the error returned is always Start's or
// Wait's, unchanged.
func Run(cmd *exec.Cmd) error {
	return run(newOps(cmd))
}

func run(o ops) error {
	if err := o.start(); err != nil {
		return err
	}
	handle, attachErr := o.attach()
	if attachErr == nil && handle != 0 {
		// Deferred so the tree is ended even if wait panics.
		defer o.end(handle)
	}
	return o.wait()
}
