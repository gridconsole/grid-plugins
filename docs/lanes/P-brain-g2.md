# P-brain-g2: close the second-gate gaps of brain-obsidian and brain-board

Repo: plugin. Docs only; no product code changed. Started from lane-P-brain-obsidian-g1 (head 64c163a5), then merged origin/lane-P-brain-board-g1 (no conflict: the two lanes touch different files) and, before the last proof run, origin/main (no conflict).

Choice on step 3: the brief allowed "match brain-board.md:479 exactly (no JPEG, no lossy step), or change both docs to one set and say why". The board row kept a bounded 256-colour palette step, which is lossy, so copying it would break the brief's own "no lossy step" and the wording the obsidian gate closed V17 on. This lane took the second branch: both docs now carry one lossless set (`grid shot compress`, same pixel size, no scaling, no palette step, no JPEG). The reason is written in both docs: the owner's "Compression must not alter what the proof shows" (brain-obsidian Addendum 5.2).

## Proof table

| Row | State | Where / command |
| --- | --- | --- |
| Do 1. V19 direct agent Edit/Write: the not-yet-adopted person save is versioned before the edit (4.3/5.4) | HOLDS | brain-obsidian.md:388-430 (4.3: the PreToolUse hook adopts the one target file first; unconditional; records only, no permission decision, exit 0; residual window stated as NOT VERIFIED); :506-508 (5.1 "When"); :592-596 (5.4 step 7); :28 (V19 row of the owned table) |
| Do 1. Acceptance row in L4, tagged V19 | HOLDS | brain-obsidian.md:989-999 (the (V19, 4.3 direct edit) row: name with a space, `history-log` off, no task folder; person bytes in `versions/`, note restored, agent bytes in `Proposals/`, hooks exit 0 with no decision even when adoption fails); L4 Builds and Files :974-983 |
| Do 2. 9.5 names the sync-server lane with rows, "not in this plugin plan" | HOLDS | brain-obsidian.md:1241-1271 (A-sync-version-write, app repo, rows for sync checks 1, 2 and 3 plus create-only); L8 :1125-1127, :1152-1154; 9.3 :1179-1181; board sync row names the same lane, brain-board.md:484 |
| Do 3. V17: one row set, no JPEG, no lossy step; the docs do not disagree | HOLDS | brain-board.md:289 (why), :286-302 (Q5), :481 (L5b row), :486; brain-obsidian.md:747-799 (7.3), :1067-1076 (L5b rows), :1278-1282 (9.5). `grep -o -F` with the 8 row sentences on both files: each sentence found once in each doc (16 matches, exit 0) |
| Do 4. P-loop-home inbox row relabelled V23; sync row V16/V23/V17 | HOLDS | brain-board.md:478 ("V23: two homes write two inbox sections"; V column V16, V23, V9); :484 (V16 task.md, V23 `Needs you.md`, V17 `proof/`; V column V15, V16, V23, V17, V9); :13-16 (owned table and the V23 carried line) |
| Do 5. V16 `home:` hand edit left open, options with their lane rows | HOLDS | brain-board.md:490-506 ("Open for Abdu: V16 home edit": options A, B, C, each with the exact P-loop-home row and other rows it changes, and how the answer goes in with one edit); :37 and :478 no longer decide the rewrite and point at the section |
| Do 6. Commit after each step | HOLDS | d15265b9 (1), ceae58d8 (2), 6881fd5d (3), d88507f1 (4), bfd42829 (5) |
| Do 6. Report at the hand folder, each gate item with doc:line, last line DONE/NOT DONE | HOLDS | R-P-brain-g2.md in the hand folder (copy of this file) |
| Merge of lane-P-brain-board-g1, conflict resolution stated | HOLDS | merge commit after 64c163a5; no conflict |
| Do not build | HOLDS | `git diff --stat origin/main...HEAD` lists only docs/ files (4 files: two lane docs from the merged lanes, brain-board.md, brain-obsidian.md), plus this lane doc |
| Repo-wide walk tests (plugin) after merging main | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok, exit 0; `go test -count=1 ./internal/job/lint/` ok, exit 0 |
| `git fetch origin && git merge origin/main` before the last proof run | HOLDS | merge made, no conflict |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the V19 hook step records only: brain-obsidian.md:400-405 |
| No attribution lines; no private paths, hosts or emails in committed files | HOLDS | `grep -n -E "/srv/|/home/|@gmail"` on both docs: no match |
| New tests fail on base / tests of changed packages, imports, lint, Windows and macOS builds | not run, not asked | docs-only lane; no Go package changed |
| Guest walk tables (`make realplugin ...`) | not run, not asked | app-repo check; this is the plugin repo |

## What changed

- brain-obsidian.md: 4.3, 5.1, 5.4 step 7 and L4 decide and test the pre-edit adoption for an agent's own Edit or Write (V19). 9.5, L8 and 9.3 name A-sync-version-write as the server half. 7.3, L5b and 9.5 carry the one lossless V17 set. L5b no longer names V14 (L5a proves it), per the gate's note on L5b.
- brain-board.md: Q5 drops the palette step and says why; the L5b row, :14 and :486 carry the one set without stale line pointers. The P-loop-home and sync rows carry V16, V23 and V17 labels. Decision point 4 and P-loop-home no longer decide the `home:` rewrite; "Open for Abdu: V16 home edit" holds the three options.

## Open items

- V16 `home:` hand edit: a question for Abdu (brain-board.md:490-506). The P-loop-home brief waits on it.
- A-sync-version-write is named but has no brief; the orchestrator writes it in the app repo.
- V13/4.5 owner question (no Grid header means you wrote it) is still open, unchanged (brain-obsidian 9.5).

## NOT VERIFIED

- Whether Claude Code's Edit and Write refusal of a file changed since Read covers every person save in the window between the pre-tool adoption and the tool's write (brain-obsidian 4.3).
- Whether lossless UI screenshots at 1440 and 3840 wide reach the 300 KB target (L5b measures it).
- Nothing in this lane needs a live screen.

DONE
