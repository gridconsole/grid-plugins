# M-plugin-skipped-3: four skipped plugin lanes landed in one branch

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1a. merge lane-M-agents-a, both sides kept | HOLDS | suite_test.go want list keeps main's five `agent-*` ids and the lane's four (`internal/eval/suite_test.go:18-19`); home_test.go count is 25, derived by running `go test ./internal/eval/` (exit 0) |
| 1b. merge lane-M-jobskills | HOLDS | suite_test.go keeps `plan-quality-*` and adds `job-plan`, `job-run`, `job-review`; catalog.go takes the lane's skill names (`plan-jobs`, `review-job`, `run-job`, which match the folders under `skills/`) and main's hook list, which has no `brief-lint` because main retired it (`internal/settings/catalog.go:29-30`, `retiredHooks` at :33); `go test ./internal/settings ./internal/eval` exit 0 |
| 1c. merge lane-D-brain-wire | HOLDS | command.go: main's `propose --task` plus the lane's `proposeFlags()` (`internal/braincmd/command.go:177-179`); hook.go: main's `history`, `recall` plus the lane's `topicContext` (`internal/hook/hook.go:252-256`), and the lane's `usageHold` call is not carried over because that function no longer exists on main (grep finds no definition), so no hold was reintroduced; topics/README.md keeps main's built-in-topics pointer and the lane's wiring text; `skills/run-job` and `skills/review-job` (not named in the brief, conflicted after 1b) keep main's body and add the lane's `topics:` line; `go test ./internal/braincmd ./internal/hook ./internal/brain ./internal/eval` exit 0 |
| 1d. merge lane-F-review-levels-drop | HOLDS | no make target regenerates the fixture. I recaptured `contract/fixtures/fmt-job-render/valid/full.json` with the test's own writer: `go test ./contract/formats -run TestFormatSavedRenderWriter -update-format-fixtures` exit 0. No `max_rounds` is left in `full.json` or `elided.json`. |
| 2. fetch and merge origin/main | HOLDS | `git fetch origin` then `git merge origin/main` (ebd0a240), no conflicts |
| 3. is-ancestor x5 | HOLDS | `git merge-base --is-ancestor` exit 0 for origin/lane-M-agents-a, origin/lane-M-jobskills, origin/lane-D-brain-wire, origin/lane-F-review-levels-drop and origin/main, each against HEAD |
| 4. `go test -count=1 ./...` | HOLDS | exit 0, every package ok |
| 4. `make lint` | HOLDS | exit 0 |
| 4. `go vet ./...` | HOLDS | exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go vet ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go vet ./...` exit 0 |
| Tests of changed packages and their importers | HOLDS | the whole module passes (`go test ./...`, exit 0), which covers every importer |
| New tests fail on base | n/a | no new tests; merge work only |
| No test dropped | HOLDS | none deleted |
| One existing test edited | see note | `internal/job/lint/json_test.go:52` |
| No guard, gate, cap or hold added | HOLDS | none added; the lane's `usageHold` call was left out because main removed that function |
| Commit rules | HOLDS | one-sentence messages, no attribution lines, no personal paths |

## Note on the edited test

`TestCheckJSONRunsTheBriefRules` was added on main after lane-F-review-levels-drop branched. It sets `max_rounds=0` to trigger a brief-rule finding. lane-F removes the `job.max-rounds` and `job.review` rules on purpose (`internal/job/lint/lint.go`, shown by `git diff origin/main..origin/lane-F-review-levels-drop`; the lane's report is `docs/lanes/F-review-levels-drop.md`), so that mutation can no longer produce a finding. I changed the mutation to an unknown risk (`m["risk"] = ["not-a-risk"]`), so the test still checks that `CheckJSON` runs the brief rules. The edit follows the lane's removal; it was not made to hide a bug. A reviewer can overrule it.

## What changed

- Five merges into lane-M-plugin-skipped-3: the four lanes in the order given, then origin/main.
- Hand-resolved files: `internal/eval/home_test.go`, `internal/eval/suite_test.go`, `internal/settings/catalog.go`, `internal/braincmd/command.go`, `internal/hook/hook.go`, `topics/README.md`, `skills/run-job/SKILL.md`, `skills/review-job/SKILL.md`, and `contract/fixtures/fmt-job-render/valid/full.json` (recaptured).
- One test line changed, in `internal/job/lint/json_test.go`.
- No new behaviour.

## NOT VERIFIED on a live screen

None; there are no UI changes.
