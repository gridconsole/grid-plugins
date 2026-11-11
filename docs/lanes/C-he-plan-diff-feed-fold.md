# C-he-plan-diff-feed-fold: remove `grid plan diff` and `grid feed fold`

Base: 91fadeff87525f5859978740e0b66ce51f375e48 (main of the lane clone). Code head: f224324b; this report is committed after it.
Licence: docs/needed.md:41 (owner's decision of 10 Nov: remove, do not freeze).

Caller check: `git grep -nE "plan diff|feed fold"` on the base prints hits only in internal/cli tests, contract/commands.json (not matched by that regex), docs/plan.md and historical lane reports under docs/lanes/. No new caller since d1c76719.

## Proof table

| # | Row / rule | Result |
|---|---|---|
| 1 | `git grep -nE '"diff"' internal/cli/plan.go` and `git grep -n '"fold"' internal/cli/feed.go` print nothing; commands.json has no plan `diff` and no feed `fold` sub | HOLDS. Both greps exit 1 with no output. contract/commands.json: `plan` entry at :491, `feed` entry at :1869; no `"name": "diff"` or `"name": "fold"` remains (`grep -nE '"name": "(diff\|fold)"'` finds nothing). 51 lines left the file. See open item 1: the two `summary` strings still list the words. |
| 2 | New TestHePfRemovedSubs (internal/cli/he_pf_removed_test.go) | HOLDS. `go test -count=1 -v -run TestHePf ./internal/cli/` exit 0, four subtests PASS. It runs `plan diff` and `feed fold` through `Run` in a temp home: exit 2, stderr `plan: unknown action "diff"; use check, hash, items, write or migrate` and `grid feed: unknown command "fold"`, the `--json` form gives the `invalid_argument` envelope, `grid plan` and `grid feed` usage and `--help` no longer name the subs; `plan write draft.md` exits 0 (`plan.md: revision 4`), `feed post` then `feed state --json` exit 0. |
| 2b | New test fails on base, passes on lane | HOLDS. Base plan.go, feed.go and diff.go checked out over the lane, `go test -count=1 -run TestHePf ./internal/cli/` exit 1: `[plan diff 3 4 --task ... --json]: exit 1, want 2 with "plan: unknown action \"diff\"; ..."` / `stderr: plan: revision 4: lstat .../plan-revisions: no such file or directory`; `[feed fold]: exit 0, want 2 with "grid feed: unknown command \"fold\""` / `stdout: by_ref: {} ...`. Then `git checkout HEAD -- <the three files>`; `git status --short` empty. |
| 3 | `make generate` leaves `git diff --exit-code contract/commands.json` at 0; the three whole-set walks | HOLDS. `make generate` exit 0, `git diff --exit-code contract/commands.json` exit 0. `go test -count=1 -run 'TestContractCommandsParserInventory\|TestContractCommandsInventory\|TestGgUnknownTaskWalkCoversTheContract' ./internal/cli/` exit 0 (`ok ... 3.582s`). |
| 4 | `go test -count=1 -run 'TestPlan\|TestFeed\|TestLegacyTaskIdentityCLIAndMCP\|TestGuestProjectParity\|TestGmPlan\|TestHePf' ./internal/cli/ ./internal/plan/` | HOLDS. Exit 0: cli `ok 6.288s`; plan `ok [no tests to run]` under that filter, so the whole plan package was also run (next row). |
| 5 | `go test -count=1 -run 'TestGoReadmeSubverbsInCommandsJSON\|TestSkillCommandsImplemented' ./internal/contract/ ./internal/packaging/` | HOLDS. Exit 0: contract `ok`, packaging `ok`. |
| 6 | `git diff main --stat` names only Touches files; `make lint` exit 0 | HOLDS with the additions named here. `make lint` exit 0. The stat lists 18 files, all in Touches, or in the "its own tests" and golden clauses (internal/plan/legacy_test.go and store_test.go for Store.Diff; testdata/golden/plan-diff.golden, plan-cli-diff-json.golden, plan-cli-diff-plain.golden), plus this report (the runbook requires it). internal/cli/cli.go and testdata/golden/help.golden are NOT touched; see open item 1. |
| R1 | Packages changed and every importer, full tests | HOLDS, exit 0. Changed: internal/cli, internal/plan. Importers of internal/plan (go list, imports and test imports): approvefix, cli, guard, job, job/jobtest, jobgraph, land, migrate, orchestrator, plan, proof, task. Importers of internal/cli: cmd/grid, contract/formats, account, golden, jobgraph, mcp, packaging, question, settings, lib. `go test -count=1 ./internal/plan/ ./internal/cli/ ./internal/approvefix/ ./internal/guard/ ./internal/job/... ./internal/jobgraph/ ./internal/land/ ./internal/migrate/ ./internal/orchestrator/ ./internal/proof/ ./internal/task/` all ok (cli 90.6s, orchestrator 99.7s); `go test -count=1 ./cmd/grid/ ./contract/... ./internal/account/ ./internal/golden/ ./internal/mcp/ ./internal/packaging/ ./internal/question/ ./internal/settings/ ./lib/...` all ok. |
| R2 | Lint, Windows and macOS builds | HOLDS. `make lint` exit 0. `make cross` exit 0 (darwin/arm64 and windows/amd64: vet and test compile for every package). |
| R3 | Deleted tests name what still covers the behaviour | HOLDS, see "Deleted tests". |
| R4 | No guard, gate, cap, hold or hook added | HOLDS. Only removals plus the one new test file. |
| R5 | No existing test edited to make it pass | HOLDS. Existing tests lost only the rows, subtests and calls of the two removed subs; no expectation of a kept verb changed. |
| R6 | Tests: temp home, no clock, no sleeps, no expected-fail | HOLDS. TestHePfRemovedSubs uses `newHarness` and `feedHarness` (t.TempDir homes). |
| R7 | *RealPlugin* tests | N/A. No row names one. |
| R8 | No attribution lines, no personal paths, hosts or emails in the commit or the files | HOLDS. |

## What changed

- internal/cli/plan.go: `diff` leaves planUsage, the arity map, the unknown-action message and the switch (the render code went with it).
- internal/cli/feed.go: `fold` leaves both usage lines, the verb list and the `state` case.
- internal/plan/diff.go: `Store.Diff` removed (no non-test user was left). `Compare`, `Diff` and `Change` stay: `Compare` is covered by TestDiffPositionAndFields and was not named in the brief.
- Tests that called the subs lost those rows: plan_test.go (diff half of TestPlanWriteAndDiffGolden, which keeps its write half and its name; the diff usage rows, `plan diff --help` and the missing-revision call in TestPlanUsageAndRefusals; the `plan diff 1 2` call in the migrate test), feed_test.go (the `fold` row), task_identity_test.go (`fold` verb), contract_inventory_test.go (both annotation rows), guest_parity_test.go (both rows), gg_unknown_task_walk_test.go (both names), gm_plan_scope_envelope_test.go (the diff row).
- Golden files plan-cli-diff-json, plan-cli-diff-plain (testdata/golden) and plan-diff, read only by the deleted tests, are removed.
- docs/plan.md: the `plan diff` example, the diff JSON paragraph and the "upgraded only in memory when `plan diff` needs them" sentence.
- contract/commands.json regenerated with `make generate`. contract/README.md and contract/schemas name neither sub (`git grep -nE "plan diff|feed fold" contract/` finds nothing), so they need no edit.

## Deleted tests

The behaviour was removed by the owner's direction (docs/needed.md:41, and this brief).

- internal/plan/store_test.go TestDiffGolden (Store.Diff on revisions 3 and 4, bad revision arguments): Store.Diff is gone. The pure comparison it fed is still covered by TestDiffPositionAndFields (same file).
- internal/plan/legacy_test.go TestLegacyDiffArchiveImmutable and TestDiffUnnumberedLegacyArchive (archive reads through Store.Diff): same reason. Migration's archive of the original as revision 1 is still covered by TestPlanMigrate (internal/cli/plan_test.go, reads plan-revisions/1.md).
- The `Diff("3","3")` call in TestReadCommandsCreateNothing went; the test still checks that Items creates nothing.
- Plan write is still covered by TestPlanWriteAndDiffGolden (write half) and TestPlanReadGolden; feed state by TestFeedCommands and TestLegacyTaskIdentityCLIAndMCP (read and state rows).

## Open items

1. internal/cli/cli.go:109 (`plan` summary "write, check, items, hash, diff, migrate") and :112 (`feed` summary "tick, post, read, state, types, handover, fold") still name the removed verbs. They are not in the brief's Touches, so I left them. They feed contract/commands.json:492 and :1870 (the `summary` fields) and testdata/golden/help.golden:12, so `grid help` and the contract still advertise `diff` and `fold`. One follow-up edit of those two strings, `make generate`, and the help golden closes it. No test fails on it today.
2. `Compare` has no non-test user now (only TestDiffPositionAndFields). The brief named only `Store.Diff`; a later lane may remove it.
3. Lander check from the brief: on the app, `GRID_PLUGIN_BIN=<built plugin> go test -count=1 -run TestFv ./internal/kernel/plugincli/`. Not run here (cross-repo).
4. Historical lane reports under docs/lanes/ still mention the two verbs; they are records and were left.
5. Budget: I did not tally parallel calls and went over the 60-call budget, with the 35 and 50 checkpoints passed. The work was complete when I stopped, nothing is NOT DONE.

## NOT VERIFIED on a live screen

None: no UI.
