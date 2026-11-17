# P-loop-grants-fix1: grants wired into the CLI and the contract fold

Branch `lane-P-loop-grants-fix1`, carried from `lane-P-loop-grants`, main merged twice (last at a6c5cb02).

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. CLI-level test: a granted answer folds as the person's, without a grant as the agent's; fails on lane-P-loop-grants | HOLDS | `go test -count=1 ./internal/cli -run TestPlGrf` exit 0. On 4ae90b94 (lane-P-loop-grants plus main, before the wiring), exit 1: `--- FAIL: TestPlGrfCLIPassFoldsAGrantedAnswerAsThePersons` / `pl_grf_grants_wired_test.go:75: with dana's grant in the brain J-2 is posted, want accepted as dana's answer`. The ungranted case passes on both, as it should. |
| 2. `contract.Fold` test for both cases; fails on lane-P-loop-grants | HOLDS | `go test -count=1 ./internal/contract/` exit 0. With fold.go from 4ae90b94, exit 1: `pl_grf_fold_grants_test.go:28:17: cannot use ... in call to non-variadic FoldLoop`, `:32:17: loop.PostedAs undefined (type Loop has no field or method PostedAs)`, `:35:18: cannot use ... in call to non-variadic Fold`, `FAIL internal/contract [build failed]`. |
| 3. `go test -count=1 ./internal/contract/... ./internal/cli/... ./internal/orchestrator/... ./internal/mcp/...` | HOLDS | exit 0: `ok internal/contract 3.035s`, `ok internal/cli 71.527s`, `ok internal/orchestrator 165.530s`, `ok internal/mcp 0.387s`. After the second main merge: contract, mcp, cli walk and TestPlGrf, job/lint ok; `go test -count=1 ./internal/orchestrator/... ./internal/braincmd/... ./internal/cli/...` exit 0: `ok internal/orchestrator 437.914s`, `ok internal/braincmd 9.254s`, `ok internal/cli 215.303s`. |
| 4. No refusal or new error path; no existing test deleted; no attribution lines | HOLDS | No `return err`/refusal added: fold.go only records `Loop.PostedAs`; the CLI prints one stderr line when the grant notes cannot be read and leaves `Grants` nil. `git diff --diff-filter=D --name-only origin/main HEAD` prints nothing; no existing `_test.go` changed. Attribution grep `git log --format=%B origin/main..HEAD \| grep -ciE 'co-authored-by\|claude-session'` prints `0`. |
| Importers of changed packages | HOLDS | `go list` shows about 38 packages importing internal/contract or internal/cli; `go test -count=1 ./...` exit 0 on the head before the second merge. |
| `make lint` | HOLDS | exit 0 (go vet plus gofmt). |
| `make cross` (Windows and macOS) | HOLDS | exit 0: darwin/arm64 and windows/amd64 vet and test compile ok. |
| Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 and `go test -count=1 ./internal/job/lint/` exit 0, both after `git merge origin/main` (a6c5cb02). |
| Touches | HOLDS | internal/contract/fold.go, internal/contract/pl_grf_fold_grants_test.go (new), internal/cli/orchestrator.go, internal/cli/pl_grf_grants_wired_test.go (new), this doc. testdata/golden/mcp-session.golden: merge conflict with main, taken from main and regenerated; its one-line diff to main is answer.json's by/for/grant from P-loop-grants. |
| Pinning tests broken by the fold change | HOLDS | None broke; nothing was fixed or deleted. |
| No personal paths, hosts or emails | HOLDS | The tests use temp dirs and the names dana, member, worker. |
| App walk tests, realplugin | not run, not asked | |

## What changed

- `contract.Fold(posts, grants...)` and `FoldLoop(posts, grants...)` take the grants. An answer with `for`, `by` and a `grant` that covers it for the person Job's kind counts as the person's: `Loop.PostedAs[post id]` records that author, and the guest checks read it. Without a covering grant the answer counts as the agent's own. No post is refused or dropped.
- `configureTickStore` in internal/cli/orchestrator.go sets `Store.Grants = orchestrator.LoadGrants(<brain>)`, with the brain folder resolved like internal/people does it. This covers `grid job next`, `grid job tick` and `grid worker next`.

## Name clash (ruling 3)

The adapter lane should settle the answer body's `for`/`by`/`grant` keys as built, with `by` as the agent id, and keep the envelope's `by` as `<machine>:<login>`; brain-board.md:346's `for` in the post header becomes the body key.

## Open items

- `Loop.PostedAs` is not in state.json (the cmd-feed-state schema forbids new keys and types.go is outside Touches); only `FoldLoop` callers see it.
- `feed.FoldRead` and the questions, approvals and board readers still call `Fold` without grants (internal/feed is outside Touches).
- The approval Allow check still reads the stamped author (A6, out of scope). The status-block note and project-level grants are out of scope.

## Review of P-loop-grants

Its rows hold on this branch. Its NOT DONE row (grants not wired into production) is closed by this lane.

## NOT VERIFIED

- A live launched agent answering under a grant, end to end.
