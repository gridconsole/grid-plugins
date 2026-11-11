# E-r3-agents-one-command

Base check: `grep -n 'one command per Bash' agents/*` printed nothing on base; agents/orchestrator.md:13 held both commands. Both as the brief said.

## Proof table

| Row | State | Evidence |
|---|---|---|
| 1. TestR3ocEveryBashAgentCarriesTheLine | HOLDS | `go test -count=1 -run R3oc ./internal/job/` exit 0 on the lane. On base (`git stash push -- agents/`) it FAILs: `r3oc_one_command_test.go:65: agents/orchestrator.md: Instructions hold no one-command Bash line; closest line lacks ["one command per Bash call" "`;`" "`&&`" "pipe" "`$("]` (same for builder, curator, reviewer), exit 1. |
| 2. TestR3ocOrchestratorReadsSplit | HOLDS | Base FAIL: `r3oc_one_command_test.go:80: agents/orchestrator.md:13 names both `grid task show` and `grid plan items``. Lane passes. Step 1 is now sub-steps 1 and 2, one Bash call each (agents/orchestrator.md:15-17). |
| 3. `go test -count=1 ./internal/job/ ./internal/contract/` | HOLDS | both `ok`, exit 0; no golden changed. |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`). |
| 4. `make eval-check` | HOLDS | exit 0, 3 evals passed. |
| 4. diff only Touches | HOLDS | agents/{orchestrator,builder,reviewer,curator}.md, internal/job/r3oc_one_command_test.go, this report. |
| Tests of changed packages and importers | HOLDS | `go test -count=1 . ./internal/hook/ ./internal/job/... ./internal/jobgraph/ ./internal/lease/ ./internal/orchestrator/ ./internal/migrate/ ./internal/packaging/...` all ok, exit 0. Importers of internal/job from `go list`: cli, hook, jobcmd, lint, jobgraph, lease, migrate, orchestrator; internal/cli (whole-module) is left to the orchestrator at landing. |
| Windows / macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| New tests fail on base, pass on lane | HOLDS | see rows 1, 2. |
| No existing test edited or deleted; no guard/gate/cap | HOLDS | only new file added; text-only agent edits. |
| No attribution lines, no personal paths | HOLDS | commit message has none. |
| *RealPlugin* tests | n/a | no row names one. |

## What changed

- Each of the four agents with Bash in `tools:` gained a "Bash:" paragraph. The line is the plan-jobs SKILL.md:28 sentence plus the `VAR=value` line (SKILL.md:30), joined into one paragraph and one line so a single line holds every pinned word. It is reworded slightly rather than copied word for word, because the skill splits them over two bullets and the test needs all pinned words on one line.
- orchestrator step 1 split into two sub-steps.
- New internal/job/r3oc_one_command_test.go.

## NOT VERIFIED on a live screen

Whether a real orchestrator run now sends two separate Bash calls (needs a live claude run; none was made).
