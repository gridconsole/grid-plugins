# M-P-loop-grants-fix1-onto-main: lane P-loop-grants-fix1 onto plugin main 8d1c595a

Carries P-loop-grants-fix1 (which carries P-loop-grants). Merge commit aa2c4a98 on lane-M-P-loop-grants-fix1-onto-main; `git fetch origin` after the merge brought nothing new.

## What changed

One conflict, `internal/contract/fold.go`. Main turned `FoldLoop` into a thin wrapper of `foldLoop(posts, info)` and added `FoldBoard`, `BoardPost`, `boardInfo`, `Loop.Home`. The lane gave `Fold` and `FoldLoop` a `grants ...Grant` parameter and added `Loop.PostedAs`. Resolution keeps both:

- `Loop` has `Allow`, `Landed`, `PostedAs` (lane) and `Home` (main).
- `FoldLoop(posts, grants...)` calls `foldLoop(posts, nil, grants)`; `foldLoop` takes `(posts, info, grants)`, so the lane's PostedAs fold body (fold.go:380-387) reads `grants`.
- `FoldBoard(posts)` calls `foldLoop(plain, info, nil)`: a per-post board counts every answer as its stamped author, as on main. It fails safe: no post is refused or dropped.

Open item: `internal/feed/board.go:181` (`contract.FoldBoard(posts)`) is a production reader of a per-post board, and it passes no grants, so the grant rule is absent on that path. internal/feed is outside this lane's Touches, and the brief's rows name the CLI Store and `contract.Fold`; a later lane that wires grants into the board reader should give `FoldBoard` a `grants ...Grant` parameter (the `foldLoop` signature already takes them).

Proofs ran on code head aa2c4a98; the doc commit that follows changes no code.
- The `time` import stays (PostedAs compares `at` with a grant's `until`).

No test, row or error code of either side was dropped. Carried NOT DONE verdicts in docs/lanes/P-loop-grants.md (Touches row, last line) and docs/lanes/P-loop-grants-fix1.md rewritten to CLOSED BY M-P-loop-grants-fix1-onto-main with the reason.

The name clash (`by` in the envelope is `<machine>:<login>`; brain-board.md:346 puts `for` in the post header): the adapter lane should settle one shape; as built, `for`/`by`/`grant` are answer body keys.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. `git merge-base --is-ancestor origin/main HEAD` | HOLDS | exit 0 (after a second `git fetch origin`) |
| 1. `git diff origin/main --stat` names only lane P-loop-grants-fix1 files | HOLDS | 15 files: contract/schemas/answer.json, internal/cli/orchestrator.go, internal/cli/pl_grf_grants_wired_test.go, internal/contract/fold.go, pl_gr_posted_as_test.go, pl_grf_fold_grants_test.go, internal/mcp/pl_gr_author_test.go, internal/mcp/tools.go, internal/orchestrator/person.go, person_grants.go, pl_gr_person_grant_test.go, store.go, testdata/golden/mcp-session.golden, docs/lanes/P-loop-grants.md, P-loop-grants-fix1.md (the lane doc of this lane is added after) |
| P-loop-grants-fix1 row 1: CLI-level test, granted answer folds as the person's, ungranted as the agent's | HOLDS | `go test -count=1 -v ./internal/cli -run GrantsWired\|Grf`: PASS TestPlGrfCLIPassFoldsAGrantedAnswerAsThePersons, PASS TestPlGrfCLIPassFoldsAnUngrantedAnswerAsTheAgents, exit 0. The failing-on-base output is pasted in docs/lanes/P-loop-grants-fix1.md:9 (`pl_grf_grants_wired_test.go:75`, exit 1 before the wiring); not rerun here |
| P-loop-grants-fix1 row 2: `contract.Fold` test, both cases | HOLDS | PASS TestPlGrfFoldCountsAGrantedAnswerAsThePersons, exit 0. The failing-on-base output (build failed, non-variadic `Fold`) is pasted in docs/lanes/P-loop-grants-fix1.md:10; not rerun here |
| P-loop-grants-fix1 row 3: `go test -count=1 ./internal/contract/... ./internal/cli/... ./internal/orchestrator/... ./internal/mcp/...` | HOLDS | exit 0: contract ok 1.619s, cli ok 63.893s, orchestrator ok 210.121s, mcp ok 0.326s |
| P-loop-grants-fix1 row 4: no refusal or new error path; no test deleted | HOLDS | `git diff origin/main --numstat -- "*_test.go"`: five files, all `N 0` (internal/cli/pl_grf_grants_wired_test.go 83 0, internal/contract/pl_gr_posted_as_test.go 102 0, pl_grf_fold_grants_test.go 64 0, internal/mcp/pl_gr_author_test.go 47 0, internal/orchestrator/pl_gr_person_grant_test.go 150 0); no deletions |
| No attribution lines | HOLDS | `git log --format=%B origin/main..HEAD \| grep -ciE "co-authored-by\|claude-session\|generated with"` printed 0 |
| Packages that import the changed ones | HOLDS | `go test -count=1 ./...` exit 0 on aa2c4a98, every package ok (it covers every importer of contract, orchestrator, cli and mcp; the orchestrator runs the full whole-module test again at landing) |
| 3. Walk test `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, ok 0.189s |
| 3. Walk test `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, ok 0.646s |
| 3. App-repo walk tests (`make webui-test`, plugincli, grid-server, slices; `make realplugin ... cmd/grid-server`) | not run, not asked (they are app-repo tests; the runbook lists them for the app repo only) |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 4. `make cross` | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile, all packages ok |
| Tests named *RealPlugin* | not run, not asked |

## NOT VERIFIED on a live screen

Nothing here has a UI. That a real launched agent's `grid feed post` answer lands as the person's end to end is still not run (as in P-loop-grants-fix1).

DONE
