# P-jobs-land-g5: which red counts as "red again" for V12.e

Repo: plugin. Branch lane-P-jobs-land-g5, cut from lane-P-jobs-land-g4 (129cd36f). Docs only.
Doc commit: 91c84274. Merge of origin/main after it (Go code from the shared-home
lanes; it does not touch docs/research). Line numbers below are on the lane head.

Why: gate G4 on P-judge-land-decide ended GATE MISSING on one point: the design's
"If the same test goes red again later, the fix Job deletes it" had no rule saying
which red result after one fix counts as "red again", and no lane row turning it
into the delete Job.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| Brief 1: one sentence in Q4 Flaky step 3 saying which red counts as red again | HOLDS | judge-land.md:505-512, "(V12.e) A test counts as red again when, after its earlier fix Job has closed, a later landing of any change finds that test red in any way the landing reports (flaky, FLAKE-UNSURE, a bisected red, or a `preexisting` red)"; then the delete fix Job naming the covering test, and the incident exception (fixed, never deleted) |
| Brief 1: written as the orchestrator's reading of the owner's answers, not a new owner question | HOLDS | judge-land.md:511-512, "This is the orchestrator's reading of the owner's answers (the ledger, "Owner answers 17 Nov ~04:30 UTC": "V12.e yes delete flaky tests"), not a new owner question" |
| Brief 2: lane 5 row, tagged (V12.e): red again detected from the four results plus a closed fix Job naming the test | HOLDS | judge-land.md:1008, "plus (V12.e) (c) red again": fake board, test X with a closed fix Job naming X and no open Job naming X, each of flaky, FLAKE-UNSURE, bisected red and `preexisting` red gives the delete case; with no closed fix Job, or with an open Job, it does not; the (V12.e) (b) incident check applies |
| Brief 2: lane 6 row, tagged (V12.e): the orchestrator posts the delete Job from it | HOLDS | judge-land.md:1009, row "12. (V12.e)": one fix Job that deletes the test and names the covering test, one case per result; an incident test is fixed, never deleted |
| Brief 2: one acceptance-map line | HOLDS | judge-land.md:1029, "(V12.e) the design's "If the same test goes red again later..." \| 5 (V12.e) (c) + 6.12" |
| Brief 3: change nothing else | HOLDS | `git diff -U0 129cd36f HEAD -- docs/research` exit 0: three hunks only, `+505,8` (Q4 step 3, appended after :504; :498-504 unchanged), `-1000,2 +1008,2` (lanes 5 and 6, text appended at the end of each row), `+1029` (map). run-jobs-launcher.md unchanged |
| Rows: grep "red again" finds the rule in Q4 step 3, a lane 5 row, a lane 6 row and a map line, each tagged (V12.e) | HOLDS | `grep -n -o "(V12.e) A test counts as red again\|(V12.e) (c) red again\|12. (V12.e) for the design's \"If the same test goes red again\|(V12.e) the design's \"If the same test goes red again" docs/research/judge-land.md` exit 0: `505:(V12.e) A test counts as red again`, `1008:(V12.e) (c) red again`, `1009:12. (V12.e) for the design's "If the same test goes red again`, `1029:(V12.e) the design's "If the same test goes red again`. Each tag and "red again" sit on one line. On the base, `git grep -n -c "counts as red again" 129cd36f -- docs/research/judge-land.md` gives no output (no match): the old hits were only the phrase "deleted like one that is red again" |
| Lane doc with proof table, last line DONE or NOT DONE | HOLDS | this file |
| Merge main before the last proof run | HOLDS | `git fetch origin && git merge --no-edit origin/main` exit 0, "Merge made by the 'ort' strategy" (30 files from main, none in docs/research) |
| Plugin walk tests | HOLDS | after the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` → `ok github.com/gridconsole/grid-plugins/internal/cli 0.036s`, exit 0; `go test -count=1 ./internal/job/lint/` → `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.156s`, exit 0 |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0, after the merge |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`, both exit 0 with no output, after the merge |
| Tests of changed packages and their importers | not run, not asked | no Go package changed (docs only), so go list has no package to give |
| New tests fail on base | not run, not asked | docs only, no test added |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the new rows are a detection in lane 5 and a fix Job the orchestrator posts. Nothing refuses to start a Job |
| No attribution lines, no private paths | HOLDS | the commit message has none; the doc cites "the ledger" and the design's words, not paths |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs them at landing |

## What changed

- docs/research/judge-land.md:
  - Q4 Flaky step 3 (:505-512): the red-again rule, as the orchestrator's reading of the owner's answers;
  - lane 5 (:1008): row (V12.e) (c), red again from the four results plus a closed fix Job naming the test;
  - lane 6 (:1009): row 12 (V12.e), the delete fix Job naming the covering test, incident test fixed, never deleted;
  - acceptance map (:1029): one line for 5 (V12.e) (c) and 6.12.

## Open items

- Not decided by this lane, because the brief does not say it: how the delete Job sits next to the Jobs the result already leads to. For a `preexisting` red, 6.7 posts a fix Job against main; for a bisected or FLAKE-UNSURE red, 6.5 posts a rework of the change. Row 12 says the orchestrator posts one delete fix Job per result; whether it replaces 6.7's fix Job for main, and whether the 6.5 rework still goes out next to it, is left for the orchestrator (rows 5, 7 and 10 are unchanged).
- The rule follows the brief: only a closed earlier fix Job makes a red "red again". An earlier `landed` flake note without a closed fix Job counts for the flaky-again case (V12.e) (a), not for (c).

## NOT VERIFIED on a live screen

Nothing; docs only.

DONE
