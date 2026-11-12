# C-qk-browser-proof-task

Base sha 44c7de5bacbac31560de11e84afd4ff15980c05d.

| Row | Result |
|---|---|
| 1 red on base | HOLDS. `go test -count=1 -v -run TestQk ./internal/cli/` on base text exit 1: `grid proof add <item> --text "<what you saw>" --kind check` -> `exit 1: proof: give a task id or folder, or run inside a task folder`. On the lane: `--- PASS: TestQkBrowserProofLinesRun` (both cut lines exit 0; `proof index --task <id> --json` lists 2 proofs on item 1). |
| 2 grep | HOLDS. `git grep -n 'grid proof add <item>' -- skills/browser/SKILL.md` hits :19 and :22, both with `--task <task>`. |
| 3 | HOLDS. `go test -count=1 -v -run 'TestProof\|TestSkillCommandsImplemented' ./internal/cli/ ./internal/packaging/` exit 0, `--- PASS: TestProofAddText`. |
| 4 | HOLDS. `make lint` exit 0; `make cross` exit 0 (darwin/arm64, windows/amd64); `git diff main --stat` names only SKILL.md and the new test file (plus this report). |
| Importers | HOLDS. Root package is imported by internal/job; tests of job/..., packaging, hook, orchestrator, lease, jobgraph, mcp, migrate, ports, lib, cmd all ok. |
| Walk tests (plugin) | HOLDS. `TestHsIpWalksEveryCommand\|TestIlGuestSiteList` ok; `go test ./internal/job/lint/` ok. App walk tests do not apply (plugin repo). |
| RealPlugin | Not applicable, no row names one. |

Changed: SKILL.md:22 command ends `--task <task>`; new test internal/cli/qk_browser_proof_line_test.go. No Go product file. Env line (:11) left out on purpose.

NOT VERIFIED on a live screen: nothing (no UI).
