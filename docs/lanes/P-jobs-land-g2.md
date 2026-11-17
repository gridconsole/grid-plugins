# P-jobs-land-g2: close run-jobs gate items (d) and (e)

Lane branch lane-P-jobs-land-g2, from lane-P-jobs-land-g1 (`e5d6076f`) with lane-P-brain-g3 merged (`0d00a788`, merge commit on this lane). Docs only. Gate report: G2-P-run-jobs-decide.md, items (d) and (e). Owner answer: decision ledger of 17 Nov, "Owner answer 17 Nov ~03:35 UTC".

## Proof table

| Row | State | Proof |
|---|---|---|
| (d) one doc defines the home source | HOLDS | Owner doc is brain-board.md decision point 4, "The task home" (docs/research/brain-board.md:26-39). It derives the home from the posts (:26-28), keeps task.md `home:` as a display copy (:30), says "One source for every home check: the posts" (:32), and keeps a hand edit of `home:` that becomes one curator proposal (option C, :37 and "Decided by Abdu: V16 home edit" :490-507, merged from lane-P-brain-g3). :39 now says "This point is the one source for the task home (ledger V20)" and names the run-jobs places that point to it. The editing direction (person and agents both edit; re-read and redo; a proposal only when two edits cannot both hold) is brain-board.md:509. |
| (d) the other doc links to it | HOLDS | `grep -n -i "home" docs/research/run-jobs-launcher.md` exit 0 and `grep -n "task\.md" docs/research/run-jobs-launcher.md` exit 0. Every line that says where the home comes from points at brain-board.md: decision 6 (run-jobs-launcher.md:66), Q4 Home item (:319), Q7 `TaskRef.Home` (:385), Q7 `home:` key (:411), lane 5 Files and Depends-on cells and its pointer to brain-board's added P-loop-home rows (:647), open item (:679). The other home lines (:49, :108, :153-155, :218, :305, :320-325, :372, :423-444, :509) use "the home machine" without defining it. :324 (`grid task move` posts `task-move` and rewrites `home:`) matches brain-board.md:30 and :126. |
| (d) no contradictory sentence | HOLDS | `grep -rn "named in task.md\|Grid-owned front-matter\|Deviation from R-run-jobs" docs/` exit 1, no output. The removed sentences: run-jobs :66 "named in task.md", :319 "task.md gets `home:` … Only that machine's launchers act", :411 "`home:` is a Grid-owned front-matter key" (a person may edit it under option C), and brain-board :39 "Deviation from R-run-jobs-launcher" (it quoted the removed :66 sentence). Neither doc has the launcher rewrite a hand-edited `home:`: run-jobs :319 "A hand edit of task.md `home:` is kept and not rewritten", brain-board :37. |
| (e) lane 4 names V26 and V27 | HOLDS | docs/research/run-jobs-launcher.md:646 V column "V5, V6, V7, V9, V26, V27". |
| (e) rows 9 and 10 tagged | HOLDS | :646 "(9, V26, V27)": row 9 runs `command` and `wait` Jobs (C points 15-17, V26) and land, integrate and cleanup Jobs (Addendum 7 B, V27). "(10, V27)": row 10 is the new attempt after `push_lease_lost` of a land or integrate Job (C point 18, B5 :587), which is the V27 merge path; the V26 half of C point 18 (restart of a blocked attempt) is lane 15's, so row 10 is not tagged V26. :669 still cites "4 row (9)" for V26. |
| V12.c left as the open owner question | HOLDS | `git diff --stat e5d6076f HEAD -- docs/research/judge-land.md` exit 0, empty. run-jobs-launcher.md has no V12 text (`grep -n V12` exit 1). |
| Line numbers kept for citing docs | HOLDS | `git show --stat HEAD`: brain-board.md 1+/1-, run-jobs-launcher.md 7+/7-; every edit stayed on its line, so the gate's and the sibling notes' line numbers still hold. |
| Lane doc with a proof table | HOLDS | this file. |
| Edit only the brief's packages (docs only) | HOLDS | `git show --stat a3b44187` lists only docs/research/brain-board.md and docs/research/run-jobs-launcher.md; plus this lane doc. The code files in the merge come from lane-P-brain-g3, which the brief told this lane to merge. |
| Tests of changed packages and importers | not run, not asked | no Go package changed (`go list` has nothing to list). |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (`ok internal/cli 0.247s`); `go test -count=1 ./internal/job/lint/` exit 0 (`ok internal/job/lint 0.423s`). |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin` exit 0; `git merge --no-edit origin/main` exit 0, "Already up to date." |
| Lint | HOLDS | `make lint` exit 0 (go vet ./...). |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0. |
| New tests fail on base | not run, not asked | docs-only lane, no test added. |
| No guard, gate, cap or hold added | HOLDS | the diff adds pointers and tags only. |
| No attribution lines, no personal paths | HOLDS | `git log -1 --format=%B` has one sentence; the diff and this doc name files by repo path or file name only. |
| Whole-module test, cross-repo set | not run, not asked | run by the orchestrator at landing. |

## What changed

- docs/research/run-jobs-launcher.md: decision 6, Q4 Home item, Q7 `TaskRef.Home` and `home:` key, lane 5 and an open item now point at brain-board.md decision point 4 for the home instead of reading it from task.md; lane 4 gains V26 and V27, row 9 is tagged V26, V27 and row 10 V27.
- docs/research/brain-board.md:39: the "Deviation from R-run-jobs-launcher" paragraph becomes "This point is the one source for the task home (ledger V20)", naming the run-jobs places that point here.

## Open items

- V12.c stays the open owner question in judge-land.md (unchanged).
- The curator accept steps for a `home-differs` proposal are this note's reading, still to be confirmed by the owner (brain-board.md:498, lane P-brain-g3).
- Row 10 carries V27 only (reason in the row above). If the gate wants V26 on row 10 too, it is a one-word edit.

## NOT VERIFIED on a live screen

Nothing; docs only.

DONE
