# C-pcov4-feed-questions-cache

Tests for person answers per role, question event order, aged events, the Cancelled Job branch, the reader cache refusals and `wireValid`. One new file, `internal/feed/feed_questions_cache_test.go` (`package feed`, no build tag). No production code changed. Helpers are prefixed `questionsCache`; `taskWriter`, `testStore`, `mustPost`, `contractBody` and `message` are reused.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 Coverage before and after | HOLDS | Before, untouched clone: `go test -count=1 -coverprofile=cov.out ./internal/feed/` exit 0, `go tool cover -func=cov.out \| tail -1` -> `total: (statements) 83.3%`. After: same command, exit 0 -> `total: (statements) 85.5%`. 85.5 is above 83.3. (The brief's 83.4% was measured on another toolchain; I compare against what I measured.) The profile was written to `cov.out` in the clone, which `.gitignore:5` (`*.out`) ignores, because the sandbox refused `mktemp -d` and any folder outside the clone. |
| 2 PersonAnswer per role | HOLDS | `TestQuestionsCachePersonAnswerRoles`, 13 subtests, feed_questions_cache_test.go:33. The brief's cases: result/owner false; owner, member, lead true; guest p-1 true; guest p-2 false; worker false; empty role false. Extra spellings that `contract.NormalizeRole` (internal/contract/types.go:20) maps: `grid-owner` true, ` Lead ` true, `grid-guest` p-1 true and p-2 false, bare `grid-` false. `PersonAnswer` 66.7% -> 100.0%. |
| 3 Aged events | HOLDS | `TestQuestionsCacheAgedEvents`, feed_questions_cache_test.go:65. Blocking, one level 1: Aging 1, Events 1. Blocking, two level 1: Aging 1, Events 1. Non-blocking, two level 1: Aging 0, Events 0. All with state "open". The control is posted by `taskWriter` with `Re` and `target` set to the question id, as questions_test.go:50-57 does. Both level 1 posts were accepted, so no allowed-level problem. |
| 4 Cancelled Job | HOLDS | `TestQuestionsCacheCancelledJob`, feed_questions_cache_test.go:100. Two person-gated questions on J-1, live: both in `NeedsYou` and in `Jobs["J-1"].OpenQuestions`, both equal to the sorted id list, both `Open`. After a task-lease control `{target J-1, status cancelled, close_reason}`: `NeedsYou` and `OpenQuestions` empty, both `Open` false. Covers questions.go:153-157 and :170-181. |
| 5 questionEventOrder | HOLDS | `TestQuestionsCacheEventOrder`, feed_questions_cache_test.go:133. `[a,b]` (task lease) -> a,b. `[a(task lease, After c), b(task lease), c]` -> c,a,b. `[a After b, b]` -> b,a. `[a After b, b After a]` -> empty, no panic, no hang; each call runs in a goroutine with a 5s `t.Fatal`. `questionEventOrder` 75.0% -> 100.0%. |
| 6 Reader cache refusals | HOLDS | `TestQuestionsCacheReaderRefusals`, feed_questions_cache_test.go:163. After one `Read` wrote `feed/.cache/reader.json`, it is overwritten with `{`, Version 2, another task ULID, and `files:null`. Each gives the exact error `invalid reader cache; remove feed/.cache/reader.json to rebuild` (cache.go:60-61). With a valid cache, deleting the cached tenure file gives `cached tenure file is missing: <name>` (cache.go:119). Entry point is `Store.Read` -> `scan` (read.go:47, cache.go:32). `scan` 74.7% -> 77.3%. |
| 7 wireValid | HOLDS | `TestQuestionsCacheWireValid`, feed_questions_cache_test.go:207. A line the store wrote: true. `not json`: false. The same line with each of the 17 listed keys removed, "wake" included: false. `wireValid` 75.0% -> 100.0%. |
| 8 Checks | HOLDS | `go test -count=1 ./internal/feed/` exit 0 (ok, 85.5%). `GOOS=windows go vet ./...` exit 0. `GOOS=darwin go vet ./internal/feed/` exit 0. `GOOS=windows go test -c -o /dev/null ./internal/feed/` exit 0. `make lint` (runs `go vet ./...`) exit 0. `gofmt -l internal/feed` prints nothing. |
| Rule: importers tested | HOLDS | Direct importers of internal/feed, from `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: cli, hook, job/jobcmd, lease, mcp, orchestrator, question, task, usage. `go test -count=1` on all nine: exit 0, every line ok. The whole-module test is left to the orchestrator. |
| Rule: new tests fail on base | NOT APPLICABLE | The brief is tests only, with no production change, so the new tests pass on base as well as on the lane. Their reach is shown by the per-function coverage rises above. |
| Rule: no existing test edited or deleted | HOLDS | `git diff --stat origin/main..lane-C-pcov4-feed-questions-cache` lists only the new test file and this report. |
| Rule: no guard, gate, cap or hold added | HOLDS | Test file only. |
| Rule: no personal paths, hosts or emails | HOLDS | None in the new file or this report. |
| Rule: no special files, chmod triggers or unix-only syscalls | HOLDS | The file uses os, filepath, json, fmt, sort, slices, strings and time only, and `GOOS=windows go vet` passes. |

## What changed

- `internal/feed/feed_questions_cache_test.go` (new): six tests.
- `docs/lanes/C-pcov4-feed-questions-cache.md` (this report).
- internal/feed coverage 83.3% -> 85.5%. `foldQuestions` 86.3% -> 94.1%.

## Outcome notes

- Gate ids must be unique per task: a second question reusing gate id `Q-1` was refused with "gate id Q-1 already exists". The Job test numbers its gate ids.
- Nothing in the code differed from the brief's expected results; no bug found, no row left out.

## NOT VERIFIED on a live screen

None; there is no UI in this lane.
