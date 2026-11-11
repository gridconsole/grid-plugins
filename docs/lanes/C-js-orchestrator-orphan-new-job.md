# C-js-orchestrator-orphan-new-job

Proof table

| Row | Result |
|---|---|
| 1. `go test -count=1 -run TestJsOo ./internal/job/lint/` | HOLDS, exit 0 (`ok ... lint`). On the base (agent edit stashed) it fails: `js_orchestrator_orphan_text_test.go:13: agents/orchestrator.md still holds "repost or close"` and `:25: step 6 lacks "as a new Job"` |
| 2. `go test -count=1 ./internal/job/lint/ ./internal/cli/` | HOLDS, exit 0 (lint ok, cli ok 60s) |
| 2. `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 2. `make cross` | HOLDS: darwin/arm64 and windows/amd64 vet and test compile ran clean, see the completion note below |
| 3. diff names only agents/orchestrator.md, the new test, this file | HOLDS; one line of orchestrator.md changes (line 23) |
| Importers of the changed package (`go list`) | internal/cli and internal/job/jobcmd import internal/job/lint; both tests pass (cli exit 0, jobcmd exit 0, 12s) |
| No guard, gate, cap or hold added | HOLDS: one sentence of agent text and one text test |
| No attribution lines | HOLDS |
| No existing test edited or deleted | HOLDS |
| RealPlugin tests / UI rows | none named in the brief |

## What changed
- agents/orchestrator.md:23: "For an orphan, repost or close the Job the line names." became "For an orphan, cancel the Job the line names, and post its work again as a new Job when the task still needs it."
- New internal/job/lint/js_orchestrator_orphan_text_test.go (TestJsOoOrphanNewJob).

## NOT VERIFIED on a live screen
Nothing here needs a screen.
