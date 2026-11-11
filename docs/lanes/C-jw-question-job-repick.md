# C-jw-question-job-repick

A person's `question ask` picks a new Job id when the one it picked was taken before its post. The lane starts from lane-C-jd-ic-job-post-exists-fix (stacked, merged in at 67e65dc3; it is not on main yet). `git merge main` says "Already up to date" at the start and again before the last proof run.

## Base check

- feed/jobid.go:12-13 on the jd-ic lane: `// ErrJobExists marks a Job post whose Job id the Feed already holds.` and `var ErrJobExists = errors.New("job exists")`. `JobExistsError.Is` (:23-25) matches `ErrJobExists` and `ErrInvalidRecord`.
- cli/question.go on the base (before this lane): `postQuestionJob` read every job record with `f.ReadOnly` (:45), took `top+1` (:55), then posted once: `f.Post(in, lease)` for the lease branch (:60) and `orchestrator.PostPerson(f, person, in)` for the person branch (:62). Any error was returned as it was; nothing tested `errors.Is(err, feed.ErrJobExists)`. So it did not re-pick, and the brief's stop condition did not apply.

## Proof table

| Row | Status | Proof |
| --- | --- | --- |
| 1. Person branch re-picks | HOLDS | `TestJwRpPersonAskRepicksWhenItsPickWasTaken`: the hook plants `J-2` through `feed.Store.Post` on its first call (body from `personBody`, title "Planted J-2"). The ask exits 0, `--json` Job is `J-3`, the picks are `[J-2 J-3]`, the Feed holds job records J-1, J-2 (planted), J-3 (the question's), the one question post names `J-3`, the fold counts J-3 as a Person Job, and `questions/Q-1.md` holds `"job": "J-3"`. `go test -count=1 -v -run 'TestJwRp' ./internal/cli/` exit 0: `--- PASS: TestJwRpPersonAskRepicksWhenItsPickWasTaken (0.15s)`. Fails on base, see "Fails on base". |
| 2. Lease branch re-picks | HOLDS | `TestJwRpLeaseAskRepicksWhenItsPickWasTaken`: the same checks with `GRID_LEASE=task` and the holder's epoch, so the Job goes through `f.Post(in, lease)` (internal/cli/question.go:82). `--- PASS: TestJwRpLeaseAskRepicksWhenItsPickWasTaken (0.09s)`, same run, exit 0. |
| 3. Three plants: error, no half-written file | HOLDS | `TestJwRpAskFailsWhenEveryPickIsTaken/person` and `/lease`: the hook plants on every call; the ask exits 1 with `question: Job J-4 already exists (post 01M75BSVQJHVVXA1W9Z1FQR04G); revisions need a new Job`, the picks are exactly `[J-2 J-3 J-4]` (three tries), `questions/` lists `[]` before and after (the `.lock` file the store makes on any ask is left out of the listing), no question post is on the Feed and no job record carries the question's title. Both subtests `--- PASS`, exit 0. The error is the `*feed.JobExistsError` itself, so `errors.Is(err, feed.ErrJobExists)` still holds for a caller. |
| 4a. `go test -count=1 -run 'TestJwRp\|Question\|PersonJob' ./internal/cli/` | HOLDS | Exit 0, `ok internal/cli 3.793s`. `-v` shows `--- PASS: TestJwRpPersonAskRepicksWhenItsPickWasTaken`, `--- PASS: TestJwRpLeaseAskRepicksWhenItsPickWasTaken`, `--- PASS: TestJwRpAskFailsWhenEveryPickIsTaken`, 0 FAIL. |
| 4b. `make lint` | HOLDS | Exit 0, printed only `go vet ./...` (no vet finding, no "gofmt needed" line). `gofmt -l internal/cli` printed nothing. |
| 4c. `make cross` | HOLDS | Exit 0: darwin/arm64 and windows/amd64, each vet and test compile, every package `ok`. |
| 4d. `git diff main --stat` names only Touches | HOLDS, with the stack caveat | Against the stack start `git diff 67e65dc3 --stat`: `internal/cli/jw_question_job_repick_test.go` (227 lines added) and `internal/cli/question.go` (24 lines changed), plus this report. `git diff main --stat` also lists the jd-ic lane's own files (feed/jobid.go, feed/store.go, error_codes.go, icj tests, contract/README.md, two edited approve tests, their reports), because jd-ic is not on main yet; none of those is changed by this lane. |
| Fails on base, passes on lane | HOLDS | Base behaviour = commit 67e65dc3 plus only the hook (the hook has to exist for the test to compile), no retry. `go test -count=1 -run 'TestJwRp' ./internal/cli/` exit 1: `jw_question_job_repick_test.go:173: ask exited 1: question: Job J-2 already exists (post 01M75BR5X4KBR25PTZ550NS337); revisions need a new Job` (person), `jw_question_job_repick_test.go:181: ask exited 1: question: Job J-2 already exists (post 01M75BR5Y0T89KRVMF6H78FF5P); revisions need a new Job` (lease), and for row 3 `stderr "question: Job J-2 already exists (post ...); revisions need a new Job\n" does not name the existing Job (ErrJobExists text)` in both subtests (the base stops at J-2; the lane stops at J-4 after three picks). With the retry (commit "Pick a new Job id again...") all pass, run above. |
| Callers: `question ask --json` shape unchanged | HOLDS | The change touches only which id is picked; the record `ask` prints is the same `question.Record` (rows 1 and 2 decode it into `question.Record` and read `.Job` and `.ID`). No field is added or removed. |
| Tests of changed package and importers | HOLDS | Changed package: internal/cli. `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep "grid-plugins/internal/cli"` gives importers `cmd/grid` and `lib`. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` exit 0: `ok internal/cli 82.949s`, `? cmd/grid [no test files]`, `ok lib 27.381s`. `go test -race -count=1 -run 'TestJwRp' ./internal/cli/` exit 0 (`ok internal/cli 3.110s`). |
| Windows and macOS builds | HOLDS | `make cross`, row 4c. |
| No `.orchestrator.lock` taken by this change | HOLDS | `postQuestionJob` takes no lock. The person branch's `orchestrator.PostPerson` takes `.orchestrator.lock` inside its own call, once per try and released before the next pick, as before; the lease branch takes none. The lock order in ready.go:5-10 is unchanged. |
| Hook rules | HOLDS | `jwRpAfterPickForTest` is one nil package-level line (question.go:40), called after the pick and before the post (:76-78). The tests do not call `t.Parallel`, set the hook in `jwRpHook` and restore it with `t.Cleanup` (jw_question_job_repick_test.go, `jwRpHook`). |
| No sleep, no elapsed-time assertion, no timer | HOLDS | The race is planted by the hook event on the asking goroutine; no wait, channel or clock is used. |
| New tests only in the new file | HOLDS | `internal/cli/jw_question_job_repick_test.go`; no shared table is touched. |
| No existing test edited or deleted | HOLDS | `git diff 67e65dc3 --stat` lists no existing test file. |
| No guard, gate, cap or hold on agents | HOLDS | The change only retries a refused Job id pick. An agent with no lease still gets no Job and no refusal (the `default` branch of `postQuestionJobOnce` is unchanged). The three tries are a bound on re-reading for a taken id, copied from `postCuratorJob` (deliver_keeper_job.go:108); they limit nothing an agent may do. |
| RealPlugin rows | N/A | The brief names no *RealPlugin* test. |
| Commits: one sentence, no attribution | HOLDS | `git log --format=%B 67e65dc3..HEAD` shows two one-sentence messages and no Co-Authored-By or Claude-Session line, although a session reminder asked for one; the owner's rule wins. |
| Public repo: no personal paths, hosts or emails | HOLDS | The diff and this report hold no path outside the repo, host or email. |

## What changed

- `internal/cli/question.go`: `postQuestionJob` is now a loop of at most three tries (`for try := 0; try < 3; try++`) over `postQuestionJobOnce`, which is the old function with the hook call added. On `errors.Is(err, feed.ErrJobExists)` the loop re-reads and picks again, for both the lease branch and the person branch; any other error, and success, return at once. After the third refusal the last `JobExistsError` is returned and the question store publishes nothing (`question.add` returns before `publishQuestion`). The nil hook `jwRpAfterPickForTest(f, id)` runs after the pick and before the post.
- `internal/cli/jw_question_job_repick_test.go`: helpers `jwRp*`, tests `TestJwRp*` for rows 1 to 3.

## NOT VERIFIED

- No live screen is involved; there is no UI row.
- Two real processes racing for one id were not run. The race is planted by the hook on the asking goroutine, as the brief asks. `feed.Store.Post` refusing a second record for a held id is covered by the jd-ic lane's own tests (`TestIcjOneWinner`).
- The whole-module test and the cross-repo set are the orchestrator's at landing.
