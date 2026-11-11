# C-nc2-task-summary-verb: lane report

Base: main 996f9991. `grid task summary` did not exist there: `go run ./cmd/grid task summary T-01 --for owner --text x` exits 2 with `grid task: unknown command "summary"`. This lane adds the verb `grid task summary <task> --for owner|developer|customer --text <words> [--json]`, which writes `summary.md` in the task folder for the app's task page to read.

## Proof table

| Row / rule | Result |
|---|---|
| 1. Red on base | HOLDS. With internal/task/summary.go removed and write.go, task.go and inprocess.go put back to 996f9991, the new tests fail: the task package does not build (`s.Summary undefined`, `undefined: SummaryOptions`), and the CLI tests fail with `exit 2: grid task: unknown command "summary"`. The lane files were then restored from HEAD; `git status --short` was clean of those four. |
| 1. PASS lines on the lane | HOLDS. `go test -count=1 -v ./internal/task ./internal/cli -run TestNcSummary` exit 0: PASS TestNcSummaryShapeAndReceipts, TestNcSummaryKeepsEveryOtherByte, TestNcSummaryJournalReplay, TestNcSummaryRefusals, TestNcSummaryRolePolicy, TestNcSummaryClosedTaskAndState (internal/task/nc_summary_test.go); PASS TestNcSummaryWritesAndAnswers, TestNcSummaryRefusals, TestNcSummaryGuestRefused, TestNcSummaryInProcess, TestNcSummaryUsageLine (internal/cli/nc_summary_cli_test.go). |
| 2. Shape, keep, receipts, replay | HOLDS. TestNcSummaryShapeAndReceipts: a fresh file is exactly `## Owner\n\n## Developer\n\n## Customer\n`; a write fills one body with one blank line between sections and one trailing newline; one history receipt per write with action `summary` and note `summary · <reader>`; the folder listing is stable (the first write only adds history.md and .task.lock for a task that was never written). TestNcSummaryKeepsEveryOtherByte (6 subtests): CRLF and an extra line before `## Owner` are kept byte for byte, other sections are untouched. TestNcSummaryJournalReplay: a journal with history.md + summary.md replays ok; any other name is still `invalid journal file` (internal/task/write.go:594). |
| 3. Refusals and guest | HOLDS. TestNcSummaryRefusals (internal/cli, run with summary.md absent and again after a real write): `--for boss` and no `--for` exit 2 (`--for must be owner, developer or customer`), blank and missing text exit 2 (`a summary needs words`), a text line starting `## ` exits 2 (`would open a new section`), a control character exits 2 (`forbidden control`, the task text check), an unknown flag and a missing task exit 2, and `T-99` exits 1 with `grid task summary: scope: task T-99 not found`; summary.md is absent or byte-equal after every case. The task-package TestNcSummaryRefusals checks the same refusals against `Store.Summary` (`ErrEditUsage`, no write). TestNcSummaryGuestRefused: an invited guest (project-roles.json person `gus`, GRID_ROLE `guest`, GRID_MEMBER `gus`) exits 1 with stderr exactly `grid task summary: scope: role cannot write task summaries`, summary.md is absent, and the role `member` then writes with exit 0. |
| 4. `go test -count=1 -run TestContractCommandsParserInventory ./internal/cli` | HOLDS, exit 0. |
| 4. `go test -count=1 ./internal/cli/ ./internal/task/ ./internal/contract/` | HOLDS, exit 0: cli ok 50.9s, task ok, contract ok. |
| 5. hsIpAnswer for the verb | HOLDS. internal/cli/inprocess.go:58 `"summary": {}`; TestNcSummaryInProcess requires `hsIpAnswer(hsIpTable, task summary T-01 --for owner --text x)` to answer true. |
| 5. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0. |
| 5. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0. |
| 6. `git grep -n 'grid task summary'` | HOLDS. One line each: agent-templates/grid-planner.md:30 and contract/formats.md:225 (the sentence wraps to line 226, one line holds the string). |
| 6. `make lint` | HOLDS, exit 0 (`go vet ./...`). |
| 6. `make cross` | HOLDS, exit 0: darwin/arm64 and windows/amd64 vet and test compile. |
| Importers of internal/task | HOLDS. `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E 'grid-plugins/internal/task( \|$)'` lists braincmd, cli, guard, hook, job, job/jobcmd, job/jobtest, land, migrate, orchestrator, signal, updates, workflow. `go test -count=1` on all of them, plus job/frontmatter and job/lint, exit 0, all ok (orchestrator 77s). |
| Role rule | HOLDS. internal/task/summary.go:13 `var SummaryAllowed = defaultTaskEditAllowed`; summary.go:58-59 refuses with `scope: role cannot write task summaries`, exit 1. TestNcSummaryRolePolicy checks `SummaryAllowed` for "", owner, member (allowed), guest and worker (refused), the refusal text through `Store.Summary` with role guest, and that a replaced `SummaryAllowed` is honoured. |
| Refusals before any write | HOLDS. summary.go:40, :44, :46, :51 return `ErrEditUsage` (exit 2) before `withWrite` runs (summary.go:62). |
| Same lock and journal as `Note` | HOLDS. summary.go:62-71 uses `withWrite` and `commitWrite` with `{summaryFile: ...}`; internal/task/write.go:594 allows `summary.md` in a replayed journal. |
| New tests fail on base, pass on the lane | HOLDS, see row 1. |
| No existing test edited to pass, none deleted | HOLDS. Edits to existing tests are added rows only (listed below); no line removed or weakened. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS. The only refusal is the role rule the brief names. The planner template line asks for the command; nothing blocks an agent that skips it. |
| `make generate` | HOLDS. Ran it; it changed contract/commands.json only (the `task summary` entry and the task summary line). `UPDATE_CONTRACT_COMMANDS=1` was not used. |
| No sudo, no claude, no curl, no server, no push, nothing outside the lane clone | HOLDS. |
| Commits | One sentence each, no attribution lines, no personal paths, hosts or emails. |

## What changed

- `internal/task/summary.go` (new): `SummaryAllowed`, `SummaryOptions`, `Store.Summary`, and the section editor `setSummary`. It trims and lowercases the reader, refuses an unknown reader, empty text, a `## ` line and bad task text (`ErrEditUsage`), then the role rule, then writes through `withWrite` and `commitWrite`. The editor replaces the body of the last matching heading, keeps every other byte, takes the line ending from the heading's own line (else CRLF if the file has one, else LF), and appends a missing heading at the end.
- `internal/task/write.go:594`: `summary.md` joins the names a journal replay accepts.
- `internal/cli/task.go`: the `summary` subverb, its usage line, and `runTaskSummary` (exit 2 usage, 3 rejected, 1 otherwise). `internal/cli/inprocess.go:58`: the in-process row.
- `contract/formats.md`: a section "Task summary (summary.md)" with the shape, the rules, the receipt and the role rule. `agent-templates/grid-planner.md:30`: the planner runs the verb once per reader at each step change.
- `contract/commands.json`: regenerated by `make generate`.
- Tests (new): `internal/task/nc_summary_test.go`, `internal/cli/nc_summary_cli_test.go`.

## Deviation from the brief's Touches

The brief's Touches did not list these files. Walk tests that read the command table or commands.json fail without them (each `t.Fatalf`s on a verb it has no row for), so a new verb cannot land without them. Each edit is one row or one string, as earlier lanes did:

- `internal/cli/cli.go`: the `task` summary line now ends `...decline|delete|rename|restore|summary` (TestExHelpSummariesNameEveryVerb: "task leaves out verbs [summary]"). Joined with `|`, the line is 118 columns; with `, ` it was 128 and TestExHelpLinesFitOneLine requires under 120. `testdata/golden/help.golden` carries the same line.
- `testdata/golden/task-help.golden`: the new usage line (TestTaskRenameDeclineHelp compares it with taskUsage).
- `internal/cli/contract_inventory_test.go`: `task summary` with operand `<task>` and required `--for`, `--text`.
- `internal/cli/guest_parity_test.go`: `task summary` parity row.
- `internal/cli/gg_unknown_task_walk_test.go`: `task summary` in ggTaskWalked.
- `internal/cli/fv_guest_task_test.go`: the `summary` guest row (exit 1, scope text).
- `internal/cli/jv_agent_author_test.go`: `task.go:runTaskSummary` in wantCalls.

If the reviewer holds Touches as strict, this is the one place the lane would be REJECTED; the brief cannot fit without these rows.

## Decisions to check in review

- A reader word is matched case-insensitively and trimmed; the heading written is always `## Owner`, `## Developer` or `## Customer`.
- With a duplicate heading the write goes to the last one, as the app's parser lets the last duplicate win.
- A file with CRLF gets CRLF in the new body (taken from the heading line, else from the file).

## NOT VERIFIED on a live screen

The "Check before accept" screenshot pair (app route /tasks/<id>, the owner, developer and customer views reading summary.md) is not taken here; nothing in this repo runs the app. The UI track shoots it after landing. The editor follows the app parser as the brief describes it (a line `## <Heading>` opens a section, heading lowercased and trimmed, CRLF folded, last duplicate wins); the app's own parser was not run against files this verb writes, and a heading with leading spaces before `##` is not refused in text because the brief's rule is a line that starts with `## `.

DONE
