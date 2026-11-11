# C-jo-reopen-plan-job-id-checked-under-lock

Base: plugin main 99c11b7a (every path:line below is read on the lane's head, which is that main plus two commits). No `feed.ErrJobExists` on this base (`grep -rn ErrJobExists internal` finds nothing), so a second `job` record for a counted id is written and the fold marks it `ignored`.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. `TestJoRpHeldIDSkipped`: a held J-3 is planted, `task reopen T-07` takes J-4 | HOLDS | `go test -count=1 -run '^TestJoRpHeldIDSkipped$' -v ./internal/cli/` exit 0: `--- PASS: TestJoRpHeldIDSkipped (0.14s)`. Fails on base (reopen_job.go stashed): `the reopen Job is not the open J-4: &{ID:J-3 ... Title:Approve the plan again after the reopen ...}` |
| 2. `TestJoRpStalePick`: closure kept, member posts J-3, closure posts J-4 | HOLDS | `go test -count=1 -run '^TestJoRpStalePick$' -v ./internal/cli/` exit 0: `--- PASS: TestJoRpStalePick (0.23s)`. Fails on base: `job records by id map[J-1:1 J-2:1 J-3:2], want J-3 once (the member's) and J-4 once` |
| 3. `-run Reopen` and the whole cli package | HOLDS | `go test -count=1 -run 'Reopen' -v ./internal/cli/` exit 0: PASS TestErReopenFromJobPostsPlanJobAtTaskEpoch, TestErReopenTypedGlobalLeaseEpochBeatsTaskEpoch, TestReopenMakesApprovedFalseUntilGridApproveAnswersTheNewPlanJob, TestReopenMakesApprovedFalseUntilJobResultAnswersTheNewPlanJob. `go test -count=1 ./internal/cli/` exit 0: `ok  github.com/gridconsole/grid-plugins/internal/cli  66.995s` |
| 4a. `make lint` | HOLDS | `make lint` exit 0 (`go vet ./...`); `gofmt -l internal/cli` prints nothing |
| 4b. `make cross` | HOLDS | exit 0: vet and test-compile for darwin/arm64 and windows/amd64 |
| 4c. `git diff --name-only main` | HOLDS | lists only `internal/cli/jo_reopen_plan_job_id_test.go` and `internal/cli/reopen_job.go` |
| Importers of internal/cli tested | HOLDS | `go list` (Imports, TestImports, XTestImports) names cmd/grid, contract/formats, internal/account, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1` on them exit 0: ok for formats, account, golden, jobgraph, mcp, packaging, question, settings, lib; cmd/grid has no test files |
| New tests fail on base, pass on lane | HOLDS | both failing runs are quoted in rows 1 and 2 |
| No existing test edited or deleted | HOLDS | `git diff --stat main` shows one new test file and reopen_job.go |
| No guard, gate, cap, hold or blocking hook | HOLDS | the change picks a different free id and refuses nothing an agent may do; no new error, exit code or `--json` code |
| Files not to edit (store.go, question.go, deliver_keeper_job.go, receipt.go, internal/orchestrator/, task_write.go) | HOLDS | none in `git diff --name-only main` |
| `*RealPlugin*` tests | N/A | no row names one |
| Commits: one sentence, no attribution | HOLDS | `git log --format=%B 99c11b7a..HEAD` |
| No pushing | HOLDS | a bundle only |

## What changed

- `internal/cli/reopen_job.go:71-115`: the closure `reopenPlanJob` returns no longer posts the id picked before the reopen. It picks again at post time (`reopenJobTop`, :130): one above the highest J-<n> over the body `id` of every `job` entry of `store.ReadOnly(feed.Filter{Type: "job"})`, whatever its verdict, and over the `highest` of `snap.Items` (dotted child ids by their J-<n> part, as before). It posts with `orchestrator.WriteUnless` (:95), whose `done` reads the `job` entries again under .orchestrator.lock (`reopenJobTaken`, :149) and skips the write when one carries the id. A skipped write picks again from a fresh read and posts again; each round sees one more taken id, so the loop ends. A failed read in `done` writes nothing and the closure returns that error. `next.Continues` still names the old plan Job.
- `reopenJobNumber` (:120) is the J-<n> parse; the early part of `reopenPlanJob` (:23-70) is untouched.
- `internal/cli/jo_reopen_plan_job_id_test.go`: `TestJoRpHeldIDSkipped`, `TestJoRpStalePick` and helpers `joRpClosed`, `joRpBody`, `joRpIDs`, `joRpReopenJob`.

## Deviation from the brief

Row 1 gives `feed post --type job --job J-3 --after <ULID> --body ...` with no `--lease`. On this base that exact command exits 3: `grid feed: $: oneOf matched 0 branches: $.lease: expected string` (the record has no lease). The brief said to stop on a refusal; the refusal is a missing flag, not a refusal of the scenario, so the test adds `--lease task`, as `internal/cli/contract_commands_test.go:373` does for a `feed post`. The planted record is then written and read back as `held` (the test checks the verdict).

## NOT VERIFIED

- The skip-and-pick-again loop (a post that lands between `reopenJobTop` and the lock) is not run by any test: the brief's rows make the order by calls, and a write that lands inside that window needs a goroutine or a hook in production code. Row 2 covers a write that lands before the closure's pick; the loop itself is read, not run.
- A `feed post` job record does not take .orchestrator.lock (it goes through `Store.Post`), so a `feed post` of the same id in the window between the lock check and the append is not excluded by this lane; that is the `feed.ErrJobExists` lane's refusal.
- No live screen is involved.
