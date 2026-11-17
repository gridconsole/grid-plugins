# P-jobs-land-g3: Abdu's V12.c answer (option B), and V12.e and V12.f, in judge-land.md

Repo: plugin. Branch lane-P-jobs-land-g3, cut from lane-P-jobs-land-g2. Docs only.
Doc commit: e74e3923. Line numbers below are on the lane head.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| Brief: no "Open for Abdu: V12.c" left | HOLDS | `grep -nE "Open for Abdu" docs/research/judge-land.md`: no hit, exit 1 |
| Also apply: no "Open for Abdu" anywhere in judge-land.md | HOLDS | same grep, exit 1 |
| B column, decision note 5 | HOLDS | judge-land.md:80-86 "The test run is a runner Job the land Job waits on; the land Job pushes only the tested sha" |
| B column, rejected list (separate-runner bullet deleted; new bullet rejects option A with "the owner's design names a runner Job") | HOLDS | judge-land.md:145-147 |
| B column, Q4 "The runner condition" moves to the runner Job; check, rerun, not waivable, preexisting, bisect and flake run in `grid land test` | HOLDS | judge-land.md:452-461 |
| B column, lane 1 adds the `tested` post schema | HOLDS | judge-land.md:993, row 6 "(V12.c)" |
| B column, lane 3 row 5 "one integrate Job, one runner Job, and one land Job that waits on it" | HOLDS | judge-land.md:995, row 5 "(V27, V12.c)" |
| B column, lane 4 adds `grid land test` to `internal/cli/land.go`, new row (m) | HOLDS | judge-land.md:996, Files and row "(m) (V12.c)" |
| B column, lane 5: `check.go`, `bisect.go`, `flake.go` called by `grid land test`, rows stay | HOLDS | judge-land.md:997 |
| B column, lane 6 row 2 "two runner Jobs on one machine never run at once"; new row 9 `base_moved` gives exactly one new runner Job and land Job | HOLDS | judge-land.md:998, rows "2. (V12.c)" and "9. (V12.c)" |
| B column, sibling run-jobs-launcher.md lane 4 row 9 adds `grid land test`; Q9 `no_test_run` line unchanged | HOLDS | run-jobs-launcher.md:646 row (9); run-jobs-launcher.md:447 untouched |
| Open section replaced by "Decided by Abdu: V12.c", his words verbatim, what Grid does | HOLDS | judge-land.md:910-913 (quote from the 17 Nov ledger: "v12c: yes separate test job is fine that it runs the test, unless that causes problems"), :921-927 (what Grid does), :929-934 (where applied) |
| "Unless that causes problems": the cost (retest when main moves between test and land) written, and B checked against drain V12.f, per-change landing, `no_test_run`, bisect and flake | HOLDS | judge-land.md:936-972; no contradiction found, so no question for Abdu |
| Grep finds no sentence saying the land Job runs its own tests | HOLDS | `grep -nE "test run is the land\|runs the checks\|runs its own\|run checks\|its checks"`: hits only :64 (`grid land test` list) and :810 (the runner Job in B.8). A wider `land Job.*(test\|check\|bisect\|flak\|rebase)` grep: the remaining hits are the runner Job, the land Job waiting on a test, the land pushing the tested SHA, and two lines that name option A as rejected (:145, :917) |
| Brief 3 / also apply: V12.f "confirm" wording dropped, drain reading decided | HOLDS | judge-land.md:526 ("Abdu decided it on 17 Nov ("V12.f confirmed")"), :1040-1042 open items; `grep -n confirm` hits only the two quotes of his words (:526, :974) |
| Also apply: V12.e flake rule, flaky again after one fix is deleted like red again, naming the covering test; incident tests fixed, never deleted; optional-question text removed | HOLDS | judge-land.md:498-504; the old optional note is gone (`grep -nE "optional"` exit 1) |
| Lane doc with proof table, last line DONE or NOT DONE | HOLDS | this file |
| Tests of changed packages and their importers | not run, not asked: no Go package changed (docs only; `go list` not needed). No `*_test.go` reads judge-land.md or run-jobs-launcher.md (`grep -rln` exit 1) |
| Plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok ... internal/cli 0.097s`, exit 0; `go test -count=1 ./internal/job/lint/`: `ok ... internal/job/lint 0.374s`, exit 0 |
| Lint | HOLDS | `make lint` (go vet ./...), exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Merge main before the last proof run | HOLDS | `git fetch origin` then `git merge origin/main`: "Already up to date." |
| New tests fail on base | not run, not asked: docs only, no test added |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the change adds a runner Job and a `base_moved` end; the launcher still makes no decision and has no cap (judge-land.md Q3 step 6) |
| No attribution lines, no private paths | HOLDS | commit message has none; the doc cites "the decision ledger of 17 Nov", not its path |
| Whole-module test, cross-repo set | not run, not asked (orchestrator runs them at landing) |

## What changed

- docs/research/judge-land.md: V12.c option B applied in decision notes 1, 4, 5 and 6, the rejected list, Q2, Q3 (runner Job; "after L-7 lands" now on the runner Job; `grid land test` steps 1-5 and `grid land change` steps 6-7), Q4 (runner condition, flake step 2 reads `tested`, new `base_moved` table row, drain row), Q5 (L2, L4), Addendum 7 B.8 and B.9, Q6 lanes 1, 3, 4, 5 and 6, the acceptance map and the open items. The open section is now "Decided by Abdu: V12.c". V12.e (flake rule) and V12.f (drain) are marked decided.
- docs/research/run-jobs-launcher.md: lane 4 row 9 lists `grid land test` among the verbs run as command Jobs.

Three points B needed that the B column did not spell out, written in the doc:
1. The order edge "after L-7 lands" goes on the runner Job, not the land Job; otherwise every land in a queue would end `base_moved` (judge-land.md:372-380).
2. The runner pushes the tested SHA to its own attempt ref, because a second rebase gives a different SHA (judge-land.md Q3 step 5). `tested` carries `ref` and `head_sha` as well as `sha` and `base_sha`.
3. A lost lease on the land still gives one mechanical new attempt (decision note 6, sibling lane 4 row 10 unchanged); that attempt ends `base_moved` and never pushes an untested SHA.

## Open items

- run-jobs-launcher.md:315 and :406 list the land results by name and do not name `tested` or `base_moved`. Both still read true (a failed exit wakes the orchestrator, and `landed` copies the flakes), so they were left as they are; the brief named only lane 4 row 9 of that doc.

## NOT VERIFIED on a live screen

Nothing; docs only.

DONE
