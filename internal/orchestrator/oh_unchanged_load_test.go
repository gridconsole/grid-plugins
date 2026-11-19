package orchestrator

import "testing"

// A pass over an open task whose Feed folder has not changed since the last pass
// does not read the Feed files again. The proof is the reader lock, as in
// oh_quiet_task_test.go: the test holds it, and a pass that reads would wait for it.
func TestOhUlUnchangedOpenTaskIsNotReread(t *testing.T) {
	h := newStreamHome(t, 1)
	h.post(0, "J-1")
	s := h.stream(4242, nil)
	h.pass(s) // spawns J-1
	h.pass(s) // settles: nothing new to print
	h.lines()
	l := ohSqHold(t, h, []int{0})
	if read := l.pass(s); len(read) != 0 {
		t.Fatalf("the pass read the unchanged task %v", read)
	}
	if lines := h.lines(); len(lines) != 0 {
		t.Fatalf("the quiet pass printed %v", lines)
	}
}
