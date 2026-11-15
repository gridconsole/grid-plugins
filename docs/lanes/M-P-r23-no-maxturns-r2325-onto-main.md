# M-P-r23-no-maxturns-r2325-onto-main: lane P-r23-no-maxturns onto plugin main 9630011b

Carries `P-r23-no-maxturns` (its own proof table is `docs/lanes/P-r23-no-maxturns.md`, unchanged).

## Conflicts and how they were resolved

`git merge origin/main` conflicted in two files; both were the `maxTurns` front matter line against a `tools:` line main changed.

| File | Lane side | Main side | Resolved |
|---|---|---|---|
| `agent-templates/grid-checker.md` | drops `maxTurns: 40`, `tools:` without Bash | keeps `maxTurns: 40`, adds Bash to `tools:`, new body text and description | main's `tools:` line (with Bash), no `maxTurns`; main's body and description kept |
| `agent-templates/grid-reviewer.md` | drops `maxTurns: 45`, `tools:` with Edit | keeps `maxTurns: 45`, removes Edit from `tools:`, new `found` posting text | main's `tools:` line (no Edit), no `maxTurns`; main's body kept |

Auto-merged without conflict: grid-builder.md, grid-curator.md, grid-operator.md.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1. origin/main is an ancestor | HOLDS | `git merge-base --is-ancestor origin/main HEAD` exit 0 |
| 1. diff names only lane files | HOLDS | `git diff origin/main --stat`: 9 agent-templates (Explore, builder, checker, curator, designer, library, operator, reviewer, runner), `docs/agents.md`, `docs/lanes/P-r23-no-maxturns.md`, `internal/contract/agent_templates_nocap_test.go` (the P-r23 lane's own files), plus this doc |
| 2. P-r23 row 9 (no maxTurns) | HOLDS | `grep -rn maxTurns agent-templates` finds nothing; `go test -count=1 ./internal/contract -run 'TestTemplatesCarryNoMaxTurns\|TestTemplatesNeedNoRedirectFile\|TestTemplatesNeedNoPipe' -v` exit 0, three PASS |
| 2. P-r23 row 10 (no redirect file, no pipe) | HOLDS | same run: `TestTemplatesNeedNoRedirectFile` and `TestTemplatesNeedNoPipe` PASS. Fails-on-main lines are recorded in `P-r23-no-maxturns.md` rows "Test row 10" and "pipe (round 1)". |
| 2. P-r23 row 3 go test of changed packages and importers | HOLDS | `go test -count=1 ./internal/contract/ ./internal/hook/ ./internal/settings/ ./internal/packaging/... ./internal/braincmd/ ./internal/job/... .` all `ok`, except one flake: `TestOjSoTwoSessionsOwnChanges` failed once ("history skipped: session lock busy") while packages ran in parallel; `go test -count=1 ./internal/hook/` alone exit 0 `ok 4.235s`. `go test -count=1 ./internal/cli` exit 0 `ok 40.435s`. |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v` exit 0, both PASS (39 commands, 39 table entries); `./internal/job/lint` ok 0.181s |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 4. `make cross` | HOLDS | exit 0, darwin/arm64 and windows/amd64 vet and test compile all ok |
| NOT DONE rewrite in carried lane docs | HOLDS, nothing to rewrite | `grep -n "NOT DONE" docs/lanes/P-r23-no-maxturns.md` finds no line |
| Merge origin/main | HOLDS | merged at the start of this lane (commit c6f95789) |
| App guest walk tables | not run, not asked (plugin repo) | |

## What changed
- Merge commit only: conflicts in grid-checker.md and grid-reviewer.md resolved as above. No test, row or error code dropped from either side.

## NOT VERIFIED on a live screen
Whether Claude Code runs the agents without a turn limit; nothing here is a screen.

Open: the routine's `--max-turns` / `--max-budget-usd` flags are still left for the owner, as in `P-r23-no-maxturns.md`.

DONE
