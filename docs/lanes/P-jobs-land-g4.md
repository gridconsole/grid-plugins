# P-jobs-land-g4: V12.e acceptance rows in the judge-land lane plan

Repo: plugin. Branch lane-P-jobs-land-g4, cut from lane-P-jobs-land-g3 (048b380d). Docs only.
Doc commits: d2627b48 (the rows, the map, the open item, RJ:315, JL:20) and
54dd6679 (the lane 5 tags made the literal "(V12.e)", and lane 6 row 11 given
the open-Job exception). Merge of origin/main: 31be7d0b (README.md and a lane
doc from main; neither touches the two research docs). Line numbers below are on
the lane head.

Why: gate G3 on P-judge-land-decide ended GATE MISSING, item 1: the owner's
V12.e answer (decision ledger of 17 Nov, ~04:30: "V12.e yes delete flaky
tests") is decided in judge-land.md Q4 Flaky step 3 but no build lane carried
it as acceptance rows.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| Brief 1: lane 5 row (a), tagged "(V12.e)": the second occurrence is found from a closed earlier fix Job or a flake note that names the test, not only from open Jobs | HOLDS | judge-land.md:1000, "plus (V12.e) (a) the second occurrence". The open-Job rule is checked first, so 6.6 still holds; then a closed fix Job, then an earlier `landed` flake note gives the delete case |
| Brief 1: lane 5 row, the incident check that the open item gave lane 5 | HOLDS | judge-land.md:1000, "plus (V12.e) (b) the incident check". The test's doc comment or a brain decisions note names an incident: fix case, never delete; with neither: delete case |
| Brief 2: lane 6 row (b), tagged "(V12.e)": on the second time the orchestrator posts a fix Job that deletes the test and names the test that still covers its behaviour | HOLDS | judge-land.md:1001, row "10. (V12.e)" |
| Brief 2: lane 6 row (c), tagged "(V12.e)": a test that pins a past incident gets a fix Job whose acceptance says fix, never delete | HOLDS | judge-land.md:1001, row "11. (V12.e)", for example `TestRealPluginE0812TaskStopClosesTheJobForGood` |
| Brief 3: one line per new row in the acceptance map | HOLDS | judge-land.md:1017 (5 (V12.e) (a)), :1018 (6.10), :1019 (5 (V12.e) (b)), :1020 (6.11) |
| Brief 3: close the open item on "pins a past incident" if the rows cover it | HOLDS | judge-land.md:1058-1062 "**Closed (V12.e):**", pointing at lane 5 row (V12.e) (b) and lane 6 row 11. The rule stays as it was (doc comment or brain decisions note). The owner's possible marker word is kept as a remark |
| Brief 4: fold in the one-line doc edit, RJ:315 wake list naming `tested` with flakes and `base_moved` | HOLDS | run-jobs-launcher.md:315 "the land results `conflict`, `red`, `preexisting` and `base_moved`, a `tested` with flakes, and a `landed` with flakes". The phrase "a `landed` with flakes" is kept, so the quote at judge-land.md:968-972 still reads true |
| Brief 4: other one-line notes folded | HOLDS | G2 note 1: judge-land.md:20 V26 now reads "Q6 lanes 1, 9 and 10". Lane 1 row 5 is tagged (V26) at :996 |
| Brief 4: the rest are left as they are and listed | HOLDS | see "Gate notes left as they are" below |
| Rows: grep "(V12.e)" finds rows in lane 5, lane 6 and the acceptance map | HOLDS | `grep -n -o -F "(V12.e)" docs/research/judge-land.md`, exit 0: 3 hits on :1000 (lane 5, the row starting `\| 5 \| **P-land-tests**`), 4 on :1001 (lane 6, `\| 6 \| **P-land-launch**`), :1017-1020 (map), :1058-1059 (closed open item). `grep -n -E "V12\.e [ab]\)"` on judge-land.md: exit 1, no old-style tag left |
| Rows: every row quotes the owner rule as the ledger states it | HOLDS | The quoted strings "V12.e yes delete flaky tests", "a test that is flaky again on a later land, after one fix, is deleted like one that is red again, naming the test that still covers its behaviour" and "a test that pins a past incident is fixed, never deleted" are found by `grep -n -F` in the ledger, exit 0, at ledger :69 and :71. `grep -c -F` with the same three strings on judge-land.md prints `6`: lines 1000, 1001, 1017, 1018, 1019 and 1020, which are every new row |
| Rows: no other decision changes | HOLDS | `git diff -U0 048b380d d2627b48 \| grep "^@@"` exit 0 shows five hunks (54dd6679 edits only lines 1000, 1001, 1017, 1019 and 1059 inside them): judge-land.md `-20 +20`, `-1000,2 +1000,2` (lanes 5 and 6), `+1017,4` (the map), `-1054,3 +1058,5` (the open item), and run-jobs-launcher.md `-315 +315`. No field is added to the `tested` schema (lane 1 row 6, :996, unchanged). Q4 Flaky step 3 (:498-504) is unchanged, and the rows restate it |
| Sibling gate G3-P-run-jobs-decide: apply its MISSING items | HOLDS (none to apply) | its Verdict: "GATE PASS. All 14 rows are PRESENT, and no row is MISSING or CONTRADICTED". It was read again after it changed on disk during this lane, with the same verdict. Its notes (RJ:446, :406 and others) are left |
| Lane doc with proof table, last line DONE or NOT DONE | HOLDS | this file |
| Merge main before the last proof run | HOLDS | `git fetch origin` exit 0; `git merge --no-edit origin/main` exit 0, "Merge made by the 'ort' strategy", README.md and docs/lanes/P-run-jobs-a7-fix1.md only |
| Plugin walk tests | HOLDS | after the merge, on 54dd6679: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` → `ok github.com/gridconsole/grid-plugins/internal/cli 0.089s`, exit 0; `go test -count=1 ./internal/job/lint/` → `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.410s`, exit 0 (also green on 31be7d0b: 0.170s and 0.785s) |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0, on 31be7d0b; 54dd6679 changes docs only |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`, both exit 0 with no output, on 31be7d0b; 54dd6679 changes docs only |
| Tests of changed packages and their importers | not run, not asked | no Go package changed (docs only), so go list has no package to give |
| New tests fail on base | not run, not asked | docs only, no test added |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the new rows are fix Jobs the orchestrator posts and a check in `flake.go`. Nothing refuses to start a Job |
| No attribution lines, no private paths | HOLDS | the commit message has none; the docs cite "the decision ledger of 17 Nov" and "gate G3", not paths |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs them at landing |

## What changed

- docs/research/judge-land.md:
  - lane 5 (:1000): rows (V12.e) (a), the second occurrence, and (V12.e) (b), the incident check;
  - lane 6 (:1001): rows 10 (V12.e), the delete fix Job naming the covering test, and 11 (V12.e), the incident fix Job (none when an open Job already names the test, as 6.6);
  - the acceptance map (:1017-1020): one line for each new row;
  - the open item on "pins a past incident" is closed (:1058-1062);
  - :20: V26 lists lane 1 (G2 note 1).
- docs/research/run-jobs-launcher.md:315: the wake list names `base_moved` and a `tested` with flakes.

## Gate notes left as they are

- G2 note 2: JL:526 says `done` edge for a cross-task wait, while RJ C19 names `landed`. This is rule text, not a one-line index fix.
- G2 note 3: lane 3 row 8 does not say where `judgement_default_flow` is recorded (against :278). This is rule text.
- G2 note 4: lane 2 row 8 gives every rework the head base, while RJ C5 and B.5 give the secret rework `carry`. This is rule text.
- G2 note 5: RJ:536 (and :449, :454) cite R-judge-land.md line ranges. The right targets need a read of that file; this is not a one-line edit.
- Lane 6 row 8 also depends on RJ 1 row 4. The "Depends on" cell already covers it, so there is nothing to change.
- Sibling gate notes (RJ:446, RJ:406, V21 wording, name differences, V26 owner options, BB:498, lane 5 V column) are not this brief's.

## Open items

- A reading this lane made, flagged for the reviewer and not decided by the owner. The brief says "a flake note that names the test"; lane 5 row (V12.e) (a) counts only a flake note in an earlier change's `landed` post, following the ledger's "flaky again on a later land". An earlier `tested` post of the same change does not count, because a retest after `base_moved` would otherwise read its own first flake as a second one. A flake seen only in a `tested` post still counts once its fix Job is closed.

## NOT VERIFIED on a live screen

Nothing; docs only.

DONE
