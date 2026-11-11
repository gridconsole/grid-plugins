# C-sk-deliver-no-gate

Base: 0e60a23a855078c08a52b8cabb8e2cb4a1a63b5f (main in this clone; the brief names d0400870).

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1. New test fails on base, passes on lane | HOLDS | `internal/contract/skill_deliver_text_test.go` (`TestSkillDeliverText...`). On base (test added, text unchanged), `go test ./internal/contract -run TestSkillDeliverText -count=1` exit 1: `skills/deliver/SKILL.md still contains "ship gate"`, `... "raises the gate"`, `... "gate: ship"`, `... "an unapproved plan"`, `... "If no curator is available"`, `does not name "step: ship"`, `agents/curator.md contains the word "gate"`. On the lane: 4 tests PASS, exit 0. |
| 2. Each refusal/curator sentence cites code read this session | HOLDS | Plan item lacks proof: `internal/proof/proof.go:332` `plan item %s lacks %s proof; run grid proof add %s`; no items: `proof.go:328` `plan has no items to prove`; both reached by `idx.Check()` at `internal/task/deliver.go:169`. No plan.md: `deliver.go:162-163`. Deliverable file not importable: `deliver.go:172-176` (`CheckImport`). Closed or stopped: `deliver.go:148-149`. Paused or already waiting: `deliver.go:153` (`NextState(t.State, "status", "waiting")`, comment `:151-152`). Waiting proposals refuse nothing: `deliver.go:36-37` and `:156-161`. No plan-approval refusal: the only "approv" in the file is the comment at `:230`. Task ends `state: waiting`, `step: ship`: `deliver.go:226`, `:237`, `:240`. Curator Job posted by deliver: `internal/cli/deliver.go:58-79` (`postCuratorJob`), output `:90-95` "%d brain proposals wait for the curator (Job %s)". |
| 3. `git grep -n -i "gate" -- skills/deliver agents/curator.md` prints nothing | HOLDS | exit 1, no output |
| 4. packaging checks | HOLDS | `go test ./internal/packaging -run 'TestShippedAgentsAndSkillsAreWellFormed\|TestPluginFileChecksBiteOnBrokenFixture' -count=1` exit 0 (`ok`). The new SKILL.md quotes only `grid proof add` and verbs already quoted. |
| 5. contract, packaging, eval tests; lint | HOLDS | `go test ./internal/contract/... ./internal/packaging/... ./internal/eval/... -count=1` exit 0 (contract, packaging, packaging/cmd, eval all `ok`). `make lint` exit 0. |
| Tests of importing packages | HOLDS (none to run) | The only Go change is a `_test.go` file in `internal/contract`; no package can import it, so no importer changes. `go list` of imports shows many importers of `internal/contract` (cli, task, brain, ...); none sees a change. Whole-module test is the orchestrator's. |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| No guard, gate, cap or hold added | HOLDS | Only Markdown text and one test file changed. The skill now says not to start a second curator, which removes a race, and adds no block. |
| New test in its own file, no shared table, no new package-level names | HOLDS | The file declares four `Test` functions and uses `readPluginFile` from `skill_examples_test.go:18`. |
| Only the allowed files touched | HOLDS | `skills/deliver/SKILL.md`, `agents/curator.md` (description only), the new test, this report. |
| Eval checks that grep the old words | HOLDS (none) | `grep -rln -i "ship gate\|gate: ship\|raises the gate" evals` finds nothing. |
| Public repo: no personal paths, hosts or emails | HOLDS | Text edits carry none. |

## What changed

- `skills/deliver/SKILL.md`: description (line 3), the person-step rule (line 10), the curator sentence in step 5 (line 21), the dry-run refusal list (line 22) and the Done line (line 25) now say what `grid deliver` does: no gate, `step: ship`, the curator Job posted by deliver.
- `agents/curator.md`: description only.
- `internal/contract/skill_deliver_text_test.go`: new.

## NOT VERIFIED on a live screen

Nothing here is UI. I did not run `grid deliver` against a real task, so the dry-run wording is tied to the code lines above and not to a live run.
