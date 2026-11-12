# C-wl-repost-missing-first-job: the worker pass posts J-1 for an open task that has no Job

Repo: plugin. Branch lane-C-wl-repost-missing-first-job, from main.

## Base check

On the base, one pass of a Stream over a task folder with a Feed identity and no Job record
(`newStreamHome(t, 1)`, then `h.pass(h.stream(4242, nil))`) printed:

    BASE: spawn lines=0 job records=0

No spawn line and no job record, so the brief's premise holds and the build went ahead.

## Proof table

| Row | Result | Command / place |
|---|---|---|
| 1 TestWlPostsMissingFirstJob | HOLDS | `go test -count=1 -run 'TestWl' -v ./internal/orchestrator/` exit 0: `--- PASS: TestWlPostsMissingFirstJob`. Feed holds exactly `[J-1]`, one spawn line for J-1 (internal/orchestrator/wl_first_job_test.go, TestWlPostsMissingFirstJob). Red with the new step switched off (`if true \|\| s.FirstJob == nil`): `wl_first_job_test.go:79: the Feed holds job records [], want exactly J-1`. |
| 2 TestWlLeavesOthersAlone | HOLDS | same run, `--- PASS: TestWlLeavesOthersAlone`. A task with J-2 holds only J-2; closed and waiting tasks get no job record and no record at all (no handover, so no lease taken); FirstJob was never asked; a Stream with FirstJob nil posts nothing and prints nothing. |
| 3 TestWlOnePostPerTask | HOLDS | same run, `--- PASS: TestWlOnePostPerTask`. Two passes of one Stream plus a pass of a second Stream (pid 5151): each Feed holds exactly one J-1. `kind: setup` in task.md reaches FirstJob as "setup" (asked = `["", "setup"]`). Red with the step off: `task 0 holds job records [], want exactly one J-1`, `FirstJob was asked for kinds [], want ["" "setup"]`. |
| 4 passes + packages | HOLDS | `go test -count=1 -run 'TestPgClosedTaskLaterPassesWriteNothing\|TestWl' -v ./internal/orchestrator/` exit 0, a `--- PASS:` line for each of the three tests and no FAIL. `go test -count=1 ./internal/orchestrator/ ./internal/cli/` exit 0 (orchestrator 181.5s, cli 90.9s), including TestPgClosedTaskLaterPassesWriteNothing and TestTaskNewTwiceEachTaskGetsItsOwnFirstJob (both `--- PASS` in a separate -v run). |
| 5 TestWlFirstRaceJobAlreadyPosted | HOLDS | `go test -count=1 -run 'TestWlFirst' -v ./internal/cli/` exit 0: `--- PASS: TestWlFirstRaceJobAlreadyPosted`, `TestWlFirstRacePassPostedFirst`, `TestWlFirstHeldWithoutJobStillFails`. Red on base behaviour (both new branches in postFirstJob switched off): `wl_first_job_race_test.go:68: a second postFirstJob on a task with J-1 returned: Job J-1 already exists (post 01M76R6PHHXX05P07DT2P616RE); revisions need a new Job`, and `TestWlFirstRacePassPostedFirst`: the pass posted J-1 first and postFirstJob then returned the lease refusal. File restored with `git checkout --` (status clean). |
| 6 lint, cross, diff | HOLDS | `make lint` exit 0 (`go vet ./...`). `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile). `git diff main --stat` names stream.go, worker.go, task_new_first_job.go and the two new test files, plus this report `docs/lanes/C-wl-repost-missing-first-job.md`, which the builder runbook requires of every lane (other lanes commit theirs in docs/lanes). `git diff main --stat -- . ':!docs/lanes'` names the five files only. |
| Packages that import the changed ones | HOLDS | `go list` shows importers of internal/orchestrator or internal/cli: cmd/grid, internal/cli, internal/mcp, internal/migrate, lib. `go test -count=1 ./internal/orchestrator/ ./internal/cli/ ./internal/mcp/ ./internal/migrate/ ./lib/ ./cmd/grid/` exit 0 (cmd/grid has no test files). |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` exit 0 (`walked 39 commands, 39 table entries`; both PASS). `go test -count=1 ./internal/job/lint/` exit 0. The app walk tests (webui-test, TestFvArgvCollects, the grid-server walks, slices Imports) belong to the app repo and do not apply to this plugin lane. |
| New tests fail on base, pass on lane | HOLDS | pasted above (rows 1, 3, 5). On a true base the new tests do not compile (no `Stream.FirstJob` field), so the red runs used the lane code with the new behaviour switched off by one temporary edit per file, restored with `git checkout --`. |
| No test edited or deleted | HOLDS | `git diff main --stat` shows only added lines (451 insertions, 0 deletions). |
| No guard, gate, cap, hold or blocking hook | HOLDS | The step only adds a post when a task has no Job. It adds no refusal, delay or cap on agents. |
| *RealPlugin* tests | n/a | The brief names none. |
| Windows and macOS builds | HOLDS | `make cross` above. |

## What changed

- `internal/orchestrator/stream.go`: new optional field `Stream.FirstJob func(kind string) map[string]any` and
  `Stream.wlFirstJob`, called in `Stream.task` after the snapshot is loaded and before the `hasOpenJob` early
  return. With FirstJob set, no Job record in the snapshot, and task.md state neither closed nor waiting
  (read with `taskfile.ScalarFrontmatter`, as pgClosedRead does), the pass:
  validates the record from `FirstJob(kind)` against the `job-record` schema, takes the task lease (AcquireTask and
  handover, as the pass does), loads the snapshot again under the lease, and if it still holds no Job posts J-1
  with `orchestrator.Write`. It then goes on with the pass, so the same pass assigns J-1. A Job that appeared
  meanwhile (`feed.ErrJobExists`, or Jobs on the second read) is not an error.
- `internal/cli/worker.go`: the Stream literal sets `FirstJob: firstJobOf`.
- `internal/cli/task_new_first_job.go`: `postFirstJob` returns nil when `TakeTask` is refused with
  `lease.ErrTaskHeld`, or `Write` with `feed.ErrJobExists`, and the task's Feed holds a J-1 job record
  (new helper `wlFirstJobPosted`). Otherwise the error is returned as before.
- Tests: `internal/orchestrator/wl_first_job_test.go` (the three named tests plus
  `TestWlHeldLeaseLeftAlone`) and `internal/cli/wl_first_job_race_test.go` (the named test plus
  `TestWlFirstRacePassPostedFirst`, the real race, and `TestWlFirstHeldWithoutJobStillFails`).

## Decisions where the brief was silent

- **A lease held by someone else on a task with no Job is left alone quietly.** The pass prints no skip line
  and tries again next pass. Today that task prints nothing either, and the holder is most likely
  `grid task new` between its two writes. A dead holder is adopted by AcquireTask as always. Other lease errors
  print the usual `lease_failed` skip line.
- **A task with a pending approval receipt (`ReceiptPending`) gets no J-1.** The receipt move posts that task's
  approve person Job in the same pass, so the task is waiting on a person.
- **A task.md that cannot be read, or has no front matter, is left alone.** Posting a Job needs a known state.
- **No new skip code.** An invalid FirstJob record or a failed post prints `tick_failed` (as a failed receipt
  move does), a stale lease `task_held`; the lease taken for the post is given back, so a person is not
  locked out of a task with no Job. The record is validated before the lease is taken, so a bad record does
  not post a handover record every pass.
- **Other states** (queued, running, paused, stuck, reworking, ...) all get J-1, as the brief says: only closed and
  waiting are left alone.

## Open item for the reviewer (concurrency)

A pass that holds the lease and has not posted yet still makes `postFirstJob` return its refusal, and
`grid task new` prints "was created but its first Job was not posted" although the pass posts J-1 right
after. The brief accepts this ("the task starts either way"); the lane does not retry or wait. Tested:
`TestWlFirstHeldWithoutJobStillFails` (lease held, no J-1 on the Feed, the refusal is returned as today).

## NOT VERIFIED on a live screen

Nothing here is UI. Not run, as it is not in this repo or needs a live worker: the app's worker caller
(internal/kernel/worker/job.go:336) reading the new spawn line; the orchestrator runs the cross-repo set.

## Budget

The brief said about 55 calls; the lane used more, about 70, because of the two-file red-on-base proofs, the
walk tests, `make lint` and `make cross`, and one retry after the sandbox refused `rm` and `sed`.

Verdict: DONE
