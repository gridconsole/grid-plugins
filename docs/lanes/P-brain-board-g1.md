# P-brain-board-g1: close the decision-gate gaps in the brain board decision

Lane: P-brain-board-g1, plugin repo, docs only. Edited file: docs/research/brain-board.md (plus this lane doc). No product code, no build.

## Proof table

| Row | Verdict | Proof |
| --- | --- | --- |
| 1 (a) Owned V-numbers V9, V15, V16, V17 at the top, each with its section | HOLDS | docs/research/brain-board.md, section "Owned ledger rows" (table after the Base paragraph). The gate's own `grep -nE "\bV[0-9]+\b\|ledger"` now hits (17 V-number lines). Commit dc6d1510. |
| 2 (b) Every Q9 lane names its V-numbers and takes acceptance rows from them, with a V17 compression row | HOLDS | Q9 table: a V column on P-loop-board, B1, B3, P-loop-home, P-loop-retire, B5, A1, the P-shared-home requirement and the sync requirement; every acceptance row starts with its V-number. V17 compression rows are on the added L5b P-brain-curator-shots row, and the paragraph under the table says Q5 overrides R-brain-obsidian 7.3 and L5b's `--jpeg` item. Commit 1a5d3fcd. |
| 3 (c) V15: what the post body holds and which [[links]] it carries, with an example | HOLDS | Q8, "Post body and links": five rules (one renderer from header and body, layout, back-links only, path-qualified link form with `## Links` rels from internal/brain/note.go:87, vault reader edges), a per-type table, and an example post. Also decision point 3, the Q4 status block link, F4, Q7 step 3, the Q8 vault reader bullet and B1/B3 rows. Commit 1ca32678. |
| 4 V17 format: WebP or optimised PNG, no JPEG; tool, size target, readability check | HOLDS | Q5 "Format" and "Compression is the curator's": the owner's words quoted, jpg and single-frame gif stored as png at `grid proof add` so post links never change, tool `grid shot compress` (lossless re-encode, palette only when it passes the check, same pixel size, same path), 300 KB target as an aim with `over_target` reported, readability checked in the tool, in L5b tests and by a person in the Mac round trip. Decision point 8 updated. `grep -nE "jpg\|jpeg\|JPEG"` shows only the no-JPEG rule and the jpg-to-png conversion. Commit c256f5eb. |
| 5 Gate note 2 (home source) settled with evidence; note 3 (Excluded files) dropped or explained | HOLDS | Decision point 4, "One source for every home check: the posts", citing run-jobs Addendum 4.2, Addendum 2.3 and R-run-jobs-launcher Q4 "Task move"; the Q3 stream.go row now names the fold's home; P-loop-home and P-loop-retire rows test it. Q2 point 5: the Excluded files fallback is dropped, with why. Commit 1398ddb8. |
| Review fixes after the steps | HOLDS | Commit 1a22d7ce: renderer inputs named (header, body, own path, linked file names from the names cache; unknown name renders the id), type table and example fields taken from contract/schemas (question: `question`, `options`, `why`, `who`, `blocking`; author role `agent`; no `verdict` type), V9 lane list in the top table matches Q9, every acceptance row prefixed, L5b cited at R-brain-obsidian.md:818-835 and :605-621, a fixture with more than 256 colours, and the home rule flagged as a deviation from R-run-jobs-launcher. |
| 6 Commit after each step; last line DONE; copy to hand as R-P-brain-board-g1.md | HOLDS | Five step commits above plus the fix commit. `tail -1 docs/research/brain-board.md` printed `DONE`. `cp` to the hand folder as R-P-brain-board-g1.md, then `diff` printed nothing (exit 0). |
| Small one-sentence commits, no attribution lines | HOLDS | `git log --format=%H%n%B origin/main..HEAD`: one sentence each, no trailer lines. |
| No private paths, hosts or emails in committed files | HOLDS | `grep -nE "/srv\|/home/\|@"` on the doc prints nothing. |
| New tests fail on base and pass on the lane | not run, not asked | Docs-only lane; the brief says do not build. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The size target is an aim, not a limit (Q5); the readability bound only chooses the encoder output and blocks no agent. |
| Tests of changed packages and importers | not run, not asked | No Go package changed. |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0. |
| Windows and macOS builds | not run, the brief forbids it | The brief says "Do not build"; no Go file changed. |
| Merge origin/main before the last proof run | HOLDS | `git fetch origin` and `git merge --no-edit origin/main` (brought docs/research/judge-land.md and docs/lanes/P-judge-land-decide.md), exit 0. |
| Walk tests (plugin) | HOLDS | After the merge: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok 0.162s; `go test -count=1 ./internal/job/lint/` ok 0.706s. |
| Whole-module test and cross-repo set | not run, not asked | Run by the orchestrator at landing. |

## What changed

- docs/research/brain-board.md: owned ledger rows table; V-traced Q9 lane plan with an L5b row for V17; Q8 "Post body and links" with a per-type table and example; Q5 format WebP or optimised PNG with `grid shot compress`, the 300 KB target and the readability check; decision point 4 settles the home source as the posts; the Excluded files fallback is dropped.

## Open items

- The gate marked V17 CONTRADICTED and called it a question for the owner. The brief settled it by the owner's own words (WebP or optimised PNG, no JPEG). R-brain-obsidian section 7.3 and its L5b item still say `--jpeg`; this note overrides them, but that note itself should be amended by its owner lane.
- Whether 1440 and 3840 wide UI screenshots reach 300 KB without scaling is not measured; L5b records it.
- The readability bound (99.5% of pixels within 16 per channel, none over 48) is a starting value for L5b to pin on fixtures.

## NOT VERIFIED

- Obsidian rendering of post bodies, path-qualified links (also inside a larger vault), the status callout and relative image links.
- A compressed screenshot read at 100% in Obsidian (the Mac round trip, R-brain-obsidian 9.4 step 7).

DONE
