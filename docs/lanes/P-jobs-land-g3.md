# P-jobs-land-g3: Abdu's V12.c answer (option B), and V12.e and V12.f, in judge-land.md

Repo: plugin. Branch lane-P-jobs-land-g3, cut from lane-P-jobs-land-g2. Docs only.
Doc commits: e74e3923 (judge-land.md and run-jobs lane 4 row 9), 367de84f (the
rest of both docs made consistent with B). Line numbers below are on the lane head.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| Brief: no "Open for Abdu: V12.c" left | HOLDS | `grep -c "Open for Abdu" docs/research/judge-land.md` prints `0` |
| Also apply: no "Open for Abdu" anywhere in judge-land.md | HOLDS | same command, `0` |
| B column, decision note 5 | HOLDS | judge-land.md:80-86 "The test run is a runner Job the land Job waits on; the land Job pushes only the tested sha" |
| B column, rejected list (separate-runner bullet deleted; new bullet rejects option A, reason "the owner's design names a runner Job") | HOLDS | judge-land.md:145-147 |
| B column, Q4 "The runner condition" moves to the runner Job; check, rerun, not waivable, preexisting, bisect and flake run in `grid land test` | HOLDS | judge-land.md:452-461 |
| B column, lane 1 adds the `tested` post schema | HOLDS | judge-land.md:996, row "6. (V12.c)" |
| B column, lane 3 row 5 "one integrate Job, one runner Job, and one land Job that waits on it" | HOLDS | judge-land.md:998, row "5. (V27, V12.c)" |
| B column, lane 4 adds `grid land test` to `internal/cli/land.go`, new row (m) | HOLDS | judge-land.md:999, Files and row "(m) (V12.c)"; row (d) and (f) also adjusted |
| B column, lane 5: `check.go`, `bisect.go`, `flake.go` called by `grid land test`, rows stay | HOLDS | judge-land.md:1000 |
| B column, lane 6 row 2 "two runner Jobs on one machine never run at once"; new row 9 `base_moved` gives exactly one new runner Job and land Job | HOLDS | judge-land.md:1001, rows "2. (V12.c)" and "9. (V12.c)"; rows 1, 6 and 8 adjusted to the runner Job |
| B column, sibling run-jobs-launcher.md lane 4 row 9 adds `grid land test`; Q9's `no_test_run` line unchanged | HOLDS | run-jobs-launcher.md:646 row (9); run-jobs-launcher.md:447 unchanged |
| Open section replaced by "Decided by Abdu: V12.c", his words verbatim, what Grid does | HOLDS | judge-land.md:913-916 (his words, from the 17 Nov decision ledger: "v12c: yes separate test job is fine that it runs the test, unless that causes problems"), :924-930 (what Grid does), :932-938 (where applied) |
| Step 2, "unless that causes problems": the cost (a retest when main moves between test and land) written, and B checked against drain V12.f, per-change landing, the `no_test_run` start condition, bisect and flake | HOLDS | judge-land.md:940-975. No contradiction makes B unworkable, so this lane does not raise a question for Abdu (:975) |
| Grep finds no sentence saying the land Job runs its own tests | HOLDS | `grep -nE "test run is the land\|runs the checks\|runs its own\|run checks\|its checks\|rebases again"` on both docs: judge-land.md:64 (the `grid land test` list), :812 (the runner Job in B.8), run-jobs-launcher.md:616 (`grid land test` ... runs the checks). Every `grid land change` hit in both docs now pushes a tested SHA. Option A is named only as rejected (judge-land.md:145, :920) |
| Step 2 consistency in the sibling (it names judge-land's land verb) | HOLDS | run-jobs-launcher.md:448 (test run is the runner Job), :451-452 (`grid land test` rebases and checks; the land pushes the tested SHA; a lost lease gives `base_moved`), :454 (the "after L-7" edge is on the next runner Job), :536, :616 (C point 8), :627 (C point 19) |
| Brief 3 superseded by "Also apply": V12.f "confirm" wording dropped, drain decided | HOLDS | judge-land.md:526 ("Abdu decided it on 17 Nov ("V12.f confirmed")"), :979, :1043-1045 |
| Also apply: V12.e flake rule: flaky again after one fix is deleted like red again, naming the covering test; incident tests are fixed, never deleted; optional-question text removed | HOLDS | judge-land.md:498-504, :980-982; `grep -nE "optional" docs/research/judge-land.md` exit 1, no output |
| V12.f and V12.e stay as written (brief step 3) | superseded | the brief's "Also apply" (owner, ~04:30 UTC) answers both; applied as above |
| Lane doc with proof table, last line DONE or NOT DONE | HOLDS | this file |
| Tests of changed packages and their importers | not run, not asked | no Go package changed (docs only). No `*_test.go` names judge-land or run-jobs-launcher (`grep -rln ... --include=*_test.go` printed nothing) |
| Plugin walk tests | HOLDS | on 367de84f: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` → `ok github.com/gridconsole/grid-plugins/internal/cli 0.107s`; `go test -count=1 ./internal/job/lint/` → `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.367s` |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0, on e74e3923; the later commits change docs only |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`, both exit 0 with no output, on e74e3923; the later commits change docs only |
| Merge main before the last proof run | HOLDS | `git fetch origin && git merge --no-edit origin/main` → "Already up to date." |
| New tests fail on base | not run, not asked | docs only, no test added |
| No guard, gate, cap, hold or blocking hook added | HOLDS | B adds a runner Job and a `base_moved` end. The launcher still makes no decision and has no cap (judge-land.md Q3 step 6) |
| No attribution lines, no private paths | HOLDS | the commit messages have none; the doc cites "the decision ledger of 17 Nov", not its path |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs them at landing |

## What changed

- docs/research/judge-land.md: V12.c option B is applied in:
  - decision notes 1, 4, 5 and 6, and the rejected list;
  - Q2;
  - Q3: the runner Job, "after L-7 lands" now on the runner Job, `grid land test` steps 1-5 and `grid land change` steps 6-7;
  - Q4: the runner condition, flake step 2 reads `tested`, a new `base_moved` table row, the drain row;
  - Q5 (L2, L4, Brain notes);
  - Addendum 7 B.1, B.8 and B.9, and the 02:50 ruling's note readers;
  - Q6 lanes 1, 3, 4, 5 and 6, the acceptance map and the open items.

  The open section is now "Decided by Abdu: V12.c". V12.e (the flake rule) and V12.f (drain) are marked decided.
- docs/research/run-jobs-launcher.md:
  - lane 4 row 9 lists `grid land test` among the verbs run as command Jobs;
  - Q9 (:448, :451-452, :454), the land row :536 and C points 8 and 19 now say that `grid land test` rebases and checks and the land Job pushes the tested SHA.

B needed three points that its column did not spell out. The doc now states each:
1. The order edge "after L-7 lands" goes on the runner Job, not the land Job. With the edge on the land Job, every land in a queue would end `base_moved` (judge-land.md:372-380).
2. The runner pushes the tested SHA to its own attempt ref, because a second rebase gives a different SHA (judge-land.md Q3 step 5, B.1). `tested` carries `ref` and `head_sha` as well as `sha` and `base_sha`.
3. A lost lease on the land still gives one mechanical new attempt (decision note 6, sibling lane 4 row 10 unchanged). That attempt ends `base_moved` and never pushes an untested SHA.

## Open items

- run-jobs-launcher.md:315 (the wake list) and :406 (lane 1's post types) name the land results without `tested` or `base_moved`. Both still read true: a failed exit wakes the orchestrator, and `landed` copies the flakes. Lane 1's `tested` schema is judge-land lane 1 row 6. Both lines are left as they are.

## NOT VERIFIED on a live screen

Nothing; docs only.

DONE
