# C-xh2-post-refold-rows

Verdict: DONE. Rows 1 and 5 of C-xh-feed-post-refolds are green, and every acceptance row of both briefs holds. Two files are outside the base Touches list: `internal/cli/orchestrator.go` (the refold after `grid job post`) and `internal/cli/intent_release_test.go` (an edit to the existing helper `intentFiles`). Step 3 of this brief points at the report sections that name them ("Row 1" and "Red test 2"), and both are in package `internal/cli`. Rows 4 and 5 below declare both.

Base: lane-C-xh-feed-post-refolds at d39442fb, merged with origin/main (merge 88ff49cb). The merge had no conflicts, so no file was resolved by hand.
Code commit: d60415da. The report commit follows it.
Budget: about 40 calls against "about 40". The advisor was called before the edits and before hand-off.

## The failures as found (step 2)

`go test -count=1 ./internal/cli -run 'TestXh|TestExitRefusalJobSide'` after the merge, exit 1:

```
--- FAIL: TestExitRefusalJobSide (0.06s)
    --- FAIL: TestExitRefusalJobSide/ack (0.04s)
        exit_refusal_test.go:168: read /tmp/TestExitRefusalJobSideack1255719870/001/home/.grid/projects/example/tasks/T-07-example/feed/.cache: is a directory
    (the same for submit, result, checkpoint, worker-exit, integrate, post)
--- FAIL: TestXhJobPostRefolds (0.78s)
    xh_post_refold_test.go:65: needs-you.json still lists the answered question 01M76VF15SHX435CDAP28JHMK0: [01M76VF15SHX435CDAP28JHMK0]
```

## Why (step 3)

- Row 1: `grid job post` does not go through `cli/feed.go`. It goes through `cli/orchestrator.go`: `write(store, writer, feed.Input{...})` (`:395`), then stdout and `return 0` with no refold. The fix refolds right after a good write (`cli/orchestrator.go:403-414`), as `cli/feed.go:178` does: `SaveState()` takes `fold.lock` alone after `write` has returned and released its locks. A fold error prints `refold: <err>` on stderr. Stdout and the exit code do not change. The same write serves `ack`, `result`, `cancel`, `checkpoint` and the unleased `PostPerson`.
- Red test 2: the refold made by a seed `grid feed post` creates `feed/.cache/fold.lock`. `intentFiles` (`intent_release_test.go:119`) `os.ReadFile`d every entry of `feed/`, so it hit the directory. It now skips directories (`intent_release_test.go:127-130`).
- A third red came out of the first fix. `go test -count=1 ./...` then failed `TestReadUnreadablePolicy` (`job post message` and `job post control`, 3 failure kinds each): `withdraw_policy_test.go:246: want one path-free withdraw: [] warning, got "warning: cannot read task withdrawal policy (workflow.md); using withdraw: []\nwarning: cannot read task withdrawal policy (workflow.md); using withdraw: []\n"`. The write's own fold had already said the warning, and the refold said it a second time. The refold now folds with `Warn: io.Discard` (`cli/orchestrator.go:409-410`), so the operation says it once, as on base. The test is not edited.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | `git merge origin/main`, name each file resolved | HOLDS | Merge 88ff49cb, no conflicts, no file resolved. |
| 2 | Quote the failing command | HOLDS | "The failures as found", above. |
| 3 | `go test -count=1 ./internal/cli ./internal/feed -run TestXh` passes | HOLDS | Exit 0, with `-v`: `--- PASS: TestXhJobPostRefolds`, `TestXhFeedPostOpensQuestionAtOnce`, `TestXhFeedPostRefoldError`, `TestXhSaveStateKeepsLiveness`, `TestXhEmptyOverlayStillReplaces`, `TestXhLivenessDropsChangedJob`, `TestXhLivenessDropsNewAssignPost`, `TestXhSaveStateIgnoresUnusableOldState`; `ok internal/cli`, `ok internal/feed`. |
| 3 | `go test -count=1 ./internal/cli/ ./internal/feed/ ./internal/mcp/` passes | HOLDS | Exit 0: `ok internal/cli 51.959s`, `ok internal/feed 15.497s`, `ok internal/mcp 0.262s` (run with question and orchestrator, both `ok`). |
| 3 | `make lint` exits 0 | HOLDS | `make lint >/dev/null 2>&1 && make cross >/dev/null 2>&1 && echo LINT_AND_CROSS_EXIT_0` printed `LINT_AND_CROSS_EXIT_0`. |
| 4 | Fix only the reds; no row weakened, no test deleted, no skip | HOLDS, with one test-helper edit | No skip, nothing deleted, no row changed. One edit to an existing test helper: `intentFiles` skips directories (`intent_release_test.go:127-130`). The record files it compares are the same as on base. `os.ReadFile` on a directory was already fatal, so no passing run ever compared one. The report's "Red test 2" section names this fix, and step 3 of this brief points at that section. |
| 5 | Base brief row 1 (TestXhJobPostRefolds) | HOLDS | Row 3 run, above. Before the fix: `xh_post_refold_test.go:65` (above). |
| 5 | Base brief row 2 (TestXhFeedPostRefoldError) | HOLDS | `--- PASS: TestXhFeedPostRefoldError` in the row 3 run. |
| 5 | Base brief row 3 (TestXhSaveStateKeepsLiveness) | HOLDS | `--- PASS: TestXhSaveStateKeepsLiveness` in the row 3 run. |
| 5 | Base brief row 4 (TestXhLivenessDropsChangedJob, overlay test unedited) | HOLDS | `--- PASS: TestXhLivenessDropsChangedJob`; `TestSaveStateLivenessOverlaysProjectionOnly` is in `ok internal/feed`. |
| 5 | Base brief row 5: `go test -count=1 ./internal/feed/ ./internal/question/ ./internal/orchestrator/ ./internal/mcp/ ./internal/cli/` exits 0 | HOLDS | All five `ok` (cli 51.959s, feed 15.497s, mcp 0.262s, question 1.393s, orchestrator 93.579s). |
| 5 | Base brief row 5: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` and `go test -count=1 ./internal/job/lint/` | HOLDS | `ok internal/cli 0.055s`, `ok internal/job/lint 0.090s`, exit 0, after the last code change. These are also the plugin walk tests. |
| 5 | Base brief row 6: `make lint` and `make cross` exit 0 | HOLDS | `LINT_AND_CROSS_EXIT_0`, above. |
| 5 | Base brief row 6: `git diff main --stat` names only Touches | HOLDS with two named additions | `git diff origin/main --stat`: `cli/feed.go`, `feed/fold.go`, `mcp/tools.go`, the two xh test files, the reports, plus `cli/orchestrator.go` (+11) and `cli/intent_release_test.go` (+4). Those two are the paths the report's "Row 1" and "Red test 2" sections name. Step 3 of this brief says to fix the answer post on those paths. |
| 5 | Rewrite the NOT DONE rows of docs/lanes/C-xh-feed-post-refolds.md | HOLDS | Rows 1 and 5 now read `CLOSED BY C-xh2-post-refold-rows`. |
| R | Tests of changed packages and every importer | HOLDS | `go test -count=1 ./...`: no non-ok line. Importers (`go list -f '{{.ImportPath}} {{join .Imports " "}}' ./... \| grep -E 'internal/(cli\|feed\|mcp)( \|$)'`): cmd/grid, internal/approvefix, internal/cli, internal/feed, internal/hook, internal/job/jobcmd, internal/lease, internal/mcp, internal/migrate, internal/orchestrator, internal/question, internal/task, internal/usage, lib. |
| R | Repo-wide walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` and `go test -count=1 ./internal/job/lint/` exit 0, run after the last code change. |
| R | New tests fail on base, pass on the lane | HOLDS | No new test in this lane. `TestXhJobPostRefolds` failed before the fix (`xh_post_refold_test.go:65`) and passes now. |
| R | Do not edit an existing test to make it pass | HOLDS except `intentFiles`, declared in row 4 | `TestReadUnreadablePolicy` and `TestExitRefusalJobSide` themselves are unedited. |
| R | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None added. The refold takes `fold.lock` alone, after `write` has returned. |
| R | Windows and macOS builds | HOLDS | `make cross`, above. |
| R | Real-plugin tests | n/a | No row names one. |
| R | Commits: one sentence, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD`. |

## What changed

- `internal/cli/orchestrator.go`: after a good `grid job post/ack/result/cancel/...` write, `SaveState()` on a copy of the store with `Warn: io.Discard`. An error prints `refold: <err>`. Stdout and the exit code do not change.
- `internal/cli/intent_release_test.go`: `intentFiles` skips directories.
- `docs/lanes/C-xh-feed-post-refolds.md`: rows 1 and 5 are CLOSED BY this lane.

## Open items

- `orchestrator.AnswerPerson` (`cli/orchestrator.go:347`, a person's `grid job result`) and `runPersonApproval` (`cli/orchestrator.go:473`) still return without a refold. No row or test covers them. A follow-up lane could add the same three lines.
- `grid feed post` (`cli/feed.go:178`) and MCP feed_post (`mcp/tools.go:237`) refold with the store's own `Warn`. If `Post` also folds on an unreadable workflow.md, the policy warning could print twice. No test covers that, and none is red.

## NOT VERIFIED

- On a live screen: Home, Needs you right after `grid job post --type answer` with no tick (the base brief's "Check before accept" pair). The UI track takes it after landing.

DONE
