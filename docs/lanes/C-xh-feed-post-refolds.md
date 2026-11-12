# C-xh-feed-post-refolds

Verdict: NOT DONE when this lane handed off; rows 1 and 5 are CLOSED BY C-xh2-post-refold-rows (docs/lanes/C-xh2-post-refold-rows.md), so every row now holds on that lane's head.

Base: 103b9157 (local main; the brief names 44c7de5b, main has moved). Code head: 4ed9d058 (the report commit follows it).
Base check: no post path refolded on the base. `fold.go:56` `SaveState()` passes nil; `cli/feed.go:165/167/169` and `mcp/tools.go:224/226/228` end at `return output(...)` / `render(...)` with no `SaveState`. Not stopped.
Budget: about 58 calls against "about 55" (two failed edits on a wrong tab count, one wrong-premise run). The advisor was called twice.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | TestXhJobPostRefolds: `job post --type answer` leaves Needs you at once | CLOSED BY C-xh2-post-refold-rows | Before: `xh_post_refold_test.go:65: needs-you.json still lists the answered question 01M76VF15SHX435CDAP28JHMK0: [01M76VF15SHX435CDAP28JHMK0]`. Fix: refold after the write in `cli/orchestrator.go:403-414`. `go test -count=1 -v ./internal/cli ./internal/feed -run TestXh` shows `--- PASS: TestXhJobPostRefolds`, exit 0. |
| 2 | TestXhFeedPostRefoldError: exit 0, post JSON, stderr line `refold: ` | HOLDS | `go test -count=1 -v ./internal/feed/ ./internal/cli/ -run TestXh` shows `--- PASS: TestXhFeedPostRefoldError`. Base: `xh_post_refold_test.go:121: stderr has no line starting "refold: ": ""` (exit 1). Code: `cli/feed.go:171-177`. |
| 3 | TestXhSaveStateKeepsLiveness | HOLDS | Same run: `--- PASS: TestXhSaveStateKeepsLiveness`. Base: `xh_liveness_keep_test.go:68: J-2 lost its liveness after SaveState: {"jobs":{...` (exit 1). Code: `feed/fold.go:56-67, 90-93, 121-147`. |
| 4 | TestXhLivenessDropsChangedJob; State() and needs-you.json equal with and without the overlay | HOLDS, with the difference "Row 4 uses a control accept" below | Same run: `--- PASS: TestXhLivenessDropsChangedJob`, `TestXhLivenessDropsNewAssignPost`. Base: `xh_liveness_keep_test.go:135: J-2 liveness <nil>, want "alive"` (it first checks the Job keeps liveness after a result, then drops it after the accept). `TestSaveStateLivenessOverlaysProjectionOnly` still passes unedited (`go test ./...`: `ok internal/feed`). |
| 5 | `go test ./internal/feed/ ./internal/question/ ./internal/orchestrator/ ./internal/mcp/ ./internal/cli/` exits 0 | CLOSED BY C-xh2-post-refold-rows | Before: `internal/cli` exit 1 on `TestXhJobPostRefolds` and `TestExitRefusalJobSide` (`exit_refusal_test.go:168: read .../feed/.cache: is a directory`, 7 subtests). On C-xh2-post-refold-rows: all five `ok`, exit 0; `go test -count=1 ./...` has no non-ok line. |
| 5 | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` and `go test -count=1 ./internal/job/lint/` | HOLDS | Both exit 0, rerun on C-xh2-post-refold-rows after its last code change. |
| 6 | `make lint` exits 0 | HOLDS | Exit 0 (`go vet ./...`), rerun on C-xh2-post-refold-rows. |
| 6 | `make cross` exits 0 | HOLDS | Exit 0, rerun on C-xh2-post-refold-rows. |
| 6 | `git diff main --stat` names only the Touches files | HOLDS for this lane; C-xh2-post-refold-rows adds `cli/orchestrator.go` and `cli/intent_release_test.go`, the paths this report's "Row 1" and "Red test 2" sections name and that brief's step 3 points at. | `cli/feed.go`, `feed/fold.go`, `mcp/tools.go`, `feed/xh_liveness_keep_test.go`, `cli/xh_post_refold_test.go`. |
| R | Tests of changed packages and every importer | HOLDS (closed by C-xh2-post-refold-rows) | `go test -count=1 ./...`: every package `ok`. Importers of feed/cli/mcp (`go list`): cmd/grid, internal/approvefix, cli, hook, job/jobcmd, lease, mcp, migrate, orchestrator, question, task, usage, lib. |
| R | New tests fail on base, pass on the lane | HOLDS | Base failures pasted in rows 1-4. `TestXhEmptyOverlayStillReplaces` and `TestXhSaveStateIgnoresUnusableOldState` pass on base too: they pin behaviour that must not change (the tick's empty map still replaces; an unusable old state.json is no error). |
| R | No existing test edited or deleted | HOLDS for this lane | `git diff main --stat` shows no existing test file. C-xh2-post-refold-rows edits the `intentFiles` helper; its report says why. |
| R | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. No new lock is taken while Post's locks are held: the refold runs after `Post`/`Handover`/`PostJobLocked` return (`orchestrator/jw_post_job.go:26-31` releases the pass lock; `feed/jobid.go:34-38` order stands). `fold.lock` is taken alone. |
| R | Windows and macOS builds | HOLDS | `make cross`, above. |
| R | Real-plugin tests | n/a | No row names one. |
| R | Commits: one sentence, no attribution, no personal paths | HOLDS | `git log -1`. |

## Row 1: what the brief missed (closed by C-xh2-post-refold-rows)

The brief says `grid job post` goes through `cli/feed.go:165-169`. It does not. `grid job post/ack/result/cancel` are in `internal/cli/orchestrator.go`: `write := orchestrator.Write` (`:384`), `write(store, writer, ...)` (`:395`), then `return 0` with no refold. C-xh2-post-refold-rows adds the refold right after a good write (`cli/orchestrator.go:403-414`). That write also serves `ack`, `result`, `cancel` and the unleased `PostPerson`. `AnswerPerson` (`:347`) and `runPersonApproval` still return without a refold; no row covers them (open item in that lane's report).

## Red test 2: TestExitRefusalJobSide (closed by C-xh2-post-refold-rows)

`exit_refusal_test.go:165-168` posts a seed with `grid feed post`, then calls `intentFiles` (`intent_release_test.go:119-136`), which `os.ReadFile`s every entry of `feed/`. The refold now creates `feed/.cache/fold.lock`, so the helper read a directory and failed. C-xh2-post-refold-rows makes `intentFiles` skip directories (`intent_release_test.go:127-130`). The record files it compares are the same as before, and no passing run ever compared a directory.

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

- On a live screen: Home, Needs you right after an answer (the Check before accept pair). Not shot; the UI track takes the pair after landing.
- `SaveState()` after a post on a store whose `TaskID` was empty (legacy task, identity set inside `Post`): `cli/feed.go:108` guards only reads. Untested; an error there prints `refold:` and changes nothing else.
- The MCP refold has no test.

DONE / NOT DONE: DONE through C-xh2-post-refold-rows (this lane alone handed off NOT DONE)
