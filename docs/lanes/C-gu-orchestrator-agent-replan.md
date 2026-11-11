# C-gu-orchestrator-agent-replan

| Row / rule | Status |
|---|---|
| 1 TestGuAtReplanParagraph: orchestrator.md holds "Re-plan after" and "`grid job cancel" | HOLDS: `go test ./internal/job/lint/ -count=1` exit 0; paragraph at agents/orchestrator.md:45 |
| 2 NameOnlyContractCommands and NamesTheOrphanKinds pass | HOLDS: same run, exit 0 |
| 3 `go test ./internal/job/lint/ -count=1` | HOLDS: ok, exit 0 |
| New test fails on base | HOLDS: with agents/orchestrator.md stashed: `--- FAIL: TestGuAtReplanParagraph` ... `lacks "Re-plan after"`, `lacks "`grid job cancel"`, `lacks "--reason"` |
| Importers tested | HOLDS: importers of lint = internal/cli, internal/job/jobcmd; `go test ./internal/cli/ ./internal/job/... -count=1` exit 0 |
| Lint | HOLDS: `go vet ./internal/job/lint/` exit 0, `gofmt -l` empty |
| Windows / macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` and `GOOS=darwin ...` exit 0 |
| No guard/gate/cap on agents | HOLDS: text only; names only contract commands |
| Test prefix guAt | HOLDS: no package-level helpers added besides TestGuAtReplanParagraph |
| RealPlugin tests | Not needed by any row |

Changed: one paragraph in agents/orchestrator.md (points to the plan-jobs "Re-plan Job" section); new internal/job/lint/gu_agent_replan_text_test.go.

NOT VERIFIED on a live screen: nothing UI; an agent actually following the paragraph on a real re-plan Job was not run.
