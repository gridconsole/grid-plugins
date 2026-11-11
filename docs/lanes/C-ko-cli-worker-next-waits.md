# C-ko-cli-worker-next-waits

The two `grid worker next` process tests wait on spawn lines now, not on a 1.5 s sleep or a 100 ms poll.

Base: `21f0273607ae73e7aa8494035743afecdd19f8fa` (lane head before this work; `git merge main` says "Already up to date").
Base check: the two sleeps were there at the lines the brief names (`git grep -n -E 'time\.Sleep'` printed `approve_record_test.go:403` and `worker_next_test.go:149`), so both rows were built.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. no `time.Sleep` in the two files | HOLDS | `git grep -n -E 'time\.Sleep' -- internal/cli/worker_next_test.go internal/cli/approve_record_test.go` prints nothing (`... \| wc -l` prints 0); git grep exits 1 on no match, the tool shows no output |
| 2. both tests PASS, no SKIP | HOLDS | `go test -count=1 -v -run 'TestWorkerNextStreamsSpawnLinesUntilSIGINT\|TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead' ./internal/cli/` exit 0: `--- PASS: TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead (1.67s)`, `--- PASS: TestWorkerNextStreamsSpawnLinesUntilSIGINT (2.89s)` |
| 3. `go test -count=3 ./internal/cli/` | HOLDS | exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  267.796s` |
| 4. diff names only the two files | HOLDS | `git diff main --stat`: `internal/cli/approve_record_test.go`, `internal/cli/worker_next_test.go`; plus this report `docs/lanes/C-ko-cli-worker-next-waits.md` (the runbook asks for it) |
| 5. walk tests | HOLDS | after `git merge main` (up to date): `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok ... 0.144s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok ... 0.382s`) |
| 6a. `make lint` | HOLDS | exit 0 (`go vet ./...`, gofmt check printed nothing) |
| 6b. `make cross` | HOLDS | exit 0 (the background task reported "completed (exit code 0)"): darwin/arm64 and windows/amd64 vet and test compile of every package |
| Tests of changed package and importers | HOLDS | only `_test.go` files of `internal/cli` changed, so no importer sees a change; run anyway. Importers from `go list` (own Imports, TestImports, XTestImports contain `internal/cli`): `cmd/grid`, `contract/formats`, `internal/account`, `internal/braincmd`, `internal/golden`, `internal/jobgraph`, `internal/mcp`, `internal/packaging`, `internal/question`, `internal/settings`, `lib`. `go test -count=1` of all of them exit 0 (`cmd/grid` has no test files) |
| No file outside Touches, no production file | HOLDS | diff above |
| No sleep, poll, elapsed-time assertion or timeout as proof | HOLDS | the only clock left is `time.After(30 * time.Second)` in `koWnSpawn`, the failure case of a select whose other case is the line (`worker_next_test.go`, `koWnSpawn`) |
| No guard, gate, cap or limit on agents | HOLDS | tests only |
| `*RealPlugin*` tests | not applicable | neither test is named RealPlugin; both run the test binary itself as the child (`TestWorkerNextProcess`), no plugin build needed |
| Mismatch proof | not asked | no row asks for it |
| New tests fail on base | n/a | these are rewrites of two existing tests, not new tests; on base both pass because they sleep. The new waits are shown to fail without their event below (see "Failure side") |

## What changed

`internal/cli/worker_next_test.go`
- `koWnStart` starts the real `grid worker next` child (the test binary running `TestWorkerNextProcess`) with its stdout on an OS pipe, and returns the child and a channel of its stdout lines. `koWnSpawn` reads one line from that channel and returns its `spawn` object; its only clock is the 30 s failure case of the select.
- `TestWorkerNextStreamsSpawnLinesUntilSIGINT`: the 1.5 s sleep and the `cmd.ProcessState != nil` read (ProcessState is only set by Wait, so it proved nothing) are gone. A goroutine Waits on the child and sends on `exited`. After the two spawn lines and the lease checks, a third task folder `T-09-third` (new ULID) is built under a name the stream does not list (`stage-T-09-third`), its J-1 posted with `ownerPost`, then renamed into place; its spawn line is read from `lines`, and `exited` must not have a value when it arrives (select with `default`). The rename keeps the stream from meeting the task while the owner holds its lease, which would print a `skip` line. SIGINT and exit 130 are unchanged, now read from `exited`.

`internal/cli/approve_record_test.go`
- `TestApproveRecordWorkerPassMovesTheReceiptOfATaskOnlyRead`: the 30 s poll loop with `time.Sleep(100ms)` is gone. A second task `T-08-second` with a posted J-1 is added before the child starts. One pass walks the task folders in `os.ReadDir` order (`workerTaskDirs`, `worker.go:226-247`), so `T-08-second`'s spawn line comes after the pass handled `T-07-example` (receipt moved). After that line the approve Job and the accept are read once, with the same assertions as before. The move did show in the same pass: no NOT DONE row.
- Imports `os/exec`, `strconv`, `time` dropped (unused), `bytes` added.

## Failure side

To show the third-line wait is the event the test depends on, I removed the `ownerPost(t, stage, koWnThirdTaskID, "J-1")` line for one run, so no third spawn line can come (a test-file edit, reverted with `git checkout -- internal/cli/worker_next_test.go`; `git status --short` then showed only the untracked report):

```
--- FAIL: TestWorkerNextStreamsSpawnLinesUntilSIGINT (30.46s)
    worker_next_test.go:187: no spawn line within 30s
FAIL	github.com/gridconsole/grid-plugins/internal/cli	30.471s
```

The committed test passes (row 2). For the approve test the single read of the Feed has no failure run of its own: if the pass had not moved the receipt before T-08-second's spawn line, `arApproveJobs` would be empty and the existing `approve Jobs %v` assertion fails; I saw no such case, the move showed in the first pass in every run (row 2 once, row 3 three times).

## NOT VERIFIED on a live screen

Nothing here has a screen: both tests are Go tests that run a child process.
