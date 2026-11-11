# Q-render-brain-given-history

Base: main at 3ccec5cb (lane branch point). Head: see the report line from the builder.

## Proof table

| Row | Result |
|---|---|
| 1. Test fails on base, naming the missing line | HOLDS. Tests committed first (5ab81dbb) on base 3ccec5cb: `go test -count=1 -run 'TestRender(Logs\|Failure\|Changed\|Without\|BrainGiven)' ./internal/job/jobcmd/` failed: `render_brain_given_test.go:91: want exactly one brain given line, got 0 in []` (TestRenderLogsBrainGivenOnce, link `brain:project/deploy`); `:145: want between 1 and 8 brain given lines, got 0: []`; `:222: want one brain given line, got []`. |
| 2. One line, event `brain given`, detail `brain:project/deploy`, who `grid job render`, via Job id | HOLDS. TestRenderLogsBrainGivenOnce asserts the line ends ` · brain given · brain:project/deploy · by grid job render · via J-1` and that there is exactly one. Writer: internal/job/jobcmd/command.go (the `brainGiven` loop after `saveReport`). |
| 3. Cut note, personal home note, repo/task file get no line | HOLDS. TestRenderLogsNoLineForCutHomeAndFileSources: nine project notes plus `brain:home/my-habits`; the cut note (read from `not_rendered` in inputs.json), the home note, research.md and intent.md get no line. Filter: `brainGiven` skips PointerOnly, `NotRendered`, non-`brain:` pointers, and brain pointers listed only under `read:` or intent. |
| 4. Same Job again adds no line; failed render adds no line | HOLDS. `saveReport` (save.go) reports `wrote=false` on the `same()` path; the second render in TestRenderLogsBrainGivenOnce leaves one line. TestRenderFailureLogsNoBrainGiven (resolver error on missing note, exit 1) and TestRenderChangedSavedInputsLogsNoBrainGiven (saved inputs differ, exit 1) add none. The lint-error and generation-changed paths return before `saveReport`, so they cannot reach the log; they have no test of their own. |
| 5. No history.md: render exits 0 | HOLDS. TestRenderWithoutHistoryFileSucceeds removes history.md, render exit 0, asserts nothing about the file afterwards. A history write error is printed to stderr and the exit code stays 0 (command.go). |
| 6. Lock order, concurrent AppendHistory, -race | HOLDS. TestRenderBrainGivenAmongConcurrentHistoryAppends: 4 goroutines x 10 appends while render runs; every line lands once, one `brain given` line, the test finishes. AppendHistory runs after `saveReport` returns, with no lock held. |
| 7. `go test -count=1 -race ./internal/job/jobcmd/` and `go vet ./internal/job/jobcmd/` | HOLDS. Test: ok, exit 0 (4.6s). Vet: exit 0. |
| Packages importing the changed one | HOLDS. `go list` importers of internal/job/jobcmd: internal/cli (and cmd/grid via cli). `go test -count=1 -race ./internal/cli/ ./internal/job/...`: all ok, exit 0; `go test ./cmd/grid/`: no test files. |
| Lint | HOLDS. `make lint` (go vet ./...) exit 0; `gofmt -l internal/job` empty after `gofmt -w`. |
| Windows and macOS builds | HOLDS. `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| No test edited or deleted | HOLDS. Only the new file internal/job/jobcmd/render_brain_given_test.go was added; `save` keeps its signature (wrapper over `saveReport`) so existing tests are untouched. |
| No guard, gate, cap or hook added | HOLDS. Logging only; an append error never fails the render. |

## What changed

- internal/job/jobcmd/save.go: `saveReport` returns whether this call wrote the files; `save` wraps it.
- internal/job/jobcmd/command.go: after a successful `saveReport` that wrote, one `task.AppendHistory(task, "brain given", <note id>, "grid job render", <job id>)` per rendered project brain note; helper `brainGiven`. An anchor on a pointer is stripped so a note appears once.
- internal/job/jobcmd/render_brain_given_test.go: the new tests.

## Notes for landing

- O-render-brain-recall edits the same file. Recalled notes arrive in `Inputs.Sources` like any other candidate, so `brainGiven` logs them too without a change; the conflict is only textual, around the Render call.
- A render whose `save` rename race is lost to another process (same files) logs nothing; the process that wrote logs.

## NOT VERIFIED on a live screen

Nothing in this lane is UI. The scorecard count of `brain given` against `brain read` is not checked against a live task.
