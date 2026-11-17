# P-brain-g2: close the second-gate gaps of brain-obsidian and brain-board

Repo: plugin. Docs only; no product code changed. Started from lane-P-brain-obsidian-g1 (head 64c163a5), then merged origin/lane-P-brain-board-g1 (no conflict: the two lanes touch different files) and, before the last proof run, origin/main (no conflict). The decision ledger changed on disk during the run; I re-read it: the V16, V17, V18, V19 and V23 rows still name the same "Decided in" lanes, so nothing here moved.

Choice on step 3: the brief allowed "match brain-board.md:479 exactly (no JPEG, no lossy step), or change both docs to one set and say why". The board row kept a bounded 256-colour palette step, which is lossy, so copying it would break the brief's own "no lossy step" and the wording the obsidian gate closed V17 on. This lane took the second branch: both docs now carry one lossless set (`grid shot compress`, same pixel size, no scaling, no palette step, no JPEG). The reason is written in both docs: the owner's "Compression must not alter what the proof shows" (brain-obsidian Addendum 5.2).

## Gate items

| Gate item | State | Doc:line that closes it |
| --- | --- | --- |
| Obsidian gate, V19 (b): how a not-yet-adopted Obsidian save is versioned before an agent's own Edit/Write (brief Do 1) | HOLDS | brain-obsidian.md:386-431 (4.3: the PreToolUse hook adopts the one target file before the tool writes; unconditional on session id, `history-log`, task folder and `ValidName`; records only, no permission decision, exit 0; lock tried for at most 500 ms, then skipped and logged; uncovered cases stated, NOT VERIFIED where unsourced); :59 (Decision 5 now names this trigger); :513-515 (5.1 "When"); :599-603 (5.4 step 7); :28 (V19 row of the owned table) |
| Obsidian gate, V19 (a): an L4 acceptance row tagged V19 (brief Do 1) | HOLDS | brain-obsidian.md:996-1008 (the (V19, 4.3 direct edit) row: a name with a space, `history-log` off, no task folder; person bytes in `versions/`, note restored byte-equal, agent bytes in `Proposals/`, hooks exit 0 with no decision when adoption fails and when the lock is busy); L4 Builds :981-983 |
| Obsidian gate, routed: the V19 server half needs a named lane (brief Do 2) | HOLDS | brain-obsidian.md:1250-1283 (9.5: lane A-sync-version-write, "not in this plugin plan", rows for sync checks 1, 2 and 3 plus create-only; repo is the sync server's, evidence points to the app, the orchestrator confirms); L8 :1134-1136 and :1161-1163; 9.3 :1188-1190; brain-board.md:484 names the same lane |
| Obsidian gate, check 2 note: L5b names V14 but carries no V14 row | HOLDS | brain-obsidian.md L5b ledger rows (just under :1056): V14 dropped, "proved in L5a, not here" |
| Board gate note 2 / brief Do 3: V17 one row set, no JPEG, no lossy step; the two docs must not disagree | HOLDS | brain-board.md:289 (why), :288-300 (Q5), :481 (L5b row), :486; brain-obsidian.md:754-806 (7.3), :1076-1085 (L5b rows), :1290-1294 (9.5). `grep -o -F` with the 8 row sentences on both files: each found once in each doc (16 matches, exit 0) |
| Board gate item (b): stale pointers to R-brain-obsidian.md:818-835 / :605-621 and the "JPEG item" / `crop --jpeg` override wording | HOLDS | brain-board.md:14 and :486 now point by section name (9.1, 7.3); no `crop`, `--jpeg` or "JPEG item" left (`grep -n -i -E "palette\|quantis\|1568\|crop\|jpeg item"` shows only the "why" sentence and the no-scaling lines) |
| Board gate note 3 / brief Do 4: inbox row to V23, sync row to V16/V23/V17 | HOLDS | brain-board.md:478 ("V23: two homes write two inbox sections"; V column V16, V23, V9); :484 (V16 task.md, V23 `Needs you.md`, V17 `proof/`; V column V15, V16, V23, V17, V9); :13-16 (owned table lanes and the V23 carried line) |
| Board gate verdict, V16 `home:` hand edit: left for Abdu with options (brief Do 5) | HOLDS | brain-board.md:490-506 ("Open for Abdu: V16 home edit": A rewrite, B keep and raise a finding, C keep and write a curator proposal; each with the exact P-loop-home row and the other rows it changes, and how the answer goes in with one edit); :37 and :478 no longer decide the rewrite and point at the section |
| Board gate notes 1, 4 and 5 | not run, not asked | not in the brief; untouched |

## Lane rules

| Rule | State | Where / command |
| --- | --- | --- |
| Commit after each step | HOLDS | d15265b9 (1), ceae58d8 (2), 6881fd5d (3), d88507f1 (4), bfd42829 (5), 12db9617 (fixes after review of my own pass), then this lane doc |
| Report in the hand folder, last line DONE/NOT DONE | HOLDS | R-P-brain-g2.md and P-brain-g2-report.md in the hand folder (copies of this file) |
| Merge of lane-P-brain-board-g1, conflict resolution stated | HOLDS | merge commit after 64c163a5; no conflict |
| Do not build | HOLDS | `git diff --stat origin/main...HEAD`: only docs/ files (two lane docs from the merged lanes, brain-board.md, brain-obsidian.md, this lane doc) |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS | merge made, no conflict |
| Repo-wide walk tests (plugin), at the final doc content | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'`: `ok github.com/gridconsole/grid-plugins/internal/cli 0.054s`, exit 0; `go test -count=1 ./internal/job/lint/`: `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.204s`, exit 0 |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the V19 hook step records only and never waits past its bound: brain-obsidian.md:401-408 |
| No attribution lines; no private paths, hosts or emails in committed files | HOLDS | `grep -n -E "/srv/\|/home/\|@gmail"` on both docs: no match |
| New tests fail on base / tests of changed packages and importers, lint, Windows and macOS builds | not run, not asked | docs-only lane; no Go package changed |
| Guest walk tables (`make realplugin ...`) | not run, not asked | app-repo check; this is the plugin repo |

## What changed

- brain-obsidian.md: Decision 5, 4.3, 5.1, 5.4 step 7 and L4 decide and test the pre-edit adoption for an agent's own Edit or Write (V19). 9.5, L8 and 9.3 name A-sync-version-write as the server half. 7.3, L5b and 9.5 carry the one lossless V17 set. L5b no longer names V14.
- brain-board.md: Q5 drops the palette step and says why; the L5b row, :14 and :486 carry the one set without stale line pointers. The P-loop-home and sync rows carry V16, V23 and V17 labels; the sync row names A-sync-version-write. Decision point 4 and P-loop-home no longer decide the `home:` rewrite; "Open for Abdu: V16 home edit" holds the three options.

## Open items

- V16 `home:` hand edit: a question for Abdu (brain-board.md:490-506). The P-loop-home brief waits on it.
- A-sync-version-write is named but has no brief, and its repo is to be confirmed (the app, by the evidence in 9.5).
- The V13/4.5 owner question (no Grid header means you wrote it) is still open, unchanged (brain-obsidian 9.5).

## NOT VERIFIED

- Whether Claude Code's Edit and Write refusal of a file changed since Read covers every person save in the window between the pre-tool adoption and the tool's write, and whether Obsidian's File recovery keeps overwritten bytes (brain-obsidian 4.3).
- Whether lossless UI screenshots at 1440 and 3840 wide reach the 300 KB target (L5b measures it).
- Nothing in this lane needs a live screen.

DONE
