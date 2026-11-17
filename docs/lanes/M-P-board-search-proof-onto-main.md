# M-P-board-search-proof-onto-main: lane P-board-search-proof merged onto plugin main

Carries P-board-search-proof (docs/lanes/P-board-search-proof.md). Merged origin/main (8aec52da) into the lane; one conflict, in internal/brain/query.go.

## Conflict resolution

- `Hit` struct: main added `Conflict []string` (person-note conflicts, brain-obsidian 6.3); the lane added `Kind string`. Both fields kept. The lane's `Kind` constants, `Kinds`, `ValidKind`, `postTypes`, `entryKind`, `kindClass` and `proposalEntries` stay after the struct, which the conflict markers had glued into one block.
- Sort in `SearchKind`: the lane's knowledge-before-history sort (class, then score, stable) runs first, then main's `if s.Vault() { hits = personFirst(hits, entries) }`, so a person note still comes before the agent notes that supersede it and `Conflict` is still filled. `personFirst` only moves a conflicting agent hit behind its person note; every other hit keeps the knowledge-first order.
- Nothing dropped from either side: no test, row or error code removed.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. `git merge-base --is-ancestor origin/main HEAD` and diff names only lane files | HOLDS | exit 0. `git diff origin/main --stat` lists 12 files: agent-templates/grid-library.md, agents/library.md, contract/commands.json, contract/schemas/cmd-brain-search.json, docs/lanes/P-board-search-proof.md, internal/brain/pbsp_search_test.go, internal/brain/query.go, internal/braincmd/command.go, internal/proof/pbsp_shot_test.go, internal/proof/shot.go, testdata/golden/brain-help.golden, testdata/golden/brain-search-help.golden. All are files P-board-search-proof touched (its lane doc "What changed"). |
| 2a. P-board-search-proof row 1: knowledge ranks above history for an equal score | HOLDS | `go test -count=1 -v -run 'TestPbSp\|TestSessionStartPrintsTheIndexNotNoteBodies' ./internal/brain/ ./internal/proof/ ./internal/hook/`: `--- PASS: TestPbSpKnowledgeRanksAboveHistoryForEqualScore`, exit 0. |
| 2b. row 2: `--kind` works | HOLDS | same run: `--- PASS: TestPbSpKindFilter`, `--- PASS: TestPbSpProposalKind`. |
| 2c. row 3: SessionStart index-only test passes | HOLDS | same run: `--- PASS: TestSessionStartPrintsTheIndexNotNoteBodies`. |
| 2d. row 4: png and webp land at `proof/J-n-rowN.<ext>`, linked from the post | HOLDS | same run: `--- PASS: TestPbSpPNGAndWebPLandNextToThePost`, `--- PASS: TestPbSpSecondShotOfARowKeepsTheFirst`. |
| 2e. row 5: jpg and one-frame gif stored as png, same decoded pixels | HOLDS | same run: `--- PASS: TestPbSpJPEGAndSingleFrameGIFAreStoredAsPNG` with sub-tests shot.jpg, shot.jpeg, shot.gif, misnamed.png; `--- PASS: TestPbSpAnimatedGIFIsNotAScreenshot`, `--- PASS: TestPbSpShotRefusesBadInput`. |
| Standing: `go test -count=1 ./internal/brain/... ./internal/proof/... ./internal/hook/...` | HOLDS | exit 0: `ok .../internal/brain`, `ok .../internal/proof`, `ok .../internal/hook`. |
| Standing: contract inventory and walk tests | HOLDS | `go test -count=1 -v -run 'TestContractCommandsInventory\|TestContractCommandsParserInventory\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList' ./internal/cli/`: `--- PASS:` for each of the four; exit 0. |
| 3. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | `-v` run: every test `--- PASS`, `ok .../internal/job/lint`, exit 0. |
| 3. Walk tests of `./internal/cli` as a whole package | HOLDS | `ok .../internal/cli 44.416s` in the importers run below. |
| 3. App-repo walk tests (`make webui-test`, TestFvArgvCollects, TestEveryWiredMount, slices Imports, realplugin) | not run, not asked (app repo, not this plugin lane) | |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 4. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 `go vet ./...` and test compile of every package. |
| Changed packages and every importer | HOLDS | Changed: internal/brain, internal/proof, internal/braincmd. Importers of brain, proof, hook (from `grep -rl` of the import path in *.go): people, packaging, braincmd, cli, land, projectenv, golden, job, job/jobcmd, job/jobtest, routine, task, workflow, machinefolder, contract/formats. `go test -count=1 ./internal/people/... ./internal/packaging/... ./internal/braincmd/... ./internal/land/... ./internal/projectenv/... ./internal/golden/... ./internal/job/... ./internal/routine/... ./internal/task/... ./internal/workflow/... ./internal/machinefolder/... ./contract/... ./internal/cli/...`: every line `ok`, exit 0. |
| New tests fail on base | HOLDS (carried) | Unchanged from docs/lanes/P-board-search-proof.md row "New tests fail on base" (`h.Kind undefined`, `undefined: KindNote` on base query.go; `AddShot`, `PostBody`, `ErrAnimated` absent on base). The merge adds no new test. |
| No existing test edited or deleted | HOLDS | The merge touched no `_test.go` of either side; main's tests for `personFirst` and the vault reader pass in the runs above. |
| CLOSED BY rewrite of NOT DONE in carried lane docs | HOLDS | `grep -in "not done" docs/lanes/P-board-search-proof.md` prints nothing, so there was no NOT DONE verdict, row or last line to rewrite. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Nothing added; the merge only combines two data fields and two sorts. |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` printed `0`. |
| Pushing | HOLDS | Nothing pushed; bundle only. |
| Guest walk tables (`make realplugin ...`) | not run, not asked (app repo) | |

## What changed

Only `internal/brain/query.go` needed a hand edit (see Conflict resolution); the rest is main's merge. Merge commit: "Merge origin/main into the board-search-proof lane, keeping Kind and Conflict on Hit and the knowledge-first sort before personFirst."

## Open items (carried from P-board-search-proof)

- Nothing calls `AddShot` or `PostBody` from a command yet; a follow-up Job wires `grid proof add --row`.
- internal/hook/brain_search.go (`parseBrainSearch`) does not know `--kind`.
- A combined vault search with `--kind` and `personFirst` is covered only by each side's own tests, not by one test of both; no row asks for it.

## NOT VERIFIED on a live screen

Nothing was seen in the app or a browser; every row is proven by go tests and make targets.

DONE
