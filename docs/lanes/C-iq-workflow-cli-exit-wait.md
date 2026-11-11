# C-iq-workflow-cli-exit-wait

Base: ee8a96c1 (lane-Pint102 merge on main). Both 2 s contexts were still in internal/workflow/files_harden_test.go (:124 and :177 on this base), so the brief applies.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. no `time.Second` in the two tests; `go test -run '^TestWorkflowHarden'` passes | HOLDS | `git grep -n 'time.Second' -- internal/workflow/files_harden_test.go` prints nothing (exit 1, no match). `go test -count=1 -v -run '^(TestWorkflowHarden\|TestIqWf)' ./internal/workflow/` exit 0: PASS TestWorkflowHardenLeafErrors, BadProjectCLI, FIFO, OpenChild, SocketAndDevice, WrappedErrors, TestIqWfHangDeadline, TestIqWfStoppedChild. |
| 2. mutation symlink | HOLDS | `continue` in place of `return nil, errSymlink` (files.go:233): `go test -count=1 -run '^TestWorkflowHardenBadProjectCLI$'` FAIL: `--- FAIL: TestWorkflowHardenBadProjectCLI/symlink` and `files_harden_test.go:135: errors = [b/workflows.md: cannot read]; want b/workflows.md: symlink refused`. Reverted with `git checkout -- internal/workflow/files.go`. |
| 3. mutation FIFO, `before` subtest | NOT AS WRITTEN, holds on `after-lstat` | The brief's `before` subtest cannot fail under this mutation: `openSource` lstats the path first and returns errNotRegular for a FIFO that is already there (files.go:257-259), so the open never runs. With `O_NONBLOCK` dropped, `-run '^TestWorkflowHardenFIFO$/^before$'` printed `ok`. Also, dropping `\|syscall.O_NONBLOCK` literally leaves `syscall` unused and the package does not compile; I used `os.O_RDONLY\|syscall.O_RDONLY`. The subtest that opens a FIFO is `after-lstat` (the FIFO replaces the file between lstat and open): `go test -count=1 -timeout 90s -run '^TestWorkflowHardenFIFO$/^after-lstat$'` FAIL after 81.10 s (the hang context: 90 s less a tenth): `--- FAIL: TestWorkflowHardenFIFO/after-lstat (81.10s)` / `files_harden_test.go:179: FIFO read blocked`. This is the test's own line, not a go test panic. Reverted with `git checkout -- internal/workflow/open_unix.go`; `git status --short` clean. |
| 4. busy machine, no timing | HOLDS | `TestIqWfStoppedChild` in iq_wait_unix_test.go (first line `//go:build unix`): SIGSTOP right after Start, SIGCONT only after `waiting` is closed and the hang context is live; run exits 0 with 2 files and 2 templates, no errors. PASS (2.57 s, in the run listed under row 1). |
| 5. hang deadline is the test's own | HOLDS | `TestIqWfHangDeadline` asserts deadline values: not before `t.Deadline()` less one minute, not after `t.Deadline()`; with `-timeout 0` it asserts the context has no deadline. PASS (row 1 run). |
| 6. diff names only the three files; package test ok; lint 0 | HOLDS (plus this report) | `git diff main --stat` on the code commit: files_harden_test.go, iq_wait_test.go, iq_wait_unix_test.go only. This report is added by a second commit as the runbook requires. `go test -count=1 ./internal/workflow/` ok (26.6 s). `make lint` (`go vet ./...`) exit 0. `gofmt -l internal/workflow` empty. |
| Importers tested | HOLDS | `go list` importers of internal/workflow: internal/cli, internal/feed, internal/task (and itself). `go test -count=1 ./internal/workflow/ ./internal/cli/ ./internal/feed/ ./internal/task/` exit 0: ok workflow 26.6 s, cli 214.1 s, feed 6.0 s, task 10.8 s. |
| Windows and macOS builds | HOLDS | `make cross` exit 0: vet and test-compile for darwin/arm64 and windows/amd64, all packages ok. |
| New tests fail on base | NOT APPLICABLE as base-fail | The two new tests exercise helpers that do not exist on base, so they cannot compile there. The behaviour change is proven by the mutation runs above (rows 2 and 3), which fail on the test's own message. |
| No guard/gate/cap on agents; no existing test edited to pass; nothing deleted | HOLDS | Only test code changed; no test deleted or skipped. Edits inside the two named tests replace the 2 s contexts and drop the unused `context` and `time` imports. The `FIFO read blocked` message lost "for two seconds" as the brief says. |
| RealPlugin tests | NOT APPLICABLE | No row names one. |

## What changed

- `iq_wait_test.go`: `iqWfHangCtx` (test deadline less min(tenth of time left, 1 min); never ends without a deadline), `iqWfRun` (start, close `waiting` just before `cmd.Wait`, return output and error; optional `afterStart` hook for the stopped-child test), `TestIqWfHangDeadline`.
- `iq_wait_unix_test.go`: `TestIqWfStoppedChild`.
- `files_harden_test.go`: BadProjectCLI and FIFO use `iqWfHangCtx` and `iqWfRun`; FIFO fails with "FIFO read blocked" when the hang context ends. The child (`TestWorkflowHardenOpenChild`) is unchanged.

## Notes

- `iqWfRun` takes the hook as a trailing variadic so the three-argument call in the brief still works.
- A hang in the FIFO test now takes about 90% of the go test timeout before it reports (81 s at `-timeout 90s`), the cost of having no fixed deadline.

## NOT VERIFIED on a live screen

Nothing here is a UI row.
