# C-er-descriptions-triggers

Base: main a9fc9c41 (the brief named bf4b8e21; the clone's main is a9fc9c41). All eight files had three sentences on base (line 3 quoted in the brief matched); none was left alone.

## Proof table

| Row | Result |
|---|---|
| 1. TestErDescTwoSentences fails on base | HOLDS. On base, `go test -count=1 -run TestErDesc .` exit 1: `er_descriptions_test.go:69: brain-keeper: 3 sentences, want at most 2: ...` (also plan-jobs, setup, signals, grid-builder, grid-checker, grid-designer, grid-library; `last sentence does not start with "Use "` for brain-keeper, signals, builder, checker, designer). Passes on the lane: exit 0. |
| 2. Kept meaning by phrase, setup has no "first run in a new folder" | HOLDS. Checked inside TestErDescTwoSentences (helper erDescKept, er_descriptions_test.go:73); failed on base for setup (`setup still says "first run in a new folder"`), passes on the lane. |
| 3. `go test -count=1 . ./internal/settings/ ./internal/hook/ ./internal/golden/ ./internal/packaging/` | HOLDS, exit 0: ok on all five. |
| 4. `make lint` | HOLDS, exit 0 (go vet ./...). |
| 4. `git diff main --stat` | HOLDS: the eight files plus er_descriptions_test.go (this report file is added after, as the runbook asks). |
| 4. `git diff main -U0 -- skills agent-templates \| grep '^@@'` | HOLDS: eight hunks, all `@@ -3 +3 @@`. |
| Packages that import the changed ones | `go list` shows only internal/job imports the root package; `go test -count=1 ./internal/job/` exit 0. |
| Windows build | HOLDS: `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS: `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| RealPlugin tests | Not needed: no *RealPlugin* test is part of the rows. |
| No guard, gate, cap; tools lines untouched | HOLDS: only line 3 changed in each file. |
| No test edited or deleted | HOLDS: one new file only. |
| Commit rules | HOLDS: one-sentence message, no attribution lines. |

## What changed
Each extra sentence is folded into the first with a comma, semicolon or colon, and the "Use ..." sentence is last. Setup drops "or on the first run in a new folder". New test file er_descriptions_test.go (helpers erDesc*, test TestErDescTwoSentences).

## NOT VERIFIED on a live screen
How Claude Code picks a skill or agent from the shorter descriptions needs a live session.
