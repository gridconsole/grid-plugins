package cli

import "github.com/gridconsole/grid-plugins/internal/worker"

// vqAppRunning says whether the Grid app runs on the Grid home. It reads the two signs
// grid open reads (open_start.go state): another handle holds worker.lock (the worker
// package's presence "reported"), or app-url names an address something listens on.
// Neither is a PID probe. A worker list that fails (no lock support, a worker.lock that
// is not a regular file) says nothing about the worker, so it does not count as running.
//
// It is a look at this moment: an app started after it is not seen.
func vqAppRunning(env Env) bool {
	if entries, err := (worker.Store{GridHome: env.GridHome}).List(); err == nil {
		for _, e := range entries {
			if e.Presence == "reported" {
				return true
			}
		}
	}
	base := appBaseURL(env)
	return base != "" && listening(base)
}
