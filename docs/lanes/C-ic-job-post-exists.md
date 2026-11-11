# C-ic-job-post-exists: report

Status: NOT DONE. Row 5 hit the brief's stop condition. Five tests on main post a second `job` record for J-1 and expect exit 0. The brief says not to change them.

## Proof table

| Row / rule | Verdict | Proof |
| --- | --- | --- |
| 1 TestIcjJobExistsRefused | HOLDS | `go test -count=1 -run '^TestIcj' -v ./internal/feed/` exit 0: `--- PASS: TestIcjJobExistsRefused (0.12s)`. On base it does not build: `internal/feed/icj_job_exists_test.go:69:22: undefined: ErrJobExists` |
| 2 TestIcjOneWinner | HOLDS | same run: `--- PASS: TestIcjOneWinner (0.05s)`. Start barrier at icj_job_exists_test.go:125-136; hook pause and `TryLockExclusive` probe at :152-166; no `time.After`. On base: `icj_job_exists_test.go:152:4: p.afterJobCheck undefined` |
| 3 TestIcjJobPostExistsCLI | HOLDS | `go test -count=1 -run '^TestIcj' -v ./internal/cli/` exit 0: `--- PASS: TestIcjJobPostExistsCLI (0.04s)`. On base: `icj_job_post_exists_test.go:108:35: undefined: feed.JobExistsError` |
| 4 TestIcjUnleasedKeepsFirstJob | HOLDS | same run: `--- PASS: TestIcjUnleasedKeepsFirstJob (0.02s)`. Sets `GRID_LEASE_EPOCH=""` so the post takes the unleased path (orchestrator.go:287). Base: the file does not build (as row 3) |
| 5a TestIcjErrorCode | HOLDS | same run: `--- PASS: TestIcjErrorCode (0.00s)` |
| 5b package run | NOT DONE (stop condition) | `go test -count=1 ./internal/feed/ ./internal/orchestrator/ ./internal/approvefix/ ./internal/mcp/ ./internal/contract/ ./internal/cli/` exit 1. These passed: `ok internal/feed 4.796s`, `ok internal/orchestrator 119.652s`, `ok internal/mcp 1.346s`, `ok internal/contract 2.751s`. internal/cli fails (list below). My grep showed no line for approvefix; NOT VERIFIED whether that package has tests |
| 6 diff and jobid.go | HOLDS | `git diff --name-only main` before this report: contract/README.md, internal/cli/error_codes.go, internal/cli/icj_job_post_exists_test.go, internal/feed/icj_job_exists_test.go, internal/feed/jobid.go, internal/feed/store.go. All are in Touches. This report is the runbook artifact. jobid.go has no build tag; jobLock is at internal/feed/jobid.go:39-53 and uses fslock.OpenNoFollow (:44) and LockExclusive (:48); the Unlock is at store.go:258 |
| Small commits, one sentence, no attribution | HOLDS | `git log --oneline main..lane-C-ic-job-post-exists` |
| New tests fail on base, pass on lane | HOLDS (compile failure only) | Base failure lines are above. Both new files fail to build on base, so I did not run rows 1-4 against base behaviour. The base behaviour comes from the brief's scratch run |
| No guard, gate, cap or hold on agents | HOLDS | Post only refuses a malformed write (a second record for one Job id), store.go:248-265. Nothing is removed |
| Tests of changed packages and importers (`go list`) | NOT DONE | internal/cli fails (row 5b). I did not run `go list` because of the call budget |
| Lint, Windows and macOS builds | NOT DONE | Not run because of the call budget. The orchestrator runs `make cross` on the merge result |
| gofmt | HOLDS | `gofmt -l internal/feed internal/cli contract` printed nothing |
| No clock, sleep or timeout in tests; temp dirs; synthetic people | HOLDS | t.TempDir through testStore and orchestratorHarness; second writer `sam` |
| UI / RealPlugin | n/a | no UI; no RealPlugin test |

## Tests on main that post a second J-1 and expect success (row 5 stop)

Each one runs `job post --type job --job J-1 --body personBody(t, "J-1", nil)` with `orchestratorHarness`. The harness has already posted J-1 at internal/cli/orchestrator_test.go:43. Each test expects exit 0 and now gets `3 Job J-1 already exists (post …); revisions need a new Job`.

- internal/cli/approve_job_test.go:143 (TestApproveWaitsForTheAfterJob, fails at :144)
- internal/cli/fu2_approve_job_waiting_test.go:33 (TestFu2ApproveJobWaitingJSON :70, TestFu2ApproveJobWaitingWithoutJSON :100, TestFu2ApproveJobWaitingRoles owner/member :135, agent :147)
- internal/cli/gl_job_waiting_code_test.go:42 (TestGlJobWaitingCode)

On main, each of these second J-1 posts was either ignored by the fold or, as an unleased fence-0 post, replaced the harness's J-1. That second case is the bug in brief case (b). An owner decision is needed: change these tests to post a free Job id, or keep the old behaviour. I have not changed them.

## What changed

- internal/feed/jobid.go (new): `ErrJobExists`, `JobExistsError{Job, Post}` (Is matches ErrJobExists and ErrInvalidRecord), `jobID`, `jobLock` (`.cache/job.lock`, written like gateLock, with the lock order in its comment) and `checkJobFree`. checkJobFree reads `ReadOnly(Filter{Type:"job"})`, looks only at `counted` entries, and allows a match only for the same record id plus `sameRequest`.
- internal/feed/store.go: hook field `afterJobCheck` (:35-36). In Post, an `else if` after the gate check (:248-265) takes job.lock after the lease lock and before the tenure file, runs the check, then calls the hook. A same-request retry carries on to the tenure scan, which returns the stored record.
- internal/cli/error_codes.go: `CodeJobExists = "job_exists"` and an `errors.Is(err, feed.ErrJobExists)` case.
- contract/README.md: a `job_exists` row added at the end of the stable-code table. On this main there is no `linked_file` row; the table ends at `nothing_to_undo`, so the new row follows that.

## Open items

- Row 5 stop: the five tests above.
- question.go:42-56 and migrate/receipt.go:201-216 read the snapshot outside any lock. Two of them running at once can pick the same Job id; the loser now gets exit 3 (job_exists) instead of a silently ignored record. Left as is, as the brief says.
- `s.stored` is not called in checkJobFree, because Read already puts a spilled body back (read.go:67-90). sameRequest therefore compares whole bodies.
- Only `counted` entries block a post. A J-id record that is `held` (waiting for an after record) does not block a second post.
- NOT VERIFIED: nothing here needs a live screen.

NOT DONE
