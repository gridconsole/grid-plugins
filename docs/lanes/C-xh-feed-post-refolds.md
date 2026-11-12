# C-xh-feed-post-refolds

Verdict: NOT DONE. Two rows are NOT DONE (1 and 5) and the lane is RED on two tests (named below). Do not land it blind.

Base: 103b9157 (local main; the brief names 44c7de5b, main has moved). Code head: 4ed9d058 (the report commit follows it).
Base check: no post path refolded on the base. `fold.go:56` `SaveState()` passes nil; `cli/feed.go:165/167/169` and `mcp/tools.go:224/226/228` end at `return output(...)` / `render(...)` with no `SaveState`. Not stopped.
Budget: about 58 calls against "about 55" (two failed edits on a wrong tab count, one wrong-premise run). The advisor was called twice.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | TestXhJobPostRefolds: `job post --type answer` leaves Needs you at once | NOT DONE | See "Row 1" below. `go test -count=1 ./internal/cli -run TestXhJobPostRefolds` exits 1 on the lane, same line as on base: `xh_post_refold_test.go:65: needs-you.json still lists the answered question 01M76RVSW3CMMPVKTVAXS2SB7A: [01M76RVSW3CMMPVKTVAXS2SB7A]` |
| 2 | TestXhFeedPostRefoldError: exit 0, post JSON, stderr line `refold: ` | HOLDS | `go test -count=1 -v ./internal/feed/ ./internal/cli/ -run TestXh` shows `--- PASS: TestXhFeedPostRefoldError`. Base: `xh_post_refold_test.go:121: stderr has no line starting "refold: ": ""` (exit 1). Code: `cli/feed.go:171-177`. |
| 3 | TestXhSaveStateKeepsLiveness | HOLDS | Same run: `--- PASS: TestXhSaveStateKeepsLiveness`. Base: `xh_liveness_keep_test.go:68: J-2 lost its liveness after SaveState: {"jobs":{...` (exit 1). Code: `feed/fold.go:56-67, 90-93, 121-147`. |
| 4 | TestXhLivenessDropsChangedJob; State() and needs-you.json equal with and without the overlay | HOLDS, with the difference "Row 4 uses a control accept" below | Same run: `--- PASS: TestXhLivenessDropsChangedJob`, `TestXhLivenessDropsNewAssignPost`. Base: `xh_liveness_keep_test.go:135: J-2 liveness <nil>, want "alive"` (it first checks the Job keeps liveness after a result, then drops it after the accept). `TestSaveStateLivenessOverlaysProjectionOnly` still passes unedited (`go test ./...`: `ok internal/feed`). |
| 5 | `go test ./internal/feed/ ./internal/question/ ./internal/orchestrator/ ./internal/mcp/ ./internal/cli/` exits 0 | NOT DONE | feed, question, orchestrator, mcp are `ok`. `internal/cli` exits 1 on two tests: `TestXhJobPostRefolds` (row 1) and `TestExitRefusalJobSide` (7 subtests: ack, submit, result, checkpoint, worker-exit, integrate, post), e.g. `exit_refusal_test.go:168: read .../T-07-example/feed/.cache: is a directory`. See "Red test 2". |
| 5 | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` and `go test -count=1 ./internal/job/lint/` | HOLDS | Both exit 0 (`ok internal/cli 0.090s`, `ok internal/job/lint 0.419s`), run after the last code change. |
| 6 | `make lint` exits 0 | HOLDS | Exit 0 (`go vet ./...`). |
| 6 | `make cross` exits 0 | HOLDS | Exit 0 (darwin/arm64 and windows/amd64: vet and test compile). |
| 6 | `git diff main --stat` names only the Touches files | HOLDS | `cli/feed.go`, `feed/fold.go`, `mcp/tools.go`, `feed/xh_liveness_keep_test.go`, `cli/xh_post_refold_test.go`. This is why the `job post` fix (below) was not made. |
| R | Tests of changed packages and every importer | HOLDS except the two red tests | `go test -count=1 ./...` (every package): all `ok` except `internal/cli`. Importers of feed/cli/mcp (`go list`): cmd/grid, internal/approvefix, cli, hook, job/jobcmd, lease, mcp, migrate, orchestrator, question, task, usage, lib. |
| R | New tests fail on base, pass on the lane | HOLDS for 2, 3, 4; row 1 fails on both | Base failures pasted in rows 1-4. `TestXhEmptyOverlayStillReplaces` and `TestXhSaveStateIgnoresUnusableOldState` pass on base too: they pin behaviour that must not change (the tick's empty map still replaces; an unusable old state.json is no error). |
| R | No existing test edited or deleted | HOLDS | `git diff main --stat` shows no existing test file. |
| R | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. No new lock is taken while Post's locks are held: the refold runs after `Post`/`Handover`/`PostJobLocked` return (`orchestrator/jw_post_job.go:26-31` releases the pass lock; `feed/jobid.go:34-38` order stands). `fold.lock` is taken alone. |
| R | Windows and macOS builds | HOLDS | `make cross`, above. |
| R | Real-plugin tests | n/a | No row names one. |
| R | Commits: one sentence, no attribution, no personal paths | HOLDS | `git log -1`. |

## Row 1: what the brief missed

The brief says `grid job post` goes through `cli/feed.go:165-169`. It does not. `grid job post/ack/result/cancel` are in `internal/cli/orchestrator.go`: `write := orchestrator.Write` (`:384`), `write(store, writer, ...)` (`:395`), then `return 0` (`:416`) with no refold; `orchestrator.Write` posts at `orchestrator/write.go:97`. `grid feed post` and `grid feed handover` (`cli/feed.go`) and MCP feed_post/feed_handover do refold now. So TestXhJobPostRefolds, built as the brief says (set up as `TestJobPostAnswerRoleAdmission`), sees:

- request: `job post J-1 --attempt 1 --type answer --re <q.Post> --body {"answer":"Yes"}`, exit 0;
- answer seen: feed/needs-you.json still `["<q.Post>"]` (the failure above).

`cli/orchestrator.go` is not in Touches and the brief says not to change production code beyond it, so I did not edit it. Follow-up lane: after `write(...)` succeeds at `orchestrator.go:395`, call `store.Feed.SaveState()` (or `feed.Store{...}.SaveState()`) and print `refold: <err>` to stderr. The same write serves `ack`, `result` and `cancel`. `AnswerPerson` at `:347` and `PostPerson` return earlier and need the same. The brief's app-caller check covered `post` only; stdout and exit code stay as they are.

The "Check before accept" screenshot pair (answer a question with `grid job post --type answer`, no tick) would still show the answered question on this lane, for the same reason.

## Red test 2: TestExitRefusalJobSide

`exit_refusal_test.go:165-168` posts a seed with `grid feed post`, then calls `intentFiles` (`intent_release_test.go:119-132`), which `os.ReadFile`s every entry of `feed/`. The refold now creates `feed/.cache/fold.lock`, so the helper reads a directory and fails. The refusal behaviour that test checks is unchanged. Fix (not made: existing test file, outside Touches): make `intentFiles` skip `entry.IsDir()`. A decision for the orchestrator: widen Touches to that helper, or accept it in the follow-up lane.

## What changed

- `internal/feed/fold.go`: `SaveState()` now keeps, under `fold.lock`, the `liveness` values already in state.json for Jobs whose `assign_post` and `status` are unchanged (`keptLiveness`). A missing or undecodable state.json keeps nothing and is no error. `SaveStateLiveness(map)` is as before.
- `internal/cli/feed.go`: after a good `feed post`, `feed handover` or `feed post --type job`, `store.SaveState()`; an error prints `refold: <err>` on stderr. Answer, stdout and exit code unchanged.
- `internal/mcp/tools.go`: the same refold after feed_post/feed_handover; a fold error changes nothing in the result.
- Two new test files with 5 feed tests and 3 cli tests.

## Differences from the brief

- Keep is on `SaveState()` only, not on `SaveStateLiveness(nil or empty)`. The existing `TestSaveStateLivenessOverlaysProjectionOnly` (`fold_state_test.go:231-238`) asserts that `SaveStateLiveness(nil)` drops liveness, and the brief says it must pass unedited; both cannot hold. The tick (`tick.go:216`) passes its own map each pass, and an empty map there means no holder is observed, so it should replace. Proved by `TestXhEmptyOverlayStillReplaces`.
- Row 4 uses a control accept, not a result: a result moves no status (`contract/fold.go:350-361`). The test checks that a result keeps the liveness, then that the accept drops it.
- MCP: no stderr (stdout is the protocol), so a refold error there is dropped, and there is no new MCP test (no MCP test file in Touches).
- Base is 103b9157, not 44c7de5b.

## NOT VERIFIED

- On a live screen: Home, Needs you right after an answer (the Check before accept pair). Not shot; and `job post` would not pass it yet.
- `SaveState()` after a post on a store whose `TaskID` was empty (legacy task, identity set inside `Post`): `cli/feed.go:108` guards only reads. Untested; an error there prints `refold:` and changes nothing else.
- The MCP refold has no test.

DONE / NOT DONE: NOT DONE
