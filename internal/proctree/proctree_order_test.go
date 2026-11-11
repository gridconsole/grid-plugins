package proctree

import (
	"errors"
	"os/exec"
	"reflect"
	"testing"
)

// recorder builds fake ops that log each call, so the order of a run can be
// checked on any system.
type recorder struct {
	calls     []string
	startErr  error
	attachErr error
	waitErr   error
	handle    uintptr
	ended     []uintptr
}

func (r *recorder) ops() ops {
	return ops{
		start: func() error {
			r.calls = append(r.calls, "start")
			return r.startErr
		},
		attach: func() (uintptr, error) {
			r.calls = append(r.calls, "attach")
			if r.attachErr != nil {
				return 0, r.attachErr
			}
			return r.handle, nil
		},
		wait: func() error {
			r.calls = append(r.calls, "wait")
			return r.waitErr
		},
		end: func(h uintptr) {
			r.calls = append(r.calls, "end")
			r.ended = append(r.ended, h)
		},
	}
}

func TestRunOrderIsStartAttachWaitEnd(t *testing.T) {
	r := &recorder{handle: 7}
	if err := run(r.ops()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := []string{"start", "attach", "wait", "end"}; !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("calls = %v, want %v", r.calls, want)
	}
	if !reflect.DeepEqual(r.ended, []uintptr{7}) {
		t.Fatalf("end got handles %v, want [7]", r.ended)
	}
}

func TestRunStartFailureStopsAtStart(t *testing.T) {
	startErr := errors.New("start failed")
	r := &recorder{handle: 7, startErr: startErr}
	err := run(r.ops())
	if !errors.Is(err, startErr) {
		t.Fatalf("run returned %v, want Start's error", err)
	}
	if want := []string{"start"}; !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("calls = %v, want %v (no attach, wait or end)", r.calls, want)
	}
}

func TestRunAttachFailureStillWaitsAndIsNotTheRunsError(t *testing.T) {
	r := &recorder{handle: 7, attachErr: errors.New("no job")}
	if err := run(r.ops()); err != nil {
		t.Fatalf("run returned %v, want nil: an attach failure is never the run's error", err)
	}
	if want := []string{"start", "attach", "wait"}; !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("calls = %v, want %v (wait runs, end does not)", r.calls, want)
	}
	if len(r.ended) != 0 {
		t.Fatalf("end was called with %v after a failed attach", r.ended)
	}
}

func TestRunAttachFailureReturnsWaitsResult(t *testing.T) {
	waitErr := errors.New("wait failed")
	r := &recorder{attachErr: errors.New("no job"), waitErr: waitErr}
	if err := run(r.ops()); !errors.Is(err, waitErr) {
		t.Fatalf("run returned %v, want Wait's error", err)
	}
}

func TestRunNoHandleMeansNoEnd(t *testing.T) {
	// A system with no tree attaches nothing: no error and a zero handle.
	r := &recorder{}
	if err := run(r.ops()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if want := []string{"start", "attach", "wait"}; !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("calls = %v, want %v", r.calls, want)
	}
	if len(r.ended) != 0 {
		t.Fatalf("end was called with %v for a zero handle", r.ended)
	}
}

func TestRunExitErrorStillEndsAndIsReturnedUnchanged(t *testing.T) {
	exitErr := &exec.ExitError{}
	r := &recorder{handle: 7, waitErr: exitErr}
	err := run(r.ops())
	var got *exec.ExitError
	if !errors.As(err, &got) || got != exitErr {
		t.Fatalf("run returned %v (%T), want the same *exec.ExitError", err, err)
	}
	if want := []string{"start", "attach", "wait", "end"}; !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("calls = %v, want %v", r.calls, want)
	}
}

func TestRunEndsEvenWhenWaitPanics(t *testing.T) {
	r := &recorder{handle: 7}
	o := r.ops()
	o.wait = func() error {
		r.calls = append(r.calls, "wait")
		panic("boom")
	}
	func() {
		defer func() { _ = recover() }()
		_ = run(o)
	}()
	if want := []string{"start", "attach", "wait", "end"}; !reflect.DeepEqual(r.calls, want) {
		t.Fatalf("calls = %v, want %v", r.calls, want)
	}
}
