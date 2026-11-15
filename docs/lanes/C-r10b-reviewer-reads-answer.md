# C-r10b-reviewer-reads-answer

Repo: plugin. Code head: 59ceaaa5. origin/main merged before the last proof run: already up to date, so the merge brought nothing in.

## Proof table

| Row | Result |
|---|---|
| Do 1: MCP feed_read with job and type result returns the same answer record as the CLI | HOLDS. No production fix was needed: internal/mcp/tools.go:193 calls `store.Read(feed.Filter{Since, Type, Job})`, the same filter the CLI uses (internal/cli/feed.go). `records()` (tools.go:260) only unwraps `Entry.Record`. TestR10bMCPReadsPersonAnswerAsResult (internal/mcp/r10b_person_answer_read_test.go) checks that the MCP record and the `store.Read` record match by ID. `go test -count=1 -run TestR10b -v ./internal/mcp/` exit 0 |
| Do 2: agents/reviewer.md (and the orchestrator text that reads a dependency's outcome) says where a person Job's answer is | HOLDS. agents/reviewer.md:15 and agents/orchestrator.md:48 (Deliver step, reads each `after` Job's result) now name the closing `control` record and `body.answer`. builder.md has no `--type result` read, and skills/ has none that reads a person Job's outcome |
| Do 3: test, MCP read of an answered person Job's result returns the answer | HOLDS. TestR10bMCPReadsPersonAnswerAsResult: one record, the closing control, `target` J-1, `answer` "Call it Lark.", and the answer text appears once |
| New test fails on main, passes on lane | HOLDS. See below. On main the MCP read test **passes**, because C-r10 already fixed the shared read and MCP uses it. Its failure is proved by a mismatch edit that switches off `personClosings` (internal/feed/read.go:211 given `""` instead of `f.Job`, then `git checkout HEAD -- internal/feed/read.go`): `r10b_person_answer_read_test.go:47: feed_read --job J-1 --type result wants the one closing record, got 0: {"bad":{},"records":[]}` exit 1. The test that fails on main is TestR10bAgentsReadPersonAnswer (internal/mcp/r10b_person_answer_text_test.go), run with main's agents/*.md checked out: `r10b_person_answer_text_test.go:40: agents/reviewer.md: no line that reads ... body.answer` and `:43: agents/orchestrator.md: ...` exit 1. On the lane: both `--- PASS`, exit 0 |
| `make lint` | HOLDS. `make lint` (go vet ./...) exit 0 |
| Tests of internal/feed, internal/mcp and importers | HOLDS. The importers come from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E "internal/(feed\|mcp)( \|$)"`: contract, internal/approvefix, internal/cli, internal/feed, internal/hook, internal/job/jobcmd, internal/lease, internal/mcp, internal/migrate, internal/orchestrator, internal/question, internal/task, internal/usage, lib. `go test -count=1` on those plus ./internal/job/... and ./internal/contract (they read the agent texts) passed on ccf64d0d. internal/mcp, internal/job/lint and internal/cli were rerun on 59ceaaa5, exit 0. approvefix and job/jobtest have no test files |
| Windows build | HOLDS. `GOOS=windows go build ./...` exit 0 |
| macOS build | HOLDS. `GOOS=darwin go build ./...` exit 0 |
| Walk tests (plugin) | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok, and `go test -count=1 ./internal/job/lint/` ok, exit 0 on 59ceaaa5 |
| fetch and merge origin/main before the last proof run | HOLDS. `git fetch origin && git merge origin/main` was already up to date |
| App guest walk tables (make realplugin) | not run, not asked: this is a plugin lane |
| Whole-module test, cross-repo set | not run, not asked: the orchestrator runs these at landing |
| No guard, gate, cap, hold or blocking hook added | HOLDS. The diff is two agent text sentences and two tests |
| No attribution lines, no personal paths | HOLDS. `git log origin/main..HEAD` |
| Touches | agents/reviewer.md, agents/orchestrator.md, internal/mcp (two new test files). The first commit put the text test in internal/job/lint. The second commit moved it into internal/mcp, so the lane's net diff is outside neither |

## What changed

- agents/reviewer.md:15: for a person Job, `grid feed read ... --type result` returns the closing `control` record, and the answer is `body.answer`, not `body.text`.
- agents/orchestrator.md:48: the same note on the Deliver step's read of each `after` Job.
- internal/mcp/r10b_person_answer_read_test.go: MCP feed_read of an answered person Job's result returns the answer, and the CLI read returns the same record.
- internal/mcp/r10b_person_answer_text_test.go: the reviewer and orchestrator texts name the `control` record and `body.answer`.

## Open items

- orchestrator.md:50 (Re-plan Job) also reads `--type result` for a dead or waiting Job. It was left unchanged: those are worker Jobs, and the brief asks for one or two lines.

## NOT VERIFIED on a live screen

Nothing. There is no UI in this lane.

DONE
