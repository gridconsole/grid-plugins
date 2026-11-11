# C-la-hook-repin

The hook package tests stop leaning on the host guard's deny; the Feed is proven carried by the deny rules `grid setup` writes.

Base: main at a7809a58 (`git merge main` answered "Already up to date"). Code head: b80a82e6. Test files only; no production file changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| Step 1: base check | HOLDS | The four tests exist on the base and carry the `host-guard` lines the brief names: `git grep -n 'host-guard'` on the base printed tooluse_test.go:32, :35, :219, :220, events_test.go:183, :184, gg_guarded_inputs_test.go:171, :175. None was re-pinned. |
| 1. no `host-guard` in the three files | HOLDS | `git grep -n 'host-guard' -- internal/hook/tooluse_test.go internal/hook/events_test.go internal/hook/gg_guarded_inputs_test.go` prints nothing (exit 1, no match). |
| 2. TestLaHookFeedCarriedBySetupRule | HOLDS | internal/hook/la_hook_feed_rule_test.go. Rules from `settings.ClaudeBlock(home, home/.grid)`; the three Feed paths in `~/.grid/...` form each matched by `path.Match` against a deny `Edit(...)` pattern with `/**` cut, on the task folder. The third path (`other/tasks/T-1`) has exactly one carrier (claude.go:53) and that rule carries nothing outside a task folder. `projects/example/notes.md` is matched by none. Subtests PASS in the row 3 run. |
| 3. row 3 command | HOLDS, exit 0 | `go test -count=1 -v -run 'TestGuardDispatch\|TestEventAtTheLimitIsStillGuarded\|TestInternalErrorsAllowAndGuardStillDenies\|TestGgGuarded\|TestLaHook' ./internal/hook/` printed `--- PASS:` for TestInternalErrorsAllowAndGuardStillDenies, TestGgGuardedContrastRecordWellFormedNotCut, TestLaHookFeedCarriedBySetupRule, TestGuardDispatch, TestEventAtTheLimitIsStillGuarded (and TestGgGuardedInputsNeverDecide, TestGgGuardedEventsMatchTheRegistration), then `ok`. |
| 4. holds after the host drop | HOLDS, exit 0 | With `guard.EvaluateHost(in, o.Cwd, env),` deleted from internal/hook/hook.go (working tree only), the same command printed the same seven `--- PASS:` lines and `ok`. `git checkout internal/hook/hook.go` restored it; `git status --short` printed nothing. `git diff main --stat` (before this report file was added) names only internal/hook/events_test.go, gg_guarded_inputs_test.go, la_hook_feed_rule_test.go, tooluse_test.go; the report docs/lanes/C-la-hook-repin.md is the only addition since. |
| 5. after the merge | HOLDS, exit 0 | `go test -count=1 ./internal/hook/` is part of `go test -count=1 ./internal/hook/ ./internal/cli/ ./internal/golden/ ./internal/packaging/`: all four `ok`. |
| 6. lint and cross | HOLDS, exit 0 | `make lint` (go vet ./...) and `make cross` (darwin/arm64 and windows/amd64 vet and test compile) finished with no failure. |
| Importers of the changed package | HOLDS | `go list` over `./...` for `internal/hook` in Imports, TestImports and XTestImports gives internal/cli, internal/golden, internal/packaging. All three `ok` in the run above. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok. `go test -count=1 ./internal/job/lint/` ok. The app-only walks do not apply to the plugin. |
| New tests fail on base, pass on lane | HOLDS for the re-pins; see note for the Feed test | See "Red and green" below. |
| No deleted test | HOLDS | No test was deleted. TestGuardDispatch lost one table row ("write feed"); its Feed proof moved to TestLaHookFeedCarriedBySetupRule. TestGgGuardedContrastDenyWellFormedNotCut was renamed to TestGgGuardedContrastRecordWellFormedNotCut and rewritten as the brief says. |
| No guard, gate, cap or deny rule added | HOLDS | Test files only. internal/guard, internal/settings, internal/cli and the two stored-level tests (dy_recall_hook_test.go, precompact_records_test.go) are untouched. |
| `*RealPlugin*` tests | Not applicable | No row names one. |

## Red and green

The old tests on main, with `guard.EvaluateHost(...)` deleted from hook.go, go red (brief's reason for the lane):

```
events_test.go:189: guard lost priority:  0
--- FAIL: TestInternalErrorsAllowAndGuardStillDenies
gg_guarded_inputs_test.go:188: well formed write was not denied: out "" stderr "" exit 0
--- FAIL: TestGgGuardedContrastDenyWellFormedNotCut
tooluse_test.go:71: got allow, want deny: ... (pretooluse and permissionrequest write feed)
--- FAIL: TestGuardDispatch
tooluse_test.go:233: the host write was not judged by the guards: ... "permissionDecision":"allow"
--- FAIL: TestEventAtTheLimitIsStillGuarded
```

The lane tests with the same line deleted pass (row 4). They are not vacuous: with `guard.EvaluateHome(...)` also deleted from hook.go (working tree only, then `git checkout`), the two role-refusal tests go red:

```
events_test.go:188: guard lost priority:  0
--- FAIL: TestInternalErrorsAllowAndGuardStillDenies
tooluse_test.go:230: the guest write into Personal was not judged by the guards:
--- FAIL: TestEventAtTheLimitIsStillGuarded
```

TestLaHookFeedCarriedBySetupRule states a fact about setup that already holds on this base (main carries claude.go:53 from C-ks-host-rules), so it passes on base and lane; it did not exist on main. On 35a498dc the third path was matched by no rule, per the brief. I did not rebuild that older rule set (internal/settings is not mine to edit). Its non-vacuity checks are the exactly-one-carrier assertion for the third path and the `notes.md` control that no rule matches.

## What changed

- tooluse_test.go: TestGuardDispatch drops the "write feed" row and the host-guard `SetHook` with its comment. TestEventAtTheLimitIsStillGuarded keeps its subject (an event of exactly `hookio.InputLimit` bytes is still judged) but the judged call is a Write into `<grid>/personal/x` with GRID_ROLE `guest`, GRID_MEMBER `gus`, GRID_PROJECT `example`, and it must be denied with the home guard's "This caller cannot access this Grid home scope."
- events_test.go: TestInternalErrorsAllowAndGuardStillDenies drops the settings.json write; after the logged SessionEnd error, a guest PermissionRequest Write to `<grid>/personal/notes.md` is still denied.
- gg_guarded_inputs_test.go: the contrast test is TestGgGuardedContrastRecordWellFormedNotCut. A well-formed PreToolUse Bash input (session id, tool use id, cwd the task folder) leaves a snapshot under its tool use id; the same input cut at 40 bytes leaves none and answers no decision.
- la_hook_feed_rule_test.go (new): TestLaHookFeedCarriedBySetupRule and the `laHook` helpers.

## NOT VERIFIED on a live screen

Nothing here is UI. `path.Match` on the pattern with `/**` cut is the brief's stand-in for Claude Code's own deny matching; I did not run Claude Code, so that the real matcher carries the same paths is not verified.
