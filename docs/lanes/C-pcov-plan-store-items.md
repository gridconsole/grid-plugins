# C-pcov-plan-store-items: proof table

Repo: plugin. Branch lane-C-pcov-plan-store-items. Change: one new file, `internal/plan/store_items_test.go`. No non-test file touched, no existing test edited, nothing deleted.

Coverage, Linux, `go test -cover ./internal/plan/ -count=1`:
- before (untouched clone): `ok  github.com/gridconsole/grid-plugins/internal/plan  0.062s  coverage: 82.0% of statements`
- after: `ok  github.com/gridconsole/grid-plugins/internal/plan  0.066s  coverage: 90.3% of statements`

Per-function, after (`go tool cover -func`): Store.Items 93.1%, DigestCoversLines 100.0%, Store.Migrate 66.7%, directory 90.9%, atomicWrite 78.6%, childDirectory 90.0%.

| Row | Status | Proof |
|---|---|---|
| 1. Items on a current plan equals plan.Items(text) | HOLDS | `TestStoreItemsCurrentPlanOnly` (store_items_test.go): reflect.DeepEqual against `WithLegacyState(Items(text),"",nil)`, and against `Items(text)` field-wise (only State, which is "open" on both). `go test -run TestStoreItems ./internal/plan/` exit 0 |
| 2. items.md + subagents fold in, .txt ignored, no writes | HOLDS | `TestStoreItemsFoldsLegacyStateWithoutWriting`: items.md marks i-1 "done" (gets State "done"); a `.md` brief running for item 4 gives "working"; a `.md` brief with status done for item 2 leaves "open" (WithLegacyState sets State only, and only "running" briefs change it; items.md gives the rest); `two-running.txt` (running, item 2) is ignored, so i-2 stays "open". Walk of the task folder (path, mode, mtime, size) is identical before and after; mtimes were set 48h back first. Exit 0 |
| 3. Error paths | HOLDS | `TestStoreItemsErrors`: items.md a directory -> `items.md must be a regular file`; plan front matter -> `plan line 1: front matter is refused; use checklist items with jobs lists`; `subagents/x.md` a directory -> `x.md must be a regular file`; **subagents as a regular file -> error `subagents must be a real directory`** (childDirectory store.go:51-53; it is not read as absent). Also no plan.md -> ErrNotExist, empty TaskDir -> error. Exit 0 |
| 4. Migrate on a current plan | HOLDS | `TestMigrateCurrentPlanIsNoOp`: form "current", Written zero, nil error, plan.md bytes and mtime equal, folder snapshot unchanged. Exit 0 |
| 5. Migrate on legacy plans | HOLDS | `TestMigrateLegacyPlan`: form-b.md -> form "B", plan.md parses with Items, Written.Hash equals Hash(new plan.md), second run returns "current"; form-a.md -> form "A", error `plan write requires Used: <note> or Recall: nothing found`, Written zero, plan.md byte-identical (migrate.go:41-43); `---\nnot: a plan\n` -> Migrate error equals the Upgrade error, plan.md untouched; no plan.md -> ErrNotExist, form "". Exit 0 |
| 6. approval.json path | not in scope (dropped by the brief) | migrate.go:44-66 stays uncovered |
| 7. DigestCoversLines | HOLDS | `TestDigestCoversLines` (16 cases): (0,1), (3,2), (-2,5) false; step lines 31-32 true; ticked step line 19 true (and 19-23 with its prose); status prose line 5 and range 2-8 false; heading line 3 false; leading blank line (1,1) false on `"\n# Plan\n\n- [ ] 1 x\n"` while (2,2) true; trailing blank false; past the end false. Exit 0 |
| 8a. go test ./internal/plan/ -count=1: 0 fail | HOLDS | exit 0, 90.3% |
| 8b. coverage: Items and DigestCoversLines above 80%, Migrate above 50% | HOLDS | Items 93.1%, DigestCoversLines 100.0%, Migrate 66.7% |
| 8c. GOOS=windows / darwin `go test -c -o /dev/null ./internal/plan/` | compiled, needs Windows and macOS runs | both exit 0 on Linux. Run on a Windows host: `go test ./internal/plan/` (tests skip via testStore on Windows, the DigestCoversLines test does not); on macOS: `go test ./internal/plan/` |
| 8d. make lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/plan` empty |
| Packages importing plan | HOLDS | `go list` importers: cli, guard, job, job/jobtest, jobgraph, land, orchestrator, proof, task. `go test -count=1` on all: exit 0 (jobtest has no test files) |
| New tests fail on base | NOT APPLICABLE | The tests describe existing behaviour; on base there is no `store_items_test.go`, so there is nothing to fail. I did not mutate non-test code to show the tests bite (the rule says tests only), so that is not shown |
| Tests only, no guard/gate/cap | HOLDS | `git diff --stat origin/main..HEAD`: store_items_test.go and this report only |

## What changed
- `internal/plan/store_items_test.go`: tests for Store.Items (current plan; legacy items.md and subagents fold-in without writes; error paths), Store.directory, atomicWrite (non-regular target, missing folder, replace leaves no `.plan-*` temp file), Store.Migrate (current, form B, form A, unparseable, missing) and DigestCoversLines.
- Only helpers `testStore`, `put`, `get`, `fixture` are reused. New helpers: `legacyText`, `snapshot`, `states`.
- No bug found. No row left out.

## NOT VERIFIED on a live screen
Nothing here is UI. The Windows and macOS test runs are compiled only (row 8c). The tests that use `testStore` skip on Windows.

DONE
