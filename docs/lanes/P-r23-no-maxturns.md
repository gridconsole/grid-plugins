# P-r23-no-maxturns: templates carry no maxTurns cap

Rows 9 and 10 of `U-chk-plugin-rev-skills.md`, checked on plugin main (0f360eaa).

| Row | Result | Evidence |
|---|---|---|
| 1. Row 9 confirmed | HOLDS, confirmed | `grep -rn maxTurns agent-templates` on main: Explore.md:6 25, grid-builder.md:6 60, grid-checker.md:6 40, grid-curator.md:6 40, grid-designer.md:6 50, grid-library.md:6 15, grid-operator.md:6 40, grid-reviewer.md:6 45. Removed from all eight. |
| 1. Row 10 confirmed | HOLDS, confirmed | `grid-builder.md:26` "`git diff -- <path>` into a file made with `mktemp`"; `grid-runner.md:15` "log file made with `mktemp`". Both need a redirect, which `skills/run-job/SKILL.md` forbids. Builder now uses `grid proof add <item> --text "the excerpt"`; runner reads output directly, uses `--text` for proof, and reads CI logs from the tool result. Fix round 1: the pipe wording (`SKILL.md:61` forbids a pipe) is gone from both; builder uses the command's own filter or output-file flag, then Read or Grep. |
| 2. Test row 9 | HOLDS | `TestTemplatesCarryNoMaxTurns` (internal/contract/agent_templates_nocap_test.go). On main: `go test -count=1 -run 'TestTemplatesCarryNoMaxTurns\|TestTemplatesNeedNoRedirectFile' ./internal/contract/` exit 1, `agent_templates_nocap_test.go:29: Explore.md:6 carries "maxTurns: 25"` and seven more. On head: ok. |
| 2. Test row 10 | HOLDS | `TestTemplatesNeedNoRedirectFile`. On main: `agent_templates_nocap_test.go:44: grid-builder.md tells the agent to use mktemp`, same for grid-runner.md. On head: ok. |
| 2. Test row 10, pipe (round 1) | HOLDS | `TestTemplatesNeedNoPipe` (same file). With the old templates restored: `go test -count=1 ./internal/contract -run TestTemplatesNeedNoPipe` exit 1, `agent_templates_nocap_test.go:59: grid-builder.md:31 tells the agent to use a pipe` and `grid-runner.md:21 ...`. On head: ok. |
| 3. go test changed packages and importers | HOLDS | `go test -count=1 ./internal/contract/ ./internal/hook/ ./internal/settings/ ./internal/packaging/... ./internal/cli/ ./internal/braincmd/ ./internal/job/lint/` exit 0, all `ok` (these are the packages that read agent-templates; cli 356s). |
| 3. Round 1 rerun | HOLDS | `go test -count=1 . ./internal/contract ./internal/job/... ./internal/cli ./internal/packaging/...` all ok; `make lint` exit 0; both builds exit 0 |
| 3. make lint | HOLDS | exit 0, `go vet ./...` |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok 0.106s; the full cli run above covers them too. `internal/job/lint` ok. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Merge origin/main | HOLDS | `git fetch origin && git merge origin/main`: Already up to date |
| App guest walk tables | not run, not asked (plugin repo) | |

## What changed
- `agent-templates/*.md`: eight `maxTurns` lines removed; builder and runner no longer use mktemp or a redirect; builder says "if the run ends early" instead of "out of turns".
- `docs/agents.md`: the rule line now says no template carries `maxTurns`.
- New test file `internal/contract/agent_templates_nocap_test.go`.

## Not changed (open)
- Row 9 also names the routine's `--max-turns` and `--max-budget-usd` (`internal/routine/execute.go:186-196`, `internal/routine/limits.go`, `routine-max-turns` flag, contract/commands.json, README). The brief is titled "Templates carry no maxTurns cap", and removing the routine limits is a feature with a policy file, flags and many tests. Left for the owner: remove it or record an exception.
- `internal/brain/check.go:46` still allows `maxTurns` as an agent front matter key; that is a permitted field, not a cap.

## NOT VERIFIED on a live screen
Whether Claude Code runs the agents without a turn limit; nothing here is a screen.
