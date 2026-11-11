# C-go-contract-readme-subverbs-flags

Base sha: cfc06ff41c6d029a43884418471436a6d2579e75. Test is in internal/contract/ (commands.json is embedded in contract/ and read by path from there).

| Row | Status | Proof |
|---|---|---|
| 1 sub-verbs of every backticked `grid <verb> <sub>` are in commands.json or known list; error names README line | HOLDS | `go test -count=1 -v -run TestGoReadmeSubverbsInCommandsJSON ./internal/contract/` exit 0; message format `contract/README.md:<line>: ...` (go_readme_subverbs_flags_test.go, README subtest) |
| 2 every `--flag` in the same backticks is a flag of the verb/sub-verb | HOLDS | same run; srfCheck flag loop; edited-copy subtest shows `--no-such-flag` reported |
| 3 known list exactly 13 with reasons; edited-copy subtest shows unknown sub-verb and unknown flag reported, README not edited | HOLDS | `known list` subtest (count 13, reason set, each entry must match a README span); `edited copy...` subtest PASS; the edit is an in-memory string |
| 4 refusal_message lines not flagged; contract+cli tests, lint, diff | HOLDS | `go test -count=1 ./internal/contract/ ./internal/cli/` exit 0; `make lint` (go vet) exit 0; only backticked `grid ` spans are read, so refusal_message text is not scanned unless backticked as a grid command; diff = new test file plus this report |
| Windows / macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Importers of changed package | HOLDS | test-only change; importers of internal/contract (artifact, brain, braincmd, cli, feed, guard, hook, intent, job, jobgraph, land, lease, mcp, migrate, orchestrator, question, session, settings, signal, task, usage, where, workflow ...) have no non-test change; cli tested above |
| New test fails on base / pasted failing lines | NOT DONE | the test is new and passes on main content by design; it was not run against a drifted README (README is read-only). The edited-copy subtest is the failing-input proof |
| No guards on agents, no existing test edited, no attribution lines | HOLDS | none added |

Note: docs/lanes report is the runbook's required file, in addition to the test file.

## NOT VERIFIED
No live screen involved. No *RealPlugin* test needed. Alternation `grid job post|ack|result|cancel` is checked part by part; `grid feed` on the known list is a bare name that also passes the walk.
