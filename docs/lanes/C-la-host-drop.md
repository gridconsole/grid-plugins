# C-la-host-drop: drop the Grid host file-tool guard

Lane branch `lane-C-la-host-drop` on plugin main. Base check (step 1): `git merge main` printed "Already up to date"; `git grep -n -w -E 'EvaluateHost|HostGuard' -- '*.go'` listed only files in Touches; `git grep -n 'host-guard'` over the five re-pinned test files printed nothing.

## Proof table

| Row | Verdict | Command or place, exit code |
|---|---|---|
| 1 symbols gone | HOLDS | `git grep -n -w -E 'EvaluateHost\|evaluateHost\|HostGuard' -- '*.go'` prints nothing (exit 1, no match). |
| 1 host.go gone | HOLDS | `ls golden/host-guard internal/guard/host.go` exits 2: "No such file or directory" for both. |
| 1 golden gone | HOLDS | `git grep -n 'host-guard' -- internal/golden golden/INDEX.md golden/known-differences.txt` prints nothing. |
| 2 TestLaHostDropHookNoLongerRefuses | HOLDS | internal/hook/la_host_drop_test.go; `go test -count=1 -v -run TestLaHostDrop ./internal/hook/` exits 0, `--- PASS: TestLaHostDropHookNoLongerRefuses (0.04s)`, nine sub-tests (levels block, warn, off; `.host/x`, `.host/approval/x.json`, `.HOST/x`). Drives `Run(Options{Event: "pretooluse", ...})` with `settings.Store{Home: grid}.SetHook("host-guard", level)` and a Write with no agent type. |
| 2 fails on base | HOLDS | With base `internal/guard/host.go`, `internal/guard/context.go`, `internal/hook/hook.go` restored (`git checkout main -- ...`), the same test fails: `--- FAIL: TestLaHostDropHookNoLongerRefuses/block/.host/x`: `la_host_drop_test.go:58: host-guard block: the hook refused .host/x: {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"<grid>/.host/x is in Grid's host folder, which only Grid writes"}}`; the `.host/approval/x.json` and `.HOST/x` block rows fail the same way; `FAIL github.com/gridconsole/grid-plugins/internal/hook`, exit 1. The base files were then put back (`git rm -f internal/guard/host.go`, `git checkout HEAD -- internal/guard/context.go internal/hook/hook.go`); `git status --short` printed nothing. |
| 3 five packages | HOLDS | `go test -count=1 -v ./internal/guard/ ./internal/hook/ ./internal/cli/ ./internal/settings/ ./internal/golden/` printed `ok` for all five and no FAIL. Named lines: `--- PASS: TestLaHookFeedCarriedBySetupRule (0.00s)`, `--- PASS: TestLaHostDropHookNoLongerRefuses (0.06s)`, `--- PASS: TestKsHostRulesGateFilesBelowHome (0.01s)`, `--- PASS: TestSetupWritesDenyRulesForTheHostGuardFiles (0.01s)`. |
| 3 importers | HOLDS | `go list` importers of guard, hook, settings, cli, golden: cmd/grid, contract/formats, internal/{account, brain, braincmd, doctor, gridhome, job, job/jobcmd, jobgraph, mcp, migrate, orchestrator, packaging, plan, project, question, task, updates}, lib. `go test -count=1` on those plus internal/job/lint printed `ok` for every package that has tests (cmd/grid and internal/updates have no test files). |
| 4 walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok (exit 0); `go test -count=1 ./internal/job/lint/` ok (exit 0, 0.229s). `git merge main` was a no-op, so the base is main. The App walk tests (`make webui-test`, plugincli, grid-server, slices) belong to the app repo; none runs here. |
| 5 lint | HOLDS | `make lint` ran `go vet ./...`, exit 0. `gofmt -l internal/` printed nothing after `gofmt -w` on setup_claude_host_test.go. |
| 5 cross | HOLDS | `make cross` exit 0: darwin/arm64 and windows/amd64, vet and test compile, every package ok. |
| New test fails on base, passes on lane | HOLDS | See row 2. |
| No attribution lines | HOLDS | The commit message is one sentence and has no Co-Authored-By; the owner's rule won over the reminder. |
| No guard, gate, cap or hold added | HOLDS | Only removals; the new test file is the sole addition and adds no rule. `claude.go` has a comment change only, no rule change. |
| Touches only | HOLDS | `git status` lists only files in Touches plus this report and the golden dirs/lines the brief names. |
| Budget | NOT MET | About 55 calls were asked; this run used about 65. The extra calls went to the base proof (restore and re-restore of three files) and to the importer and cross runs. |

## What changed

- Deleted `internal/guard/host.go` (EvaluateHost, evaluateHost and every helper that had no other user: gridRoots, settingsAt, gateAt, projectWorkflowAt, hostFolder, homeHostFolder, taskAtFold, settingsFile, gateFile, the four reason strings). `taskName` stays in context.go.
- `internal/guard/context.go`: the `HostGuard` adapter removed. `internal/hook/hook.go`: the `guard.EvaluateHost(in, o.Cwd, env),` line removed from evaluateGuards (the hook now arbitrates plan and home only).
- Tests: legacy_test.go (HostGuard case), home_test.go (`"host": HostGuard`), case_test.go (host-guard in the level loop, the "feed" row, helper `f.host`, and the Feed file only that row wrote), settings_test.go (host-guard row and two comments).
- `internal/settings/claude.go`: the claudeHostPaths comment no longer names EvaluateHost; it says what each rule carries and what is not carried by decision. No rule changed.
- `internal/cli/setup_claude_host_test.go`: the guard oracle, the host-guard SetHook, the `env` map, the `taskFolder` map and three imports removed; the table's `want` is now the expectation, checked by the deny-rule match that was already there. No table row changed. `setup_claude_test.go`: comment only.
- Goldens: `golden/host-guard/*` (four cases), INDEX.md (four case lines and the `hooks/host-guard.mjs` hash), known-differences.txt (two lines, at :5-6 on this base, not :11-12), `"host-guard"` in corpus_test.go legacyHooks.
- New `internal/hook/la_host_drop_test.go` (helper `laHdRun`, test `TestLaHostDropHookNoLongerRefuses`).

## Deleted tests and what still covers the behaviour

| Deleted | Covered by |
|---|---|
| host_test.go (`.host` in the Grid home and beside the projects folder, case, depth, lookalike names pass) | `TestSetupWritesDenyRulesForTheHostGuardFiles` rows "host ...", "HOST ...", "host deep ...", "host-like name", "dot host file" (internal/cli/setup_claude_host_test.go). |
| host_test.go (symlink and alias spellings of `.host`; Bash text never read; legacy `.host` two levels up) | Removed by the owner's direction: simplify.md:39-40 drops the fixed gates and the hard enforcement around them; Claude Code's own path matching now judges spelling. The legacy `.host` is not carried by decision (claude.go "Not carried" paragraph). |
| host_gates_test.go, task_gates_test.go, host_approval_test.go, host_preview_test.go (task gate files, questions, approvals, previews) | `TestKsHostRulesGateFilesBelowHome` (internal/cli/ks_host_rules_test.go) and the "task ..." rows of `TestSetupWritesDenyRulesForTheHostGuardFiles`. |
| host_workflow_test.go, host_project_workflow_test.go (workflows.md, settings.json, lock) | The `<place> settings.json / .settings.lock / workflows.md` rows and "PROJECTS folder", "mixed case" rows of `TestSetupWritesDenyRulesForTheHostGuardFiles`. |
| the Feed rows in host_test.go (feed and feed-notes.md) and the "feed" row in case_test.go | `TestLaHookFeedCarriedBySetupRule` (internal/hook). |
| dy_hostguard_warn_test.go (warn by default; the warning text) | Removed by the owner's direction: simplify.md:39-40; there is no host-guard level to default any more. `TestLaHostDropHookNoLongerRefuses` proves the hook prints no deny at any level. |
| settings_test.go host-guard row, home_test.go host adapter, legacy_test.go HostGuard case | Removed with the guard (same direction). The plan-guard and home-allow rows still cover the level and bad-input behaviour. |
| golden host-guard cases | Removed with the guard; `golden/known-differences.txt` entries removed with them. Corpus test passes (`internal/golden` ok). |

## Rule map, as proven

- `.host` in the Grid home and beside the projects folder: deny rules `Edit(<anchor>/.host/**)` and `Edit(<parent>/.host/**)`, proven by TestSetupWritesDenyRulesForTheHostGuardFiles (default and "outside home" layouts).
- Legacy `.host` two levels above the projects folder; settings, lock and workflows files in or below the parent of a projects folder outside the Grid home: no rule, by decision (claude.go comment).
- Task gate files and the Feed: task-folder rules (claude.go:51-53); Feed by TestLaHookFeedCarriedBySetupRule.

## NOT VERIFIED on a live screen

- That Claude Code itself refuses a Write or Edit to these paths under the deny rules. The tests check that setup writes the rules and that each pattern matches the intended paths with the repo's own matcher; no `claude` run was made.
- A file-tool write to a Grid home settings or workflows file is now refused only by Claude Code, not by a Grid hook. A person who runs `grid setup` has the rules; one who has not has no refusal.

## Open items

- golden/INDEX.md:143 still says "No hook script is omitted" though the host-guard script hash is gone; the brief named only the listed lines, so it is left.
- docs/lanes/L4.md, L29.md and C-eo-hostguard-warn.md still describe host-guard; they are history.
- `internal/settings/catalog.go:30` still lists `host-guard` (WAIT-C-la-guard-levels), as the brief says.
