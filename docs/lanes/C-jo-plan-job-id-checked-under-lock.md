# C-jo-plan-job-id-checked-under-lock

Repo: plugin. Built on plugin main 99c11b7a (every path:line below is read on that base plus this lane).
Base has no `feed.ErrJobExists` (C-jd-ic-job-post-exists-fix is not in it); the lane does not depend on it.

## Proof table

| Row / rule | Result | Command and exit code, or file:line |
|---|---|---|
| 1. `TestJoPjHeldIDSkipped` | HOLDS | `go test -count=1 -run '^TestJoPjHeldIDSkipped$' -v ./internal/migrate/` exit 0: `--- PASS: TestJoPjHeldIDSkipped (0.06s)`. The held seed posts and reads `held` (jo_plan_job_id_test.go:14-45); `PostPlanJob` returns J-6 (:69); after the record with Input.ID `u` is posted, `Jobs["J-5"].Title` is "Planted" and `Jobs["J-6"].Title` is "Approve the plan" (:80-86). On base receipt.go it fails: `jo_plan_job_id_test.go:70: want J-6 above the held J-5, got "J-5" <nil>`, exit 1. |
| 2. `TestJoPjTwoWriters` | HOLDS | `go test -count=1 -run '^TestJoPjTwoWriters$' -v ./internal/migrate/` exit 0: `--- PASS: TestJoPjTwoWriters (0.09s)`. Leased `w` and `personWriter()` wait on one closed channel, a WaitGroup collects both results, no sleep or timer (jo_plan_job_id_test.go:90-123, gate at :105-106). Both return nil, ids differ, neither is J-5, J-5 and each returned id are carried by exactly one `job` entry. On base: `jo_plan_job_id_test.go:113: the two writers took "J-5" and "J-5"; want two ids, neither J-5`, exit 1. |
| 3. `TestJoPjPersonUnlessUnderLock` | HOLDS | `go test -count=1 -run '^TestJoPjPersonUnlessUnderLock$' -v ./internal/orchestrator/` exit 0: `--- PASS: TestJoPjPersonUnlessUnderLock (0.01s)`. The probe opens `.orchestrator.lock` through `fslock.OpenNoFollow` inside `done`, `TryLockExclusive` gives `fslock.ErrLocked`, the bool is true, recordCount unchanged; a `done` returning false writes one unleased record at fence 0; a worker is refused before `done` is asked (jo_person_unless_test.go:23-74). On base it does not build: `undefined: PostPersonUnless` (x3), exit 1. Mutation proof: with `done` called once before `exclusive` the test fails `jo_person_unless_test.go:49: the lock was not held while done ran: <nil>`, exit 1; reverted with `git checkout -- internal/orchestrator/person_write.go`, `git status --short` empty. |
| 4. package tests | HOLDS | `go test -count=1 ./internal/migrate/ ./internal/orchestrator/` exit 0: `ok  internal/migrate 13.835s`, `ok  internal/orchestrator 126.244s`. `go test -count=1 -run 'Approve\|Reopen\|Person' ./internal/cli/` exit 0: `ok  internal/cli 11.217s`. `TestPostPlanJobNumbersAfterTheHighestJob` and `TestReceiptToJobWithAPersonWriterPostsAnAnswerTheHoldersPassAccepts` PASS unchanged (`go test -count=1 -run '...' -v ./internal/migrate/` exit 0). |
| 5. lint, cross, diff | HOLDS | `make lint` (`go vet ./...`) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. `gofmt -l internal/orchestrator internal/migrate` prints nothing. `git diff --name-only main` lists the four Touches files, plus this report (docs/lanes/, which the runbook requires), and nothing else. `git diff main --stat -- internal/orchestrator/write.go internal/cli/ internal/feed/store.go internal/orchestrator/ready.go internal/orchestrator/replan.go` is empty. |
| Importers of the changed packages | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E "internal/(orchestrator\|migrate)( \|$)"`: `internal/cli`, `internal/migrate`, `internal/orchestrator`. Whole `internal/cli`: `go test -count=1 ./internal/cli/` exit 0, `ok  internal/cli 63.984s`. |
| New tests fail on base, pass on lane | HOLDS | Failing lines pasted in rows 1-3 (base `receipt.go` and `person_write.go` restored with `git checkout 99c11b7a -- <file>`, then `git checkout HEAD -- <file>`; `git status --short` empty before the last commit). |
| No existing test edited or deleted | HOLDS | `git diff main --name-only` has no existing test file. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Only which id the writer takes and where it is checked changed; a worker with no lease and no person role is still refused by the same errors as before (receipt_person_test.go:75). No new error, exit code or `--json` code. |
| `*RealPlugin*` tests | N/A | No row names one; none run, none claimed. |
| UI rows | N/A | No UI change. |
| Public repo: no personal paths, hosts, emails | HOLDS | Fixtures use `dana`, `sam`; this file uses repo-relative paths only. |
| Commits | HOLDS | Two commits plus this report, one sentence each, no attribution lines. |

## What changed

- `internal/orchestrator/person_write.go:42-78`: `PostPersonUnless(store, w, in, done) (feed.Record, bool, error)` is PostPerson's body with `done` asked first inside the same `exclusive` lock, as `WriteUnless` does (write.go:51-53); true writes nothing and returns the bool true. `PostPerson` is `PostPersonUnless(..., nil)` and does what it did. write.go is untouched.
- `internal/migrate/receipt.go:208-241`: `PostPlanJob` picks one above the highest J-<n> over the body `id` of every `job` entry of `store.ReadOnly` (any Verdict) and the top number of `snap.Items`, then posts with `done` = "a `job` entry, any Verdict, already carries this id", read again under the lock. When it says taken, it picks again from a fresh read; each round sees one more taken id, so the loop ends. Same rule for a person (`PostPersonUnless`) and for the rest (`WriteUnless`). A read error inside `done` stops the post and is returned.
- Helpers directly below it (`receipt.go:245-305`: `jobNumber`, `planJobIDs`, `nextPlanJobID`, `planJobIDTaken`). The brief's Touches says "PostPlanJob only"; these are new private functions of the same pick, no other function of the file is edited.
- Old Feeds read the same: nothing is rewritten, the fold is not touched. Two presses of `grid approve` at once now give two approve Jobs (J-n and J-n+1) instead of one counted and one `ignored` record (owner question 1 default).

## Notes

- A held record whose body is only a `ref` to a body file not yet on disk has no readable `id`, so its number is not seen by the pick. Nothing in the brief covers it; no change made.
- The loser of two writers no longer reaches `feed.ErrJobExists` (exit 3) once C-jd lands: `done` runs before the post, under the lock.

## NOT VERIFIED on a live screen

Nothing here is a screen. Not run: the real `grid approve` binary against a live Feed (two terminals pressing at once); the two-writer proof is the in-process goroutine test with the real lock files.
