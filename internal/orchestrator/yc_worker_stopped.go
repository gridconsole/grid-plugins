package orchestrator

import "encoding/json"

// ycWorkerStoppedCode is the worker-exit code a worker sends with reason
// cancelled when it stopped or restarted on its own (grid-worker stop, a
// shutdown, an update). The Job did not fail and nobody cancelled it.
const ycWorkerStoppedCode = "worker_stopped"

// ycWorkerStopped reports whether a folded worker-exit code is the JSON string
// worker_stopped. It decodes as gqCauseText does: a number, no code or any other
// string is not it, so every other cancelled code keeps the final Cancelled path.
func ycWorkerStopped(raw json.RawMessage) bool {
	var s string
	return len(raw) > 0 && json.Unmarshal(raw, &s) == nil && s == ycWorkerStoppedCode
}
