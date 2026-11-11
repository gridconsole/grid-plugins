# C-ma3-checker-agent-file

Base: main 996f9991. `git ls-tree main agents/` held `.gitkeep`, `builder.md`, `curator.md`, `orchestrator.md`, `reviewer.md`; no `checker.md`, so the lane was not stopped.
Code commit: 3dc3efca (the report is the commit after it).

## Proof table

| Row | State | Proof |
|---|---|---|
| 1. `go test ./internal/job/ -run 'Role\|Instructions' -count=1` passes; red on base | HOLDS | Lane: exit 0, `ok  github.com/gridconsole/grid-plugins/internal/job  0.417s`. Base (agents/checker.md stashed, new test list in place): exit 1, `--- FAIL: TestRoleInstructionsSection (0.20s)` / `render_role_test.go:50: open ../../agents/checker.md: no such file or directory` / `FAIL github.com/gridconsole/grid-plugins/internal/job 0.307s` |
| 2. `go test ./... -run 'Agent\|Embed\|Catalog\|ImBash\|KbJa' -count=1` passes | HOLDS | exit 0; last lines `ok  .../internal/workflow  0.024s`, `ok  .../lib  0.016s`; `internal/cli` and `internal/contract` both `ok`. First run was red on `TestKbJaPostSpansRunUnderJobLease/agents/checker.md#1` (`<check-job>` placeholder the walk cannot fill); fixed by writing `<review-job>` as reviewer.md does, then green. |
| 3. `make lint` and `make cross` | HOLDS | `make lint` (go vet) exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, all `ok`). |
| 4. `git show HEAD --stat` lists only Touches files | HOLDS | `git show 3dc3efca --stat`: `agents/checker.md`, `internal/job/render_role_test.go`, `skills/browser/SKILL.md` only. The report `docs/lanes/C-ma3-checker-agent-file.md` is a second commit that the runbook requires; it is not in the code commit. |
| Rule: new tests fail on base, pass on lane | HOLDS | The changed `TestRoleInstructionsSection` (checker added) fails on base, see row 1. `TestRoleWithoutAgentFileGetsNoSection` no longer lists checker; on base that line passes only because checker had no file, so the fail-on-base test is the Instructions one. `TestInstructionsAreNotASourceOrBrainText` also lists checker. |
| Rule: no existing test edited to pass | HOLDS | The only test file edited is `render_role_test.go`, as the brief's Touches names: checker moved from the no-file list to the has-file lists. No test deleted. |
| Rule: no guard, gate, cap, hold or blocking hook | HOLDS | `agents/checker.md` carries reviewer.md's Owner decisions line ("no Grid-made guards, caps or limits") and adds none. |
| Rule: tests of changed packages and importers | HOLDS | `go test -count=1 . ./internal/job/... ./internal/cli ./internal/hook ./internal/jobgraph ./internal/lease ./internal/migrate ./internal/orchestrator ./internal/ports ./internal/mcp ./internal/packaging ./internal/contract ./lib` exit 0, every package `ok`. Importers from `go list -f '{{.ImportPath}} {{join .Imports " "}}' ./...`: cli, hook, jobcmd, lint, jobgraph, lease, migrate, orchestrator (via job); mcp, ports, lib, packaging (via those). |
| Rule: Windows and macOS builds | HOLDS | `make cross`, exit 0 (row 3). |
| Rule: plugin walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 `ok internal/cli 0.083s`; `go test -count=1 ./internal/job/lint/` exit 0 `ok internal/job/lint 0.272s`. |
| Rule: no personal paths, hosts or emails in the public repo | HOLDS | The new files hold none. |
| Rule: *RealPlugin* tests | not run | No row names one. |

## What changed

- `agents/checker.md` (new): frontmatter `name: checker`, `model: sonnet`, `effort: low` (as `internal/settings/catalog.go:20`), tools Read, Grep, Glob, Bash and the two browser MCP sets, skills `browser` and `review-job`. The body checks the built page read-only against the Job's rows, one screenshot per row at the widths the browser skill names, never changes a site, marks a row it cannot check NOT VERIFIED (never a pass), posts `found` records and one `grid job result` whose first line is `ACCEPTED` or `REJECTED` (reviewer.md steps 5 and 6). It keeps reviewer.md's Bash rule and Owner decisions lines. `AgentFor("checker")` is now true, so a checker Job renders `### Instructions`.
- `skills/browser/SKILL.md`: a `Bash:` section with the one-command line and the `Never set `dangerouslyDisableSandbox`` line, worded as `skills/review-job/SKILL.md:18-25`, so `TestImBashEveryJobAgentSkillCarriesTheLines` passes for `browser`.
- `internal/job/render_role_test.go`: `checker` leaves the no-file list in `TestRoleWithoutAgentFileGetsNoSection` and joins the roles in `TestRoleInstructionsSection` and `TestInstructionsAreNotASourceOrBrainText`.

Tests that enumerate `agents/` (no edit needed, they pick the new file up): `internal/contract/im_job_bash_lines_test.go:111`, `internal/cli/kb2_job_ask_block_walk_test.go:81`, `internal/packaging/plugin_files_test.go:199`.

## NOT VERIFIED

- Nothing needing a live screen. A live checker Job run with the built plugin was not done; the render and the walks are proved by tests only.
- Note: `internal/cli/worker.go:206-218` only picks the model and effort for the role (from `catalog.go:20`); the tool set comes from the agent file, which lists Bash because the verdict needs `grid job result`. The user-level `grid-checker` agent listed in the harness has no Bash; whether the app launches the plugin's `checker` or that one is outside this lane's Touches and not checked.
