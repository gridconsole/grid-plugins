# C-jw-park-reuse-projects: a parked Job that reuses an open gate question writes the question file

Lane branch `lane-C-jw-park-reuse-projects`, from main (`git merge main` says "Already up to date").

## What changed

- `internal/orchestrator/events.go` (park, reuse branch, lines 96-108; the Sync call is line 102): when the park finds the open question for this Job and wake `<failure>/1`, and `s.Feed.TaskID` is set, it calls `question.Store{TaskDir, TaskID}.Sync()` before `s.control(...)`. A Sync error is returned like any other park error. With TaskID empty nothing is projected, as before.
- `internal/orchestrator/jw_park_reuse_test.go` (new): helpers `jwPrCrash`, `jwPrFiles`, `jwPrPosts`, `jwPrDecode`, `jwPrCrashAfterPost`, `jwPrPark` (copied from `jrPark*` in `jr_park_question_file_test.go`, `jwPr` names), tests `TestJwPrReuseWritesQuestionFile` and `TestJwPrReuseNoTaskIDProjectsNothing`.
- Lock order: `Sync` takes the questions lock, the same one `ReserveGate` takes, inside the pass; `ready.go:8-10` says "Question ID reservation uses the question store lock inside the pass". No new lock, no guard, gate, cap or hold on agents. Nothing removed.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. Park runs again after a crash between post and park, question file removed: the file is back with the same id | HOLDS | `go test -count=1 -v -run TestJwPr ./internal/orchestrator/` exit 0: `--- PASS: TestJwPrReuseWritesQuestionFile`. It checks file `Q-1.md` decodes to id `Q-1`, `post` equal to the one Feed post, job `J-1`, state `open`. Red on base (events.go change stashed, same command, exit 1): `jw_park_reuse_test.go:124: question files [] after the park, want [Q-1.md]` / `--- FAIL: TestJwPrReuseWritesQuestionFile` |
| 2. TaskID empty: nothing projected, as on base | HOLDS | `--- PASS: TestJwPrReuseNoTaskIDProjectsNothing` on the lane, and the same PASS on base (it is a guard, not a red test): the park finishes (one control post) and `questions/` holds no `Q-n.md`, one question post |
| 3. One question file only | HOLDS | `TestJwPrReuseWritesQuestionFile` asserts `jwPrFiles == [Q-1.md]` after the park and after a second park, and one question post on the Feed (`jw_park_reuse_test.go:124` and `:135`). Neighbours `TestJrParkOneFile`, `TestJrParkWritesQuestionFile`, `TestEventsTickPark/question_reused` PASS in the same run (exit 0) |
| 4a. `go test -count=1 ./internal/orchestrator/ ./internal/question/` 0 fail | HOLDS | exit 0: `ok .../internal/orchestrator 167.126s`, `ok .../internal/question 6.941s` |
| 4b. `make lint` | HOLDS | exit 0 (`go vet ./...`); `gofmt -l internal/orchestrator` prints nothing |
| 4c. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test-compile, every package `ok` |
| 4d. `git diff main --stat` names only Touches | HOLDS | before this report: `events.go | 7 ++`, `jw_park_reuse_test.go | 170 +++` (2 files). This report `docs/lanes/C-jw-park-reuse-projects.md` is added after, as the runbook asks. |
| Packages that import the changed one | HOLDS | `go list` shows `internal/orchestrator` is imported by `internal/cli` and `internal/migrate` (and `cmd/grid`, `lib` through `cli`). `go test -count=1 ./internal/orchestrator/ ./internal/question/ ./internal/cli/ ./internal/migrate/ ./cmd/grid/ ./lib/` exit 0: orchestrator ok, question ok, cli ok 96.938s, migrate ok 16.496s, lib ok 26.177s, cmd/grid no test files |
| New tests fail on base, pass on lane | HOLDS for row 1 (red line above). Row 2's test passes on both, by design of the row |
| No existing test edited or deleted | HOLDS | `git diff main --stat` shows no test file changed besides the new one |
| No sleep, no elapsed-time assertion, no hook set | HOLDS | the new file has none; it sets no `fslock.OnBlockForTest` and calls no `t.Parallel` |
| Windows and macOS builds | HOLDS | `make cross` above |
| Commits | HOLDS | one sentence each, no attribution lines |
| `*RealPlugin*` tests | not applicable | the brief names none |
| `git merge main` before the last proof run | HOLDS | "Already up to date" |

## How the test makes the crash

`jwPrCrashAfterPost` runs the crash fixture (three crashed attempts, so J-1 is parked and `Q-1` is posted), then removes `questions/Q-1.md` and keeps the `Q-1.feed` marker. That is the state a crash leaves between the post and the projection. `jwPrPark` then runs `Store.park` again on a fresh `Load`, with `jobMemory.Failure` set to the failure id taken from the post's wake (`<failure>/1`), which is the reuse branch at events.go:96-108. For row 2 the test clears `Feed.TaskID` only for the park call; the Feed then reads the same id from a `task.md` the test writes.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run: a real crash of a real orchestrator process between the post and the park; the test removes the file the crash would have left unwritten, it does not kill a process.

DONE
