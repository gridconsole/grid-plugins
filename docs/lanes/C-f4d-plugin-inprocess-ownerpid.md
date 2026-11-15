# C-f4d-plugin-inprocess-ownerpid: proof table

Verdict: the brief's premise does not hold on main. An in-process `task new` does not leave J-1 or the task lease held by the server's pid. `postFirstJob` takes the task lease only to post J-1 and gives it back before it returns (`internal/cli/task_new_first_job.go:118`, `defer ts.ReleaseTask(w)`). It never writes a J-1 lease. So the pid in `os.Getpid()` (`:94`) is on the lease file only while `task new` runs. After the call the holder and pid are null, whichever process ran it. The lane therefore has no production change: neither option of step 2 is needed.

Evidence that this was already known:
- `lib/ih_call_test.go:115-119` (TestIhClOwnerOnTaskNew) says the same thing for a Call with OwnerPID.
- The app lane C-f4b observed it on a live run. Its doc, line 7, records `task.json: {"lease":"task","holder":null,...}` right after `task new` and no J-1.json. Its open item at line 53 ("the first Job's lease owner is the grid-server pid unless the Call carries OwnerPID") is a reading of `:94-97`, not an observation.

| Row | State | Proof |
|---|---|---|
| A1 New test fails on main and passes on the lane | NOT DONE | The new tests pass on main, because the behaviour the brief asks for is already there. The lane changes no production code, so the lane run is also the main run. `go test -count=1 -run TestF4dInProcessTaskNewFirstJobTakenByWorker -v ./internal/cli/` gives `--- PASS` (exit 0). `go test -count=1 -run TestF4dTaskNewNoOwnerLeavesLeaseFree -v ./lib/` gives `--- PASS` (exit 0). No red test can be written without inventing a bug. |
| A2 `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`) |
| A2 Package tests of internal/cli, and the binary-path first-Job test unchanged | HOLDS | `go test -count=1 ./internal/cli/ ./lib/ ./cmd/grid/ ./contract/` exit 0. `task_new_first_job.go` and `task_new_first_job_test.go` are untouched (`git diff origin/main --stat` lists only the two new test files and this doc). |
| A3 This doc with rows and Open items | HOLDS | this file |
| Step 3 test: in-process `task new` from a long-lived process, then the worker takes J-1 | HOLDS | `internal/cli/f4d_inprocess_first_job_test.go`. Task new runs in the test process with OwnerPID 0. Both leases are then free. A stream pass as another live process (the parent pid, as a separate `grid worker next`) emits `spawn` J-1 and holds the task lease. `lib/f4d_task_new_no_owner_test.go` does the same through `lib.Run` with the Call the app's libCall builds (no OwnerPID). |
| Importers tested | HOLDS | `go list` with imports: internal/cli is imported by cmd/grid and lib. lib is imported only by the tests `contract/t12_lib_call_test.go` and `lib/hs_inprocess_test.go`. All four packages are in the run above. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0. `GOOS=darwin go build ./...` exit 0. `GOOS=windows go test -run XXX -exec=/bin/true ./internal/cli/ ./lib/` exit 0, so the tests compile on Windows. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0. `go test -count=1 ./internal/job/lint/` exit 0. |
| Merge of origin/main before the last proof run | HOLDS | `git fetch origin && git merge origin/main`: already up to date. The head is the lane commit on 842493ba. |
| No guard, gate, cap, hold or blocking hook | HOLDS | test files only |
| No existing test edited or deleted | HOLDS | only new files |
| Whole-module test, cross-repo set | not run, not asked | run by the orchestrator at landing |
| RealPlugin tests | not run, not asked | the brief names none |

## What changed
- `internal/cli/f4d_inprocess_first_job_test.go`: new test. In-process `task new`, then a worker pass from another live pid takes J-1.
- `lib/f4d_task_new_no_owner_test.go`: new test. `lib.Run` `task new` with no OwnerPID leaves the task lease with holder and pid null, and no J-1 lease file.

## Open items
- No app change is needed for this brief. The app's libCall can leave OwnerPID unset for `task new`. The C-f4b doc's line 53 item can be closed with this lane as its proof.
- Seen while testing, the same on both paths, not fixed here. On Linux, `migrate.ProcessStart` returns `proc:<ticks>`, not an RFC3339 time. So `postFirstJob` (`task_new_first_job.go:99-104`) always falls back to `time.Now()` as the pid start of its short tenure. The tenure is released before the call returns, so nothing reads it. A later brief may switch it to `session.ProcessStart`.

## NOT VERIFIED on a live screen
- Nothing. The lane has no UI.

NOT DONE
