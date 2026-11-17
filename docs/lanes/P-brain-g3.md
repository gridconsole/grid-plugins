# P-brain-g3: Abdu's V16 answer and the editing direction in both brain docs

Lane: P-brain-g3, plugin repo, branched from lane-P-brain-g2 and merged with origin/main before the last proof. It changes only docs. Source: the owner answer of 17 Nov, about 03:35 UTC, at the end of the decision ledger of 17 Nov, and the P-brain-g2 report.

## Proof table

| Row | Verdict | Proof |
| --- | --- | --- |
| Do 1: brain-board "Open for Abdu: V16 home edit" replaced by the decision, option C, Abdu quoted | HOLDS | brain-board.md:490-510 ("Decided by Abdu: V16 home edit": the case, Abdu's words verbatim at :494, what Grid does at :496, how the curator settles it at :498, the rows it sets, what did not change, options not taken, the same direction for every file). Options A and B and "How the answer goes in" are removed. `grep -n "Open for Abdu"` on both docs: no hit |
| Do 1: P-loop-home rows match | HOLDS | brain-board.md:478: the placeholder V16 row is replaced by option C's row (task.md outside the markers unchanged, exactly one `task: <T>` proposal naming both machines, no second proposal on a second wake); "the brief is not written before the answer" is gone |
| Do 1: :35 matches | HOLDS | The "When they differ" paragraph is at brain-board.md:37 (the brief's :35; :35 itself, "Authority already comes from the posts", holds nothing about the edit). :37 now says the edit is kept and a proposal is written; "not decided" and "Until it is answered" are gone. Also :13 (V16 owned row: kept, proposal, L5b added to the lanes) |
| Do 1: option C's "other rows" (L5b curator row) | HOLDS | brain-board.md:481 (L5b row: V column V17, V16; the V16 curator row at its end); brain-obsidian.md:1104-1105 (L5b ledger rows name V16) and :1131-1132 (the same V16 row word for word) |
| Do 2: V19 and every place a person edit stops, locks out or refuses an agent | HOLDS | brain-obsidian.md :28 (V19 owned row, amended by the owner answer), :62-72 (Decision 6 "Both edit; no blind overwrite", owner quoted), :126-130 (old "person notes win" moved to Rejected with the owner's words), :269 (reader table: curator and skills carry edits on top, not "person authority"), :425-432 (4.3: the agent's bytes stand, nothing restored, no proposal; the tool's stale-read refusal is the check), :495-497 and :508 (4.5: no longer leans on "decision 6 forbids"), :595-606 (5.4 step 3: the one rule, whoever wrote the current bytes; a proposal only when the patch touches a key or section the other change also changed), :610-620 (5.4 steps 5 and 6), :621-626 (5.4 step 7), :630-646 (6.1 renamed "Person and agent edits on one note"; deletes follow the same rule), :681-689 (6.3: the curator applies an edit on top; a proposal waits only when it cannot), :1235 (9.2), :1278-1285 (9.4 step 6 "Both edit"), :1332-1334 (9.5 server row, client redoes on top), :1354-1369 (9.5: what was applied, and what was checked and left, with why) |
| Do 2: version check kept (no blind overwrite) | HOLDS | brain-obsidian.md:62-72 ("Every agent write carries the version it read ... Nothing writes over bytes it has not read"), :595-606 ("It never overwrites silently"), :621-626 (direct edits: the stale-read refusal, NOT VERIFIED as before at 4.3) |
| Do 2: L4 acceptance rows | HOLDS | brain-obsidian.md:999-1001 (ledger row), :1009-1011 (Builds), :1026-1033 (F9 split: other lines, both hold, `written:`; same key, proposal, person bytes unchanged), :1034-1037 (agent write to a person note on its current rev is written on top, no proposal), :1038-1048 (4.3 direct edit: person bytes in `versions/`, note on disk equals the agent's bytes, no `Proposals/` file, hooks exit 0) |
| Do 2: L8 acceptance rows | HOLDS | brain-obsidian.md:1174-1177 (ledger row), :1187-1193 (Builds: redo on top; bounded retries keep the change as a proposal, 5.4 step 5), :1204-1215 (sync check 2 split into other lines, `written:`, and same lines, `proposed:`; both F9 tests under the local lock) |
| Do 3: commit after each step | HOLDS | fe1a48a9 (step 1), 24f315ce (step 2), then the merge of origin/main and this doc |
| Do 3: report with doc:line, last line DONE or NOT DONE | HOLDS | this file, copied to the hand folder as R-P-brain-g3.md and P-brain-g3-report.md |
| Do not build | HOLDS | `git diff --stat origin/main...HEAD`: only docs/lanes/P-brain-g2.md (the lane this one is based on), docs/research/brain-board.md, docs/research/brain-obsidian.md, and this file |
| Walk test: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/cli 0.319s`, exit 0, after the merge of origin/main |
| Walk test: `go test -count=1 ./internal/job/lint/` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.496s`, exit 0, after the merge of origin/main |
| `git fetch origin && git merge origin/main` before the last proof | HOLDS | merge commit 95372dcb, no conflict |
| New tests fail on base and pass on the lane | not run, not asked | docs only; no test added |
| Tests of changed packages and importers, lint, Windows and macOS builds | not run, not asked | no Go package changed |
| No guard, gate, cap, hold or blocking hook added | HOLDS | the lane removes the restore-and-propose step after a direct edit and the person-origin redirect; the hooks still record only (brain-obsidian.md:397-418, "It records only" at :412) |
| No attribution lines; no private paths, hosts or emails in committed files | HOLDS | commit messages are one sentence with no trailer; `grep -n "/srv/\|@" docs/lanes/P-brain-g3.md` shows none |
| UI rows | not run, not asked | no UI |

## What changed

- brain-board.md: the V16 home edit is decided (option C) with Abdu's words; :13, :37, :478 and :481 carry it; the open section and its options table are gone.
- brain-obsidian.md: Decision 6 and every section that stopped an agent on a person edit now follow the direction: read the current file and carry on, on a mismatch re-read and redo on top, a proposal only when both edits cannot hold. L4, L5b and L8 rows and the Mac round trip step 6 match.

## Choices this lane made (for the owner to confirm)

- How the curator settles a `home-differs` proposal (brain-board.md:498): it never writes `home:` and never runs `grid task move`, and it records the proposal `accepted` once the two values agree again. Option C left this open; Abdu's answer does not say it in words. It is read from "Nothing moves a task by itself" (run-jobs Addendum 4.2) and from the answer keeping the edit.
- The testable form of "both edits cannot hold" (brain-obsidian.md:595-606): the agent's patch touches a front-matter key or body section that the other change also changed. This is the old agent-against-agent rule, now used whoever wrote the bytes. It is stricter than "same lines, opposite intent": two edits to one section with the same intent also become a proposal.
- Deleting a person note (brain-obsidian.md:642-646) follows the same rule. This is the change with the largest effect: an agent that read the current bytes may delete a person note, and the bytes stay in `versions/`.
- After bounded rejected retries against the sync server, the change is kept as a proposal (brain-obsidian.md:610-616). This is a second proposal case, kept so that work is never lost.

## Checked and left as they are

The local machine lock, the server refusing a stale base, the secret check, person-first ranking and L7's conflict display, immutable `board/` posts and F6, and the status block replaced inside its markers. None of them stops an agent on a person edit (brain-obsidian.md:1354-1369). The board's Q4 step 4 already re-reads and redoes on top (brain-board.md:271).

## NOT VERIFIED

- Whether Claude Code's stale-read refusal covers every person save between an agent's Read and its Edit (brain-obsidian.md 4.3, as before).
- Nothing in this lane needs a live screen.

DONE
