# C-gq-questions-skill-worker-agents

| Row | Result |
|---|---|
| 1. TestSkillAndOrchestratorNameOnlyContractCommands | HOLDS: `go test -count=1 ./internal/job/lint/ -run TestSkillAndOrchestratorNameOnlyContractCommands`, exit 0 |
| 2. Tests reading skills/questions | HOLDS: `grep -rln "skills/questions" internal/` gives internal/contract/skill_questions_ask_test.go; `go test -count=1 ./internal/contract/ ./internal/job/lint/`, exit 0 |
| 3. Diff touches only step 2 | HOLDS: one sentence added on skills/questions/SKILL.md:18 (step 2); person and lease-holder text kept |
| No guard/gate/cap on agents | HOLDS: skill text only |
| New tests | None added: skill text only; existing tests cover it |
| Packages importing changed ones | None: a markdown file changed, no Go package |
| Lint, Windows and macOS builds | NOT RUN: no Go code changed |
| No attribution in commit | HOLDS |

## What changed
Step 2 of skills/questions/SKILL.md: an agent run by the worker (GRID_ROLE=worker) posts a person Job with `grid job post --person` and stops; it does not run `grid question ask`.

## NOT VERIFIED on a live screen
Nothing UI related. No live worker run was made.
