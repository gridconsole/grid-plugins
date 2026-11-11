# C-he-job-order-person-list: remove `grid job order` and `grid person list`

Repo: plugin. Branch lane-C-he-job-order-person-list, stacked on lane-C-he-plan-diff-feed-fold. Base sha 8ce60396 (the plan-diff lane head); `git merge-base --is-ancestor f224324b HEAD` exit 0, so the plan-diff and feed-fold removal is on it. Licence: docs/needed.md:41 (owner's decision of 10 Nov: remove, do not freeze).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. `git grep -n '"order"' internal/cli/orchestrator.go` prints nothing | NOT DONE as worded, and the `order` sub is gone | The grep prints two lines: `orchestrator.go:54: orderFlag := fs.Int("order", -1, "queue order of the Job (lowest first); on job post only")` and `orchestrator.go:114: obj["order"] = *orderFlag`. Both are the `--order <n>` flag of `grid job post`, which the brief says stays (the app calls it). The "order" case-list entry, the `orderArg` parse and the order branch are removed (`git diff 8ce60396 -- internal/cli/orchestrator.go`). I did not hide the two lines behind a constant to satisfy the grep. |
| 1. `git grep -n '"list"' internal/cli/person.go` prints nothing | HOLDS | exit 1, no output |
| 1. commands.json has no `order` sub under job, no `list` sub under person | HOLDS | `make generate` removed `"name": "order"` (job, 116 lines) and `"name": "list"` (person, 18 lines) and nothing else (134 deletions, `git diff 8ce60396 --stat`); the `--order` flag stays in `job post`'s flag list |
| 2. TestHeJpRemovedSubs: unknown-sub exit for both, order control record then `job next`, `person show` exit 0 | HOLDS | `go test -count=1 -v -run 'TestHeJp' ./internal/cli/`: PASS, four subtests PASS (job order exit 2 with `job: unknown job command "order"; use render, lint or show`, plain and `--json`; person list exit 2 with `person: unknown command "list"`; J-2 and J-3 posted with `--order 5` and `--order 6`, then `job post J-3 --type control --body {"target":"J-3","order":0}`, the stored order of J-3 is 0, and `job next --json` exit 0 assigns the Jobs in the order J-3, J-2, J-1 (read from the assign control records in the Feed by `heJpAssigned`); `person show gus` exit 0). internal/cli/he_jp_removed_test.go:30. Fix round 1: the earlier check was `strings.Contains(out, "id":"J-3")`, which holds for any order because `job next` lists a row for every Job by id (internal/orchestrator/tick.go:150-173). The check now compares the assign sequence. Proof it can fail: with the control record body and the stored-order check changed to order 7 (J-3 behind J-2 at 5), `go test -count=1 -run TestHeJpRemovedSubs ./internal/cli/` exit 1: `he_jp_removed_test.go:111: job next assigned [J-2 J-3 J-1], want [J-3 J-2 J-1] (lowest order first)`. File restored with `git checkout -- internal/cli/he_jp_removed_test.go`, `git status --short` empty, then the test passes (exit 0). |
| 2. New test fails on base | HOLDS | With the base orchestrator.go and person.go checked out (`git checkout 8ce60396 -- ...`), `go test -count=1 -run TestHeJp ./internal/cli/`, exit 1: `[job order J-1 3]: exit 0, want 2 with "job: unknown job command \"order\"; use render, lint or show" / stdout: 01M754BJ5TGP255K5DCSX2YM98` and `[person list]: exit 0, want 2 with "person: unknown command \"list\""`. Files restored with `git checkout HEAD -- ...`; `git status --short` empty. |
| 3. `make generate` leaves `git diff --exit-code contract/commands.json` at 0 | HOLDS | `make generate` run twice on the head, then `git diff --exit-code contract/commands.json`: exit 0 |
| 3. `go test -count=1 -run 'TestContractCommandsParserInventory\|TestContractCommandsInventory\|TestGgUnknownTaskWalkCoversTheContract' ./internal/cli/` | HOLDS | `ok ... internal/cli 6.293s`, exit 0 |
| 4. `go test -count=1 -run 'TestPerson\|TestGmJob\|TestGuestProjectParity\|TestHeJp' ./internal/cli/` | HOLDS | `ok ... internal/cli 7.655s`, exit 0 (run after the TestPersonGoldenAndReadOnly fix below) |
| 4. `go test -count=1 ./internal/orchestrator/` | HOLDS | `ok ... internal/orchestrator 83.296s`, exit 0 |
| 5. `go test -count=1 -run 'TestGoReadmeSubverbsInCommandsJSON\|TestSkillCommandsImplemented' ./internal/contract/ ./internal/packaging/` | HOLDS | `ok internal/contract 0.083s`, `ok internal/packaging 0.022s`, exit 0 |
| 5. `git diff lane-C-he-plan-diff-feed-fold --stat` names only Touches | HOLDS with one addition | The lane branch name is not a local ref, so I diffed the same commit by sha: `git diff 8ce60396 --stat` lists contract/commands.json, orchestrator.go, person.go, person_test.go, contract_inventory_test.go, guest_parity_test.go, gg_unknown_task_walk_test.go, gm_job_scope_envelope_test.go, person-help.golden, the new he_jp_removed_test.go, and `testdata/golden/person-list-json.golden` (deleted: the golden of the removed `person list`, the contract paragraph says remove goldens that name it) |
| 5. `make lint` | HOLDS | `go vet ./...`, exit 0 |
| Tests of changed packages and of every importer | HOLDS | `go list` importers of internal/cli: cmd/grid, lib. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/ ./internal/contract/ ./internal/packaging/`: ok cli 70.220s, cmd/grid no test files, ok lib 9.926s, ok contract 1.426s, ok packaging 11.549s, exit 0. internal/orchestrator (changed in no file, covered by row 4) ok. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build -o /dev/null ./cmd/grid` exit 0; `GOOS=darwin CGO_ENABLED=0 go build -o /dev/null ./cmd/grid` exit 0 |
| No existing test edited to pass; deleted tests named | HOLDS with disclosure | No test function deleted. TestPersonEmptyCacheAndOwnerMatch (person_test.go:166) now calls `person show nobody --json` (exit 1, empty stdout, handle in stderr) as the brief directs; its read-only snapshot check is unchanged. Rows of the removed verbs left the shared tables (contract_inventory_test.go, guest_parity_test.go, gg_unknown_task_walk_test.go, gm_job_scope_envelope_test.go). TestPersonGoldenAndReadOnly (not named in the brief) also called `person list`: its `person-list-json` golden case and the `list`, `list extra`, `list --task`, `list --unknown`, `list --project unknown` rows were removed with the verb; `show`, `approvers` and the help rows are unchanged. Job order by record stays covered by TestNextOffersLowestOrderFirst and TestOrderChangeByRecord (internal/orchestrator/order_test.go:38, :68, passing in row 4). |
| No guard, gate, cap, hold or blocking hook added | HOLDS | Only removals plus one test file; the removed order branch held a not-started check (a guard on the CLI sub), and it left with the sub. The control record is unchanged. |
| Small commits, one-sentence messages, no attribution | HOLDS | `git log 8ce60396..HEAD` |
| No personal paths, hosts or emails in the diff | HOLDS | The test file uses `gus`, `Gus Example` and temp homes only |
| Lander check on the app (TestFv, fv_argv_realplugin_test.go) | NOT DONE | Not an acceptance row; the orchestrator runs it after merge. No *RealPlugin* test was needed in this lane. |

## What changed

- `internal/cli/orchestrator.go`: `order` leaves the `grid job` case list and the verb parse; the order branch (not-started check, number parse, control record build) is gone. `grid job order` now falls to `runJob` and answers `job: unknown job command "order"; use render, lint or show`, exit 2. The `--order` flag of `job post` and the `{"target":job,"order":n}` control record through `job post --type control` stay.
- `internal/cli/person.go`: `list` leaves the usage text, the verb switch and the output branches; `person list` answers `person: unknown command "list"`, exit 2. `show`, `approvers` and `check` are unchanged.
- `contract/commands.json` regenerated by `make generate`; `testdata/golden/person-help.golden` loses the `list` line; `person-list-json.golden` deleted.
- Tests: rows of the two removed subs left the shared tables, TestPersonEmptyCacheAndOwnerMatch moved to `person show`, and `internal/cli/he_jp_removed_test.go` adds TestHeJpRemovedSubs.
- contract/README.md and contract/schema/ hold no mention of either sub (`git grep -n -E 'job order|person list'` outside docs/lanes), so they needed no edit.

## Open items

- Row 1 wording: `"order"` still appears in orchestrator.go:54 and :114 because the `job post --order` flag stays by the brief. The grep cannot print nothing while that flag exists; the owner or reviewer may reword the row to `case "order"` and `verb == "order"`, which no longer match.
- The `person` summary still reads "check person access; list, show and find approvers" (internal/cli/cli.go:123, testdata/golden/help.golden:26, contract/commands.json:3193). Neither file is in Touches, so I left it; a follow-up lane can change it to "show and find approvers".
- Historical lane reports still name the subs (docs/lanes/C-gg-cli-unknown-task-walk.md, C-gm-job-unknown-scope-envelope.md, C-need-cli-verb-map.md, L54.md, S3.md). They are records of past lanes and I did not rewrite them.
- Tool-call budget: I used about 60 calls (a missed `person list` in TestPersonGoldenAndReadOnly cost an extra test round).

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. NOT VERIFIED: the app's argv through the built plugin (TestFv, run by the lander).

DONE
