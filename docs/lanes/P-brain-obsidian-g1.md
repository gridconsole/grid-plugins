# P-brain-obsidian-g1: close the decision-gate gaps in the brain decision

Lane on the plugin repo, cut from lane-P-brain-obsidian-decide. Docs only:
docs/research/brain-obsidian.md. Nothing is built. It answers gate
G-P-brain-obsidian-decide (GATE MISSING, five items) and the V17 and secrets
notes from that gate.

## Proof table

| Row | Status | Proof |
|---|---|---|
| 1. Owned V-numbers V13, V14, V19 listed at the top of the Decision block, each with its sections | HOLDS | docs/research/brain-obsidian.md:21-35 (table "Owned ledger rows"); commit e1cd144c |
| 2. Every build lane names its V-numbers and takes acceptance rows from them | HOLDS | "Ledger rows" line on L1 :829, L2 :858, L3 :884, L4 :910, L5a :953, L5b :985, L6 :1016, L7 :1037, L8 :1053. Every acceptance bullet is tagged `(Vnn)`, with rows owned elsewhere marked "owned by" (V17, V25). `grep -c -E "V1[349]"` gives 54 (exit 0); before this lane it gave no output. Commit 13d3a418 |
| 3. V13: how a note shows who wrote it in Obsidian; the deviation stated | HOLDS | Section 4.5 :423-466. Grid-written notes carry a flat `source` (task and Job) shown by Properties. A person note gets no header, so no header means you. Records are authoritative, and `source` is display only and goes stale after a person edit. Decision 4 :50-53; deviation in 7.2 :699-702; question for the owner in 9.5 :1181. L3 acceptance rows (V13, 4.5). Commit fa34f962 |
| 4. V14 rows: (i) a person edit to a `Proposals/` draft adopted as person input and used by the curator; (ii) accept removes the proposal and keeps its bytes | HOLDS | 5.1 :478 (adoption walks `Proposals/`; versions path for a proposal). 6.2 :574-605 (curator reads the current bytes, keeps the person's lines; accept record, removal, bytes in versions). L4 row (V14) for adoption. L5a rows (V14, 6.2 person input) :971, (V14, 6.2 accept) :976 and reject. Commit 6ba35475 |
| 5. V19: the lane that carries the sync-server version-check half, with an acceptance row | HOLDS | New lane L8 P-brain-sync-write :1051-1094 with rows (V19, sync check 1) :1077 and (V19, sync check 2) :1081 against an httptest stub, plus the local-lock row. Order in 9.3. 9.5 :1174 tells the orchestrator that no lane yet owns the server's own version-checked write (test-plan sync checks 1-2). Commit d91b9a24 |
| 6a. L5b aligned with V17: WebP or optimised PNG, no JPEG | HOLDS | 7.3 :714-746: lossless optimised PNG through `shot.EncodeSmall` / `grid shot crop --small`, no quantisation, WebP rejected for now (no stdlib encoder). L5b Builds :993 and rows (V17); 9.4 step 7. `grep -n -iE "jpeg\|jpg"` hits only the "no JPEG" statements, the L5b no-`image/jpeg` row and the 9.5 note quoting R-brain-board. Commit f8262588 |
| 6b. Secrets check on person notes: report-only, with file:line | HOLDS | 7.1 :660-678 cites internal/brain/ln_secret.go:6, note.go:199-208, store.go:232-235 and check.go:158. The check gives a finding with file and line and empty text, and never redacts, rewrites, proposes or blocks. L4 rows (V13, 7.1) :942. Known limit (versions sync) in 9.5. Commit f8262588 |
| 7. Commit after each step; last line DONE or NOT DONE; copy to the hand folder as R-P-brain-obsidian-g1.md | HOLDS | Six step commits e1cd144c..f8262588. `tail -n 3` shows the last line `DONE`. Copied with `cp` after this doc |
| Edit only the named file | HOLDS | `git diff --stat 593bca3a HEAD -- docs/research/brain-obsidian.md`: 1 file. The only other files are this lane doc and the origin/main merge |
| Small commits, one sentence, no attribution | HOLDS | `git log origin/main..HEAD`: no Co-Authored-By or session lines |
| No private paths, hosts or emails in committed files | HOLDS | `grep -n -iE "/srv\|/home/\|abdu\|@[a-z]+\.\|grid3\|\.claude"` over the doc: no output (exit 1) |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Docs only. The secret check reports only. L8 falls back to a proposal and exits 0, and never blocks the agent |
| Merge origin/main before the last proof run | HOLDS | `git fetch -q origin` then `git merge --no-edit origin/main`: merged (judge-land docs), exit 0 |
| Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/cli 0.173s`, exit 0 |
| Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/job/lint 0.940s`, exit 0 |
| Lint `make lint` | HOLDS | `go vet ./...`, exit 0 |
| `GOOS=windows go build ./...` | HOLDS | exit 0, no output |
| `GOOS=darwin go build ./...` | HOLDS | exit 0, no output |
| Tests of changed packages and their importers | not run, not asked | No Go package changed (docs only), so `go list` has no importer set |
| New tests fail on base, pass on lane | not run, not asked | Docs only; no test added |
| Whole-module test, cross-repo set | not run, not asked | Run by the orchestrator at landing |

## What changed

- docs/research/brain-obsidian.md:
  - an owned-rows table at the top;
  - ledger rows and tagged acceptance on L1-L7;
  - new section 4.5 (who wrote a note, as shown in Obsidian) with its
    deviation in 7.2;
  - adoption walks `Proposals/` (5.1);
  - the draft-edit and accept rules (6.2) and their rows in L4 and L5a;
  - new lane L8 P-brain-sync-write;
  - 7.3 as lossless optimised PNG (no JPEG);
  - the report-only secret check on person notes (7.1);
  - 9.3 order and 9.5 open items.
- docs/lanes/P-brain-obsidian-g1.md: this file.

## Open items for the orchestrator

- V13 question for the owner (9.5): is "no Grid header = you" acceptable,
  or should Grid add `source: <owner name>` on adoption?
- V19 server half: no lane in any plan builds the sync server's
  version-checked write (test-plan sync checks 1-2). L8 goes live only once
  that lane exists.
- V17: R-brain-board's proof-path section still says "png or jpg" because
  of the old 7.3. The board lane needs the same change; the V17 gate reads
  that doc.
- Secret in an adopted person note: purging old versions is not decided.
- The 7.3 claim about whether Go's PNG encoder drops alpha on its own is
  NOT VERIFIED (the Go source outside the lane folder was not readable
  here); the L5b builder checks it.

## NOT VERIFIED on a live screen

Nothing here needs a live screen. The Mac round trip (9.4) stays as written
for after L6.

DONE
