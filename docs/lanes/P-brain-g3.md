# P-brain-g3: Abdu's V16 answer and the editing direction in both brain docs

Lane: P-brain-g3, plugin repo, branched from lane-P-brain-g2 and merged with origin/main before the last proof. It changes only docs. Source: the owner answer of 17 Nov, about 03:35 UTC, at the end of the decision ledger of 17 Nov, and the P-brain-g2 report.

## Proof table

| Row | Verdict | Proof |
| --- | --- | --- |
| Do 1: brain-board "Open for Abdu: V16 home edit" replaced by the decision, option C, Abdu quoted | HOLDS | brain-board.md:490-510 ("Decided by Abdu: V16 home edit"): the case, Abdu's words verbatim at :494, what Grid does at :496, how the curator settles it at :498, the rows it sets at :500-503, what did not change at :505, options not taken at :507, and the same direction for every file at :509. Options A and B and "How the answer goes in" are removed. A grep for "Open for Abdu" on both docs finds nothing |
| Do 1: P-loop-home rows match | HOLDS | brain-board.md:478: the placeholder V16 row is replaced by option C's row (task.md outside the markers unchanged, exactly one `task: <T>` proposal naming both machines, no second proposal on a second wake). "the brief is not written before the answer" is gone |
| Do 1: :35 matches | HOLDS | The "When they differ" paragraph is brain-board.md:37; that is the brief's :35, because :35 itself ("Authority already comes from the posts") says nothing about the edit. :37 now says the edit is kept and a proposal is written; "not decided" and "Until it is answered" are gone. Also :13: the V16 owned row says kept plus proposal and adds L5b to the lanes |
| Do 1: option C's "other rows" (the L5b curator row) | HOLDS | brain-board.md:481 (L5b: V column V17, V16; the V16 curator row at its end, quoted again at :502); brain-obsidian.md:1109-1110 (L5b ledger rows name V16) and :1136-1137 (the same row). `grep -c -F` of the full V16 row sentence gives brain-board.md:2 and brain-obsidian.md:1, exit 0 |
| Do 2: V19 and every place a person edit stops, locks out or refuses an agent | HOLDS | brain-obsidian.md: :28 (V19 owned row, amended by the owner answer); :62-74 (Decision 6 "Both edit; no blind overwrite", with the owner quoted); :128-132 (the old "person notes win" moved to Rejected, with the owner's words); :182 (section 1 safety list: an agent writes a person note only on top of the bytes it read); :272 (reader table: the curator and skills carry edits on top, not "person authority"); :428-435 (4.3: the agent's bytes stand, nothing is restored and no proposal is written; the tool's stale-read refusal is the check); :492-496, :500-502 and :513 (4.5: no header written, an agent edit adds none, and the text no longer leans on "decision 6 forbids"); :600-611 (5.4 step 3, the one rule whoever wrote the bytes); :615-621 (5.4 step 5); :622-625 (step 6); :626-631 (step 7); :635-651 (6.1, renamed "Person and agent edits on one note"; deletes follow the same rule); :686-694 (6.3: the curator applies an edit on top, and a proposal waits only when it cannot); :1240 (9.2); :1283-1290 (9.4 step 6 "Both edit"); :1337-1339 (9.5, the server row: the client redoes on top); :1359-1374 (9.5: what was applied, and what was checked and left, with why). brain-board.md: :37, :496 and :509 (the V16 edit is kept, and the same direction is stated for every file); :271 (Q4 step 4 already re-reads and redoes on top, so it is unchanged). A grep of both docs for `person note\|writes nothing\|never writes\|not rewrit` and for `person notes win\|person-first rule\|restores the person\|decision 6 forbids\|turns the agent write\|person wins\|person authority\|never applies\|beat agent` was checked hit by hit. Every hit that is left is either the Rejected entry, search ranking, the secret check (report only), the V16 curator row or the 4.5 header rule. No other place was found |
| Do 2: version check kept (no blind overwrite) | HOLDS | brain-obsidian.md:62-74: every `grid brain` write carries the version it read, and none goes over bytes it has not read; a direct Edit or Write relies on the tool's stale-read refusal, with the two uncovered cases in 4.3. :600-611: "It never overwrites silently" |
| Do 2: L4 acceptance rows | HOLDS | brain-obsidian.md:1004-1006 (ledger row); :1014-1016 (Builds); :1031-1038 (F9 split: on other lines both edits hold and the written path is returned; on the same key a proposal is written and the person bytes are unchanged); :1039-1042 (an agent write to a person note on its current rev is written on top, with no proposal); :1043-1053 (4.3 direct edit: person bytes in `versions/`, the note on disk equals the agent's bytes, no `Proposals/` file, hooks exit 0) |
| Do 2: L8 acceptance rows | HOLDS | brain-obsidian.md:1179-1182 (ledger row); :1193-1198 (Builds: redo on top, and bounded retries keep the change as a proposal, 5.4 step 5); :1209-1220 (sync check 2 split into other lines, which return the written path, and same lines, which return `proposed:`; both F9 tests run under the local lock) |
| Do 3: commit after each step | HOLDS | fe1a48a9 (step 1), 24f315ce (step 2), 95372dcb (merge of origin/main), 5a8236ac (fixes from the self-check), then this doc |
| Do 3: report with doc:line, last line DONE or NOT DONE | HOLDS | This file, copied to the hand folder as R-P-brain-g3.md and P-brain-g3-report.md |
| Do not build | HOLDS | `git diff --stat origin/main...HEAD` lists only docs/lanes/P-brain-g2.md (from the lane this one is based on), docs/research/brain-board.md, docs/research/brain-obsidian.md and this file |
| Walk test: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/cli 0.047s`, exit 0, on the final tree after the merge |
| Walk test: `go test -count=1 ./internal/job/lint/` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.174s`, exit 0, on the final tree after the merge |
| `git fetch origin && git merge origin/main` before the last proof | HOLDS | Merge commit 95372dcb, no conflict |
| New tests fail on base and pass on the lane | not run, not asked | Docs only; no test added |
| Tests of changed packages and their importers, lint, Windows and macOS builds | not run, not asked | No Go package changed |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The lane removes the restore-and-propose step after a direct edit and the person-origin redirect. The hooks still only record (brain-obsidian.md:400-421, "It records only" at :415) |
| No attribution lines; no private paths, hosts or emails in committed files | HOLDS | Commit messages are one sentence with no trailer. `git diff -U0 origin/main` over docs/research and this file, with the added lines counted for the VM's root folder, a home folder or an at sign: count 0 (grep exit 1, no match) |
| UI rows | not run, not asked | No UI |

## What changed

- brain-board.md: the V16 home edit is decided (option C), with Abdu's words. :13, :37, :478 and :481 carry the decision, and the open section with its options table is gone.
- brain-obsidian.md: Decision 6 and every section that stopped an agent on a person edit now follow the owner's direction. An agent reads the current file and carries on; on a mismatch it re-reads and redoes its change on top; a proposal is written only when both edits cannot hold. The L4, L5b and L8 rows and step 6 of the Mac round trip match.

## Open items and choices this lane made (for the owner to confirm)

- **How the curator settles a `home-differs` proposal** (brain-board.md:498). It never writes `home:` and never runs `grid task move`. It records the proposal `accepted` (6.2 accept steps 2 to 4, with no note filed) once the two values agree again. Option C left this open, and Abdu's answer does not say it in words. The rule comes from "Nothing moves a task by itself" (run-jobs Addendum 4.2) and from the answer keeping the edit.
- **The P-loop-home brief must pick the proposal writer.** Its Files and Depends-on cells in Q9 name none yet (brain-board.md:498).
- **The testable form of "both edits cannot hold"** (brain-obsidian.md:600-611). The agent's patch touches a front-matter key or body section that the other change also changed. This is the old agent-against-agent rule, now applied whoever wrote the bytes. It is stricter than "same lines, opposite intent": two same-intent edits to one section also become a proposal.
- **Deleting a person note** (brain-obsidian.md:647-651) follows the same rule. This change has the largest effect: an agent that read the current bytes may delete a person note, and the bytes stay in `versions/`.
- **Bounded rejected retries against the sync server keep the change as a proposal** (brain-obsidian.md:615-621). This is a second proposal case, kept so that no work is lost.
- **A direct Edit or Write now has one check.** Before, a restore step backed up Claude Code's stale-read refusal; now nothing does. A person save that lands between the agent's Read and its Edit, if the tool does not refuse it, is written over; its bytes stay in `versions/` only if the pre-tool adoption ran. No new mechanism was added, so no new guard either. This is in 4.3's uncovered cases (brain-obsidian.md:400-440) and stays NOT VERIFIED.
- **`proposed:` and "the written path"** are output forms this note sets. Neither literal exists in internal/brain or internal/braincmd today; L4 picks the exact text.

## Checked and left as they are

These were checked and left: the local machine lock, the server refusing a stale base, the secret check, person-first ranking with L7's conflict display, immutable `board/` posts with F6, and the status block replaced inside its markers. None of them stops an agent on a person edit (brain-obsidian.md:1359-1374). The board's Q4 step 4 already re-reads and redoes on top (brain-board.md:271).

## NOT VERIFIED

- Whether Claude Code's stale-read refusal covers every person save between an agent's Read and its Edit (brain-obsidian.md 4.3).
- Nothing in this lane needs a live screen.

DONE
