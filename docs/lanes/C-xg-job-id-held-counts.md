# C-xg-job-id-held-counts

Verdict: CLOSED BY C-xg2-ioq-fixture-raw-append: the fixture of `TestIoqOldFeedPairFoldsAsToday` is rebuilt with a raw append and the change carried unchanged. At C-xg: the change is built and rows 1 to 4 and 6 hold. One row was open: `TestIoqOldFeedPairFoldsAsToday` in `internal/cli` goes red on this lane and is green on base. Fixing it means editing an existing test outside the brief's Touches line, which the rules forbid, so I stopped and name it here.

Base: lane branched from main at 103b9157 (the brief quotes 44c7de5b; main moved). `checkJobFree` (jobid.go:55-73 on base) skipped every entry whose Verdict is not "counted": `if entry.Verdict != "counted" || jobID(entry.Record) != id { continue }`. read.go:75 (`e.Verdict = "held"`, "waiting for body") and read.go:127 (`e.Verdict = "held"`, "waiting for record") are unchanged. The base check did not already count held entries, so the lane applies.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. TestXgHeldJobIDRefused | HOLDS | `go test -count=1 -v ./internal/feed/ -run 'TestXg\|TestIcj'` exit 0: `--- PASS: TestXgHeldJobIDRefused`. Asserts A's J-7 reads "held", B's post satisfies `errors.Is(err, ErrJobExists)` and `ErrInvalidRecord`, and `JobExistsError.Post` is A's record id. On base B's post returns nil (below). |
| 2. TestXgHeldRetryAllowed | HOLDS | `--- PASS: TestXgHeldRetryAllowed`. The same request (same record id, body, After) returns A's record with the same Seq and At and no error. It also asserts that the same Job id under a new record id from the same writer is refused. |
| 3. TestXgHeldThenLands | HOLDS | `--- PASS: TestXgHeldThenLands`. After the refusal and the retry, the awaited record is posted; `s.Read` shows exactly one J-7 job record, A's, verdict counted, nothing waiting. |
| 4. TestXgCountedStillRefused | HOLDS | `--- PASS: TestXgCountedStillRefused`. It passes on base as well, by design (the brief says "as today"): it guards the counted case against regression and does not bite on base. Base run: four of the five Xg tests fail, this one passes. |
| 5a. `go test -count=1 ./internal/feed/ ./internal/orchestrator/` | HOLDS, with one caveat | I ran them inside one combined invocation of the changed package and every importer (list below). That invocation exited 1 only because of `internal/cli`. Its lines: `ok internal/feed 40.916s`, `ok internal/orchestrator 205.480s`. I did not rerun the two alone. |
| 5b. `-v -run TestXg` PASS line | HOLDS | `--- PASS: TestXgHeldJobIDRefused` and the four other TestXg tests, run with `-v -run 'TestXg\|TestIcj'`, exit 0. TestIcjJobExistsRefused and TestIcjOneWinner pass in the same run. |
| 5c. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0: `ok internal/cli 0.127s` |
| 5d. `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0: `ok internal/job/lint 0.358s` |
| 6a. `make lint` | HOLDS | exit 0 (`go vet ./...`, no output). |
| 6b. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, every package `ok`. |
| 6c. `git diff main --stat` names only jobid.go and the new test file | HOLDS | Run before this doc was added: `internal/feed/jobid.go \| 12 ++--` and `internal/feed/xg_held_job_id_test.go \| 131 +++`, 2 files changed. This doc is added in a later commit, as the runbook asks. |
| Tests of the packages that import feed | CLOSED BY C-xg2-ioq-fixture-raw-append: the fixture is rebuilt with a raw append | `TestIoqOldFeedPairFoldsAsToday` in `internal/cli` is red on the lane (details below). Every other package in the list is ok. |
| Walk tests: plugin set | HOLDS | 5c and 5d. The app-only walk tests do not apply to this repo. |
| New tests fail on base and pass on lane | HOLDS | Base failures below; lane passes in rows 1 to 4. |
| `*RealPlugin*` tests | not applicable | The lane touches `internal/feed` only and the brief names no RealPlugin rig. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change widens one refusal that already exists: a Job post whose id is taken gets `ErrJobExists`, and it now also does so when the taking record is held. Nothing is added on an agent's reads or its way of working. |

### The NOT DONE row

`go test -count=1 ./internal/cli` on the lane (the failing line is the plugin's own refusal, so the post went through `Store.Post`):

```
--- FAIL: TestIoqOldFeedPairFoldsAsToday (0.01s)
    ioq_question_ask_free_job_id_test.go:142: post J-2 3: grid feed: Job J-2 already exists (post 01M76R2C25HJ8M1QPFNDY2Q1RR); revisions need a new Job
FAIL	github.com/gridconsole/grid-plugins/internal/cli	133.610s
```

With the lane change stashed (base), `go test -count=1 ./internal/cli -run TestIoq` gave `ok internal/cli 0.355s`, so the lane causes the red.

Why: `internal/cli/ioq_question_ask_free_job_id_test.go:141-142` builds an "old Feed that already holds the pair" by posting a held J-2 (it waits for `ioqUnknownRef`) and then a second J-2 through the CLI post path. That is the exact pair this brief makes impossible to create through Post. The behaviour the test guards, that the fold keeps the lower-seq record and ignores the later one, is not removed. Only the way the fixture is built has to change: write the pair with raw appends, as `TestIcjJobExistsRefused` does for J-9 (icj_job_exists_test.go:81-105). That edit is in `internal/cli`, outside Touches, and it changes an existing test, so it is not made here.

The other `ioq` tests (`TestIoqAskSkipsHeldJobID`, `TestIoqHeldJobResolvesBesideQuestionJob`) pass, because they pick J-3 around the held J-2.

### Packages that import feed (go list -f Deps), all tested in one run

cmd/grid, approvefix, braincmd, cli, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, ports, question, signal, task, updates, usage, lib. All `ok` or "no test files" except `internal/cli` (one red test).

### Base failures of the new tests

`go test -count=1 ./internal/feed/ -run TestXg` on base code, exit 1:

```
--- FAIL: TestXgHeldJobIDRefused (0.02s)
    xg_held_job_id_test.go:60: post = <nil>, want ErrJobExists and ErrInvalidRecord
--- FAIL: TestXgHeldRetryAllowed (0.01s)
    xg_held_job_id_test.go:76: post = <nil>, want ErrJobExists and ErrInvalidRecord
--- FAIL: TestXgHeldThenLands (0.00s)
    xg_held_job_id_test.go:87: post = <nil>, want ErrJobExists and ErrInvalidRecord
--- FAIL: TestXgHeldVsHeldRefused (0.00s)
    xg_held_job_id_test.go:127: post = <nil>, want ErrJobExists and ErrInvalidRecord
```

On base B's J-7 post returned nil, so the Feed ended up with two J-7 records.

## What changed

- `internal/feed/jobid.go`: `checkJobFree` refuses a matching entry whose Verdict is "counted" or "held". The same-request retry exemption is unchanged. The comment says so, including that a record held for a spilled body cannot match. Lock order and `job.lock` handling are unchanged: the check still runs under `job.lock` before the append.
- `internal/feed/xg_held_job_id_test.go` (new): rows 1 to 4 plus `TestXgHeldVsHeldRefused` (a second held record for a held Job id is refused). Helpers start with `xg` and reuse `testStore`, `testWriter`, `mustPost`, `icjJob`, `icjOtherWriter` and `icjJobRecords`.

## Open items

- Fix for the red test: rebuild the pair in `TestIoqOldFeedPairFoldsAsToday` with raw appends, or retire it with a note that the fold behaviour is covered elsewhere. This is a sibling `internal/cli` brief, and it should land together with this lane.
- A Job record held for its spilled body (read.go:75, "waiting for body") has only a `ref` body, so `jobID` returns "" and `checkJobFree` cannot match it. The brief's rows cover only the `after`/`re` case. A fix would need the envelope `job` field, which is a different data-format question. Backlog note.
- Concurrency: the check still runs under `job.lock` and Read takes no file locks, so two writers racing on one Job id still serialise on the lock and one sees the other's record. I did not add a new race test for held records; `TestIcjOneWinner` covers the lock for counted ones.

## NOT VERIFIED on a live screen

Nothing here is a UI change. The Judge's pick (`nextTopID` in the app) was not run against the new refusal; the brief says it counts held records already and never meets it.
