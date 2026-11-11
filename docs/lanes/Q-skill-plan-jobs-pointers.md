# Q-skill-plan-jobs-pointers: plan-jobs and the orchestrator teach described pointers

Base: origin/main 3ccec5cb (the brief names 9eea4651; this clone's main is 3ccec5cb). Branch lane-Q-skill-plan-jobs-pointers, Q-tree-orphan-jobs already merged in (3bcac3a9).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Test fails on main | HOLDS | Commit 84c73023 holds only `internal/job/lint/skill_example_test.go`; on the unchanged skill `go test -count=1 ./internal/job/lint/ -run 'Skill\|Orchestrator'` exit 1: `skill_example_test.go:86: ../../../skills/plan-jobs/SKILL.md has no fenced yaml block` (also in TestSkillExampleTeachesPointers and TestSkillExampleWithShortWhyIsFound), plus `skill must tell the orchestrator to run grid brain search first` and `orchestrator.md does not name never_picked_up / never_acked / no_exit`. |
| 2. Example lints clean | HOLDS | `TestSkillExampleLintsClean`: `lint.Check` on the block, wrapped as below, has no error finding (`lint.HasErrors` false), Resolver from `testFiles(t)` (jobtest home: note `brain:project/export-format`, plus `docs/export.md`, 40 lines, seeded in the repo folder). `go test -count=1 ./internal/job/lint/` exit 0. |
| 3. Example content | HOLDS | `TestSkillExampleTeachesPointers`: a link to `brain:project/export-format` with a why of 8+ characters (SKILL.md link block), a `read` entry `repo:docs/export.md:10-30`, `after: [J-3]`, acceptance A1 and A2, `verify` with command and min_tests 2; the marshalled `intent` is under 400 characters and holds no ``` fence (asserted). |
| 4. Short why is found | HOLDS | `TestSkillExampleWithShortWhyIsFound` cuts the first `why:` to `see`; `job.link-why` error is reported. |
| 5. Commands exist | HOLDS | `TestSkillAndOrchestratorNameOnlyContractCommands` reads `contractdata.Commands` and checks every `` `grid <verb> <sub>` `` in the skill and agents/orchestrator.md; a synthetic `grid job frobnicate` / `grid nosuch run` is asserted to be found. Flags checked in contract/commands.json: `job tree` has `--task`. `TestOrchestratorNamesTheOrphanKinds`: orchestrator.md names `never_picked_up`, `never_acked`, `no_exit` (internal/orchestrator/write.go:130-132) and `grid job tree`. |
| 6. Package tests and vet | HOLDS | `go test -count=1 ./internal/job/lint/` exit 0; `go vet ./internal/job/lint/` exit 0. |
| Importers tested | HOLDS | Importers of internal/job/lint: internal/job/jobcmd; of jobcmd: internal/cli; of cli: cmd/grid. `go test -count=1 ./internal/job/... ./internal/cli/ ./cmd/grid/` exit 0 (job ok, frontmatter ok, jobcmd ok, lint ok, cli ok 22.7s, cmd/grid no test files). |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -l internal/job/lint` empty. |
| Windows / macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No guard/gate/cap/hold | HOLDS | Only skill and agent text and one test; nothing blocks an agent. The orchestrator is told to repost or close an orphaned Job, which it does by posting Jobs as before. |
| No test edited or deleted | HOLDS | `git diff origin/main --stat -- '*_test.go'` lists only the new skill_example_test.go from this lane (plus the merged Q-tree-orphan-jobs test files). |
| No personal paths/hosts/emails | HOLDS | None in the changed files. |
| New tests in a new file | HOLDS | internal/job/lint/skill_example_test.go. |

## What changed

- skills/plan-jobs/SKILL.md: description and field list gain `links`; a Pointers section (brain search first, `links` with `rel` from the lint list and a why that says what the worker gains, `read` by line range, intent states the goal and pastes nothing, keep Jobs small); one full example Job in a fenced `yaml` block.
- agents/orchestrator.md: step 5 reads the orphan lines of `grid job tree --task <task>` each tick and reposts or closes what they name (never_picked_up, never_acked, no_exit); a paragraph on pointers over pasted text and `grid brain search`. The old step 5 (Stop) is now step 6.
- internal/job/lint/skill_example_test.go: five tests.

## Notes

- `lint.Check` takes a full brief (frontmatter, run fields `status` and `follow_up`, nine `## ` headings), not the planned fields alone. The yaml block holds the planned fields only, which is what an orchestrator writes; the test wraps it with `status`, `follow_up` and the headings (`asBrief`) before calling `lint.Check`, as `grid job post` renders a Job.
- `intent` in the contract is three `file#anchor` pointers (lint rejects one without `#`), so the skill says intent is pointers that state the goal, not prose.

## NOT VERIFIED on a live screen

- Nothing in this lane has a screen. Whether an orchestrator run follows the new text is not tested; that needs a live run.
