# P-jobs-land-g2: close run-jobs gate items (d) and (e)

Lane branch lane-P-jobs-land-g2, from lane-P-jobs-land-g1 (`e5d6076f`) with lane-P-brain-g3 merged (`0d00a788`, merge commit `311ae312`). Gate report: G2-P-run-jobs-decide.md, items (d) and (e). Owner answer: decision ledger of 17 Nov, "Owner answer 17 Nov ~03:35 UTC".

Lane commits: `a3b44187` (items d and e), `152eaaf4` (this doc), `2bf92eef` (README fix), `1927f8ab` (lane 5 wording), and the commit that updates this doc.

## Proof table

| Row | State | Proof |
|---|---|---|
| (d) one doc defines the home source | HOLDS | Owner doc is brain-board.md decision point 4, "The task home" (docs/research/brain-board.md:26-39). It derives the home from the posts (:26-28), keeps task.md `home:` as a display copy (:30), says "One source for every home check: the posts" (:32), and keeps a hand edit of `home:`, which becomes one curator proposal (option C, :37 and "Decided by Abdu: V16 home edit" :490-507, merged from lane-P-brain-g3). :39 now says "This point is the one source for the task home (ledger V20)" and names the run-jobs places that point to it. The editing direction (person and agents both edit; re-read and redo; a proposal only when two edits cannot both hold) is brain-board.md:509. |
| (d) the other doc links to it | HOLDS | `grep -n -i "home" docs/research/run-jobs-launcher.md` exit 0 and `grep -n "task\.md" docs/research/run-jobs-launcher.md` exit 0. Every line that says where the home comes from points at brain-board.md: decision 6 (run-jobs-launcher.md:66), Q4 Home item (:319), Q7 `TaskRef.Home` (:385), Q7 `home:` key (:411), lane 5 Files and Depends-on cells and its pointer to all of brain-board's added P-loop-home rows (:647), open item (:679). The other home lines (:49, :108, :153-155, :218, :305, :320-325, :372, :423-444, :509) use "the home machine" without defining it. :324 (`grid task move` posts `task-move` and rewrites `home:`) matches brain-board.md:30 and :126. |
| (d) no contradictory sentence | HOLDS | `grep -rn "named in task.md\|Grid-owned front-matter\|Deviation from R-run-jobs" docs/research/` exit 1, no output. The removed sentences are run-jobs :66 "named in task.md", :319 "task.md gets `home:` … Only that machine's launchers act", :411 "`home:` is a Grid-owned front-matter key" (a person may edit it under option C), and brain-board :39 "Deviation from R-run-jobs-launcher" (it quoted the removed :66 sentence). Neither doc has the launcher rewrite a hand-edited `home:`: run-jobs :319 "A hand edit of task.md `home:` is kept and not rewritten", brain-board :37. |
| (e) lane 4 names V26 and V27 | HOLDS | docs/research/run-jobs-launcher.md:646, V column "V5, V6, V7, V9, V26, V27". |
| (e) rows 9 and 10 tagged | HOLDS | :646 "(9, V26, V27)": row 9 runs `command` and `wait` Jobs (C points 15-17, V26) and land, integrate and cleanup Jobs (Addendum 7 B, V27). "(10, V27)": row 10 is the new attempt after `push_lease_lost` of a land or integrate Job (C point 18, B5 :587), which is the V27 merge path. The V26 half of C point 18 (the restart of a blocked attempt) is lane 15's, so row 10 is not tagged V26. :669 still cites "4 row (9)" for V26. |
| V12.c left as the open owner question | HOLDS | `git diff --stat e5d6076f HEAD -- docs/research/judge-land.md` exit 0, empty. run-jobs-launcher.md has no V12 text (`grep -n V12 docs/research/run-jobs-launcher.md` exit 1). |
| Line numbers kept for citing docs | HOLDS | `git show --stat a3b44187`: brain-board.md 1+/1-, run-jobs-launcher.md 7+/7-. The later lane 5 wording fix changes one line in place. Every edit stayed on its line, so the gate's and the sibling notes' line numbers still hold. |
| Lane doc with a proof table | HOLDS | this file. |
| Docs only | HOLDS | `git log --oneline --name-only origin/lane-P-jobs-land-g1..HEAD --no-merges` exit 0: this lane's commits `a3b44187`, `152eaaf4`, `2bf92eef` and `1927f8ab` touch only docs/research/brain-board.md, docs/research/run-jobs-launcher.md, docs/lanes/P-jobs-land-g2.md and README.md. The other commits in that range are lane-P-brain-g3's history; their code is already on origin/main (none of their Go files appear in `git diff --name-only origin/main HEAD`). `git diff --name-only origin/main HEAD` also lists internal/cli and internal/runner Go files and help.golden. They come from `43d26ad5` ("Add grid run"), which is an ancestor of the g1 base (`git merge-base --is-ancestor 43d26ad5 e5d6076f` exit 0), not from this lane. |
| README.md (outside the two docs) | HOLDS, inherited fix | On the g1 base, `go test -count=1 ./internal/cli/ ./internal/runner/` failed: `--- FAIL: TestLtReadmeNamesEveryCommand`, `lt_readme_groups_test.go:47: README.md ## Commands does not name \`grid run\``. The test came in `d91c6bb8` and `grid run` in `43d26ad5`, and both are ancestors of `e5d6076f`. Fix: README.md:32 names `grid run` in the task group. Rerun: `go test -count=1 ./internal/cli/` exit 0 (`ok internal/cli 96.054s`). No test was edited. |
| Tests of changed packages and importers | HOLDS | No Go file changed in this lane. For the code the bundle range carries from the g1 base: `go test -count=1 ./internal/cli/` exit 0; internal/runner has no test files. Importers from `go list -f "{{.ImportPath}} {{join .Imports \" \"}}" ./... \| grep -E "internal/(cli\|runner)( \|$)"`: cmd/grid (imports cli), internal/cli (imports runner), lib (imports cli). `go test -count=1 ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no test files; `ok lib 26.417s`). |
| Walk tests (plugin), on the final head | HOLDS | `go test -count=1 -v ./internal/cli -run "TestHsIpWalksEveryCommand\|TestIlGuestSiteList"` exit 0: `--- PASS: TestHsIpWalksEveryCommand (0.00s)` ("walked 41 commands, 41 table entries"), `--- PASS: TestIlGuestSiteList (0.03s)`, `ok internal/cli 0.073s`. `go test -count=1 ./internal/job/lint/` exit 0, `ok internal/job/lint 0.411s`. |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin` exit 0; `git merge --no-edit origin/main` exit 0, "Already up to date." |
| Lint | HOLDS | `make lint` exit 0 (go vet ./...), run on `a3b44187`. Later commits changed no Go file. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0 and `GOOS=darwin go build ./...` exit 0, both on `a3b44187`. Later commits changed no Go file. |
| New tests fail on base | not run, not asked | Docs-only lane; no test was added. |
| No guard, gate, cap or hold added | HOLDS | The diff adds pointers, V tags and one README line only. |
| No attribution lines | HOLDS | `git log --format=%B origin/lane-P-jobs-land-g1..HEAD \| grep -ci "co-authored\|claude-session"` prints 0. |
| No personal paths | HOLDS | The diff and this doc name files by repo path or file name only. |
| Whole-module test, cross-repo set | not run, not asked | The orchestrator runs these at landing. |

## What changed

- docs/research/run-jobs-launcher.md now points at brain-board.md decision point 4 for the task home instead of reading it from task.md. This covers decision 6, the Q4 Home item, the Q7 `TaskRef.Home` and `home:` key lines, lane 5 (which takes all of brain-board's added P-loop-home rows) and one open item.
- The same file tags lane 4: its V column gains V26 and V27, row 9 carries V26 and V27, and row 10 carries V27.
- docs/research/brain-board.md:39: the "Deviation from R-run-jobs-launcher" paragraph now reads "This point is the one source for the task home (ledger V20)" and names the run-jobs places that point here.
- README.md:32 names `grid run`. This fixes the README walk test, which was already red on the g1 base.

## Open items

- V12.c stays the open owner question in judge-land.md, which is unchanged.
- The curator accept steps for a `home-differs` proposal are brain-board's reading, and the owner has not confirmed them yet (brain-board.md:498, lane P-brain-g3).
- Row 10 carries V27 only, for the reason in its row above. If the gate wants V26 on row 10 too, that is a one-word edit.
- The README fix is outside the two gate docs. It is there because the g1 base's `grid run` commit left TestLtReadmeNamesEveryCommand red, and the whole-module test would fail at landing without it.

## NOT VERIFIED on a live screen

Nothing; docs only.

DONE
