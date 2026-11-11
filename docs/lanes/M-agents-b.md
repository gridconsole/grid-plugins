# M-agents-b: bring lane-D-plugin-agents-b up to date with main

Merge of origin/main (343338e6) into lane-D-plugin-agents-b. The merge commit is the head of lane-M-agents-b.

| Row | Status | Evidence |
|---|---|---|
| 1. Merge origin/main, resolve every conflict, list files and what was kept | HOLDS | `git merge origin/main` gave one conflict, `internal/eval/suite_test.go`. Kept every task from both sides in `want` (`internal/eval/suite_test.go:14-22`). The lane's five tasks (agent-checker-read-only, agent-designer-spec, agent-operator-no-browser, agent-library-answers, agent-explore-finds) stay. Main's six (task-skill, setup-skill, signals-skill, skill-brain-keeper, skill-browser, skill-visual-design-review) are added. The list holds 30 ids and `evals/tasks` holds 30 directories. Every other file merged cleanly. |
| 2. `go test ./...` | HOLDS | `go test ./...` exit 0, every package ok, including `internal/eval`. |
| 2. `make lint` | HOLDS | `make lint` (go vet ./...) exit 0. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Packages importing the touched package | HOLDS | The only touched file is a test file in `internal/eval`, which no package imports. `go test ./...` covers all of them anyway. |
| New tests fail on base, pass on lane | N/A | No new test. Only a merge fix to the task list in an existing test. |
| No existing test edited to pass; no deleted test | HOLDS | The suite test list is the union of both sides. Nothing was removed. Main's own deletions (plan-hold guard tests) come from main, not this lane. |
| No guard, gate, cap, hold or blocking hook added | HOLDS | The merge added none. |
| Commit rules | HOLDS | One-sentence message, no attribution lines, no personal paths or hosts. |

## What changed
- `internal/eval/suite_test.go`: union of the task lists.
- Everything else comes from main through the merge (skills, release docs, packaging, settings, guard and plan-hold removal).

## NOT VERIFIED on a live screen
- Nothing UI. The sandbox ran no claude, curl or server. The new eval tasks were not run against a real agent.
