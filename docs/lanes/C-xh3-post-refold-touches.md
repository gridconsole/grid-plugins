# C-xh3-post-refold-touches

Verdict: DONE. This lane widens Touches to two more files: `internal/cli/orchestrator.go` and `internal/cli/intent_release_test.go`. With that, every changed code file is in Touches. Every acceptance row of C-xh2-post-refold-rows, and so of C-xh-feed-post-refolds, holds on this head. No code changed in this lane. The code is d60415da from lane-C-xh2-post-refold-rows, merged with origin/main.

Base: lane-C-xh2-post-refold-rows at 93048545. Merged with origin/main in bba0ead0 (the code head of this report).
Budget: about 24 calls against "about 30". The advisor was called before the report was written.

## Step 1: merge

`git fetch -q origin && git merge origin/main`: merge commit bba0ead0, no conflicts. No file was resolved by hand.

## Step 2: diff against origin/main

`git diff --stat origin/main...HEAD` at bba0ead0:

```
 docs/lanes/C-xh-feed-post-refolds.md   |  58 +++++++++
 docs/lanes/C-xh2-post-refold-rows.md   |  71 +++++++++++
 internal/cli/feed.go                   |   7 ++
 internal/cli/intent_release_test.go    |   4 +
 internal/cli/orchestrator.go           |  11 ++
 internal/cli/xh_post_refold_test.go    | 127 ++++++++++++++++++++
 internal/feed/fold.go                  |  48 +++++++-
 internal/feed/xh_liveness_keep_test.go | 211 +++++++++++++++++++++++++++++++++
 internal/mcp/tools.go                  |   5 +
 9 files changed, 541 insertions(+), 1 deletion(-)
```

All seven code files are in Touches:
- From the base brief: `feed/fold.go`, `cli/feed.go`, `mcp/tools.go`, `feed/xh_liveness_keep_test.go` and `cli/xh_post_refold_test.go`.
- From this brief: `cli/orchestrator.go`, with the refold after `write(...)` at `internal/cli/orchestrator.go:403-414`, and `cli/intent_release_test.go`, where `intentFiles` skips directories at `internal/cli/intent_release_test.go:127-130`.

The two `docs/lanes` files are the reports of the two earlier lanes. This report adds a third `docs/lanes` file, so the stat has 10 files after the report commit.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | `git merge origin/main`, name each file resolved | HOLDS | Merge bba0ead0, no conflicts, no file resolved. |
| 2 | The lane's diff touches only Touches files | HOLDS | `git diff --stat origin/main...HEAD`, quoted above. It shows 7 code files, all in Touches, plus the lane reports. |
| A1 | C-xh2 row: `go test -count=1 ./internal/cli ./internal/feed -run TestXh` passes | HOLDS | `go test -count=1 -v -run TestXh ./internal/cli ./internal/feed` shows `--- PASS: TestXhJobPostRefolds (0.77s)`, `--- PASS: TestXhFeedPostOpensQuestionAtOnce`, `--- PASS: TestXhFeedPostRefoldError`, `ok internal/cli 0.859s`, `--- PASS: TestXhSaveStateKeepsLiveness (0.22s)`, `--- PASS: TestXhEmptyOverlayStillReplaces`, `--- PASS: TestXhLivenessDropsChangedJob`, `--- PASS: TestXhLivenessDropsNewAssignPost`, `--- PASS: TestXhSaveStateIgnoresUnusableOldState`, `ok internal/feed 1.408s`. No FAIL line. |
| A1 | C-xh2 row and this brief's row: `go test -count=1 ./internal/cli/ ./internal/feed/ ./internal/mcp/` passes | HOLDS | Run inside the base row 5 command: `ok internal/feed 42.924s`, `ok internal/mcp 2.241s`, `ok internal/cli 162.570s`. |
| A1 | C-xh2 row: `make lint` exits 0 | HOLDS | `make lint >/dev/null 2>&1 && make cross >/dev/null 2>&1 && git log -1 --format='make lint and make cross exit 0 at %h'` printed `make lint and make cross exit 0 at bba0ead0`. |
| A1 | Base row 1: TestXhJobPostRefolds | HOLDS | `--- PASS: TestXhJobPostRefolds`, above. The failure before the fix is recorded in `docs/lanes/C-xh2-post-refold-rows.md:11-20`: `xh_post_refold_test.go:65: needs-you.json still lists the answered question ...`. It was not reproduced in this lane. |
| A1 | Base row 2: TestXhFeedPostRefoldError | HOLDS | `--- PASS: TestXhFeedPostRefoldError`, above. |
| A1 | Base row 3: TestXhSaveStateKeepsLiveness | HOLDS | `--- PASS: TestXhSaveStateKeepsLiveness`, above. |
| A1 | Base row 4: TestXhLivenessDropsChangedJob, with TestSaveStateLivenessOverlaysProjectionOnly unedited and passing | HOLDS | `--- PASS: TestXhLivenessDropsChangedJob`, above. `TestSaveStateLivenessOverlaysProjectionOnly` is part of `ok internal/feed 42.924s`. |
| A1 | Base row 5: `go test -count=1 ./internal/feed/ ./internal/question/ ./internal/orchestrator/ ./internal/mcp/ ./internal/cli/` exits 0 | HOLDS | `ok internal/feed 42.924s`, `ok internal/question 8.537s`, `ok internal/orchestrator 232.477s`, `ok internal/mcp 2.241s`, `ok internal/cli 162.570s`. |
| A1 | Base row 5: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` and `go test -count=1 ./internal/job/lint/` exit 0 | HOLDS | Joined with `&&`: `ok internal/cli 0.305s`, `ok internal/job/lint 1.296s`. |
| A1 | Base row 6: `make lint` and `make cross` exit 0 | HOLDS | `make lint and make cross exit 0 at bba0ead0`, above. |
| A1 | Base row 6: the diff names only Touches files | HOLDS | Row 2, above. |
| A1 | C-xh2 step 5: no NOT DONE row is left in `docs/lanes/C-xh-feed-post-refolds.md` | HOLDS | `grep -n -E "NOT DONE\|CLOSED BY"`: rows 1 and 5 (`:13`, `:17`) read `CLOSED BY C-xh2-post-refold-rows`. `:3` and `:58` are history prose. |
| A3 | `make lint` and `make cross` pass | HOLDS | `make lint and make cross exit 0 at bba0ead0`. |
| 3 | Fix only inside Touches | HOLDS | Nothing needed fixing. This lane has no code edit. |
| R | Tests of changed packages and every importer | HOLDS | The changed packages are `internal/cli`, `internal/feed` and `internal/mcp`. The importer command is `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'internal/(cli\|feed\|mcp)( \|$)' \| cut -d' ' -f1`. It lists `cmd/grid`, `contract`, `contract/formats`, `internal/account`, `internal/approvefix`, `internal/braincmd`, `internal/cli`, `internal/feed`, `internal/golden`, `internal/hook`, `internal/job/jobcmd`, `internal/jobgraph`, `internal/lease`, `internal/mcp`, `internal/migrate`, `internal/orchestrator`, `internal/packaging`, `internal/question`, `internal/settings`, `internal/task`, `internal/usage` and `lib`. Five of them pass in base row 5. The others pass in one `go test -count=1` run, and all of them are `ok` except two with `[no test files]` (`cmd/grid`, `internal/approvefix`). |
| R | Repo-wide walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' && go test -count=1 ./internal/job/lint/` gave `ok internal/cli 0.305s` and `ok internal/job/lint 1.296s`, on bba0ead0. |
| R | New tests fail on base and pass on the lane | HOLDS | This lane adds no test. The TestXh tests come from the earlier lanes. Their failure before the fix is in `docs/lanes/C-xh2-post-refold-rows.md:11-20`. |
| R | Do not edit an existing test to make it pass | HOLDS | This lane edits no test. The one helper edit in the stack, `intentFiles` at `intent_release_test.go:127-130`, is in this brief's Touches. `TestExitRefusalJobSide` itself is not edited. |
| R | No guard, gate, cap, hold or blocking hook on agents | HOLDS | None was added. Each refold takes `fold.lock` on its own, after the post has released its locks. |
| R | Windows and macOS builds | HOLDS | `make cross` (darwin/arm64 and windows/amd64 vet plus test compile), above. |
| R | Real-plugin tests | n/a | No row names one. |
| R | UI rows | n/a | No UI row. The screenshot pair is listed under NOT VERIFIED. |
| R | Commits: one sentence, no attribution, no personal paths | HOLDS | `git log origin/main..HEAD`. |

## What changed

- The merge of origin/main (bba0ead0).
- This report.
- No code changed. The two files the C-xh2 reviewer found outside Touches are now in Touches: `internal/cli/orchestrator.go:403-414`, which refolds after `grid job post` and the other `write` verbs with the policy warning discarded, and `internal/cli/intent_release_test.go:127-130`, where `intentFiles` skips directories.

## Open items (from the C-xh2 review notes; no row covers them)

- `orchestrator.AnswerPerson` (`cli/orchestrator.go:347`) and `runPersonApproval` (`cli/orchestrator.go:473`) still return without a refold.
- The MCP feed_post refold drops its error without a word (`mcp/tools.go:237`), because the server has no stderr.
- `cli/feed.go:178` and `mcp/tools.go:237` refold with the store's own `Warn`, so the policy warning could print twice on an unreadable workflow.md. No test covers that.

## NOT VERIFIED

- On a live screen: Home, Needs you right after `grid job post --type answer` with no tick in between. This is the base brief's "Check before accept" pair, and the UI track takes it after landing.

DONE
