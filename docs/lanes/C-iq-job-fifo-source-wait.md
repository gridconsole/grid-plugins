# C-iq-job-fifo-source-wait

Base: main = ee8a96c1 (the brief names 91fadeff; the 1 s `time.After` waits were still at source_fifo_unix_test.go:30 and :38, so the work applies).

## Proof table

| Row | Result |
|---|---|
| 1. no `time.After` in source_fifo_unix_test.go; TestSourceFIFORefused passes | HOLDS. `git grep -n 'time.After' -- internal/job/source_fifo_unix_test.go` prints nothing (exit 1, no match). `go test -count=1 -v -run '^(TestSourceFIFORefused\|TestIqFifo)' ./internal/job/` exit 0, `--- PASS: TestSourceFIFORefused` with repo:, prototype:, task:. |
| 2. mutation: drop O_NONBLOCK | HOLDS. Replaced with `os.O_RDONLY\|syscall.O_RDONLY` (keeps the import used). `go test -count=1 -timeout 90s -run '^TestSourceFIFORefused$/^repo:$' ./internal/job/` FAIL: `source_fifo_unix_test.go:41: Resolve blocked on FIFO for more than 1s` (this was the old message; the test failed itself after 81.08s, no timeout panic, no "reader did not exit after cleanup", so the release path ran). The message was then changed to "Resolve blocked on FIFO" (the 1s was no longer true). Reverted with `git checkout -- internal/job/source_open_unix.go`. |
| 3. mutation: FIFO not refused | HOLDS. Dropped `\|\| !info.Mode().IsRegular()` at source.go:234; same command FAIL: `source_fifo_unix_test.go:28: want regular-file refusal, got <nil>`. Reverted with `git checkout -- internal/job/source.go`. |
| 4. TestIqFifoLateResolve | HOLDS. internal/job/iq_fifo_wait_unix_test.go:34-80: the Resolve goroutine waits on `waiting`, closed just before the select; it checks `hang.Err()` is nil, then resolves. PASS for repo:, prototype:, task: (see the run in row 1). On base it does not compile (`iqFifoHangCtx` is undefined), so it fails there. |
| 5. TestIqFifoHangDeadline | HOLDS. iq_fifo_wait_unix_test.go:82-98 compares deadline values only; PASS. |
| 6. diff names two files; package test; lint | HOLDS. `git diff main --stat` (without the mutation): iq_fifo_wait_unix_test.go and source_fifo_unix_test.go only, plus this report once committed. `go test -count=1 ./internal/job/` ok (exit 0). `make lint` exit 0 (`go vet ./...`). `gofmt -l internal/job/` is empty. |

Rules:

| Rule | Result |
|---|---|
| Importers of internal/job tested | HOLDS. Direct importers from `go list`: cli, hook, job/jobcmd, job/lint, jobgraph, lease, migrate, orchestrator, ports. `go test -count=1` on all nine: ok, exit 0. |
| Windows and macOS builds | HOLDS. `GOOS=windows go build ./...` exit 0, `GOOS=darwin go build ./...` exit 0, and `go vet ./internal/job/` on each exit 0 (the tests are `//go:build unix`, so darwin compiles them). |
| No production file changed | HOLDS. source.go and source_open_unix.go are clean (`git diff main --stat`). |
| No guard, gate, cap or hold on agents | HOLDS. Test-only change. |
| No existing test edited to pass | HOLDS. TestSourceFIFORefused: only the two `time.After` cases, the unused `time` import, the hang context line and the message text changed. The assertion at :27-29 is unchanged. |
| No deleted tests, no attribution lines | HOLDS. Nothing deleted; commit messages carry no attribution. |
| *RealPlugin* tests | Not applicable, no row names one. |

## What changed

- New `iqFifoHangCtx(t)`: deadline is `t.Deadline()` less min(one tenth of the time left, one minute); it never ends when there is no test deadline.
- TestSourceFIFORefused selects on `done` and `hang.Done()`; the release path waits under a fresh `iqFifoHangCtx`.
- New TestIqFifoLateResolve and TestIqFifoHangDeadline.

## Not verified on a live screen

Nothing here is a UI row. Not run: a loaded machine; the late start is covered only by the channel ordering, not by load.
