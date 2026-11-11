# C-il-guest-refusal-code: a guest's refused write prints the envelope with `guest_refused` under `--json`

Base: plugin main ee8a96c1. `git log --oneline -1 lane-C-hf-cli-failure-shapes` gives `161396e7 Reword the undo meaning so the README command check passes, and add the lane report.`, and `git merge-base --is-ancestor 161396e7 HEAD` holds. Merge note (fix round 1): `git fetch origin main` moved main ee8a96c1..31824184; `git merge origin/main` conflicted in internal/cli/error_codes.go only (the const block and errorCode). Both sides are kept: main's `CodeWorkflowFileChanged` and its `workflow.ErrChanged` case, then this lane's `CodeGuestRefused` and its `ilGuestRefused` case (error_codes.go:29-33, :50-53). contract/README.md and guest_parity_test.go merged without conflict: main's `workflow_file_changed` row (:78) and brain/workflow cases (guest_parity_test.go:445-471) stay as main has them. Merged main sha 31824184 (merge commit cc1e6de8).

Base check (row 1 shape, guest `task close T-01 --json`): on ee8a96c1 with only the constant added, json stdout is empty:
```
--- FAIL: TestIlGuestGateWalk/task_close (0.00s)
    il_guest_refusal_code_test.go:99: error JSON: unexpected end of JSON input: 
```

## Proof table

| Row / rule | Verdict | Proof |
| --- | --- | --- |
| 1 TestIlGuestGateWalk | HOLDS | `go test ./internal/cli -count=1 -v -run '…|TestIlGuest'` exit 0: 17 gate keys PASS (question withdraw, seven session writes, setup, task close/delete/lane/move/new/reopen/restore/stop), member and owner `task close T-01 --json` and `setup --yes --json` PASS, `guest_with_task_lease/task_new` PASS. The key set comes from guestParityCommands and guestWriteVerb (il_guest_refusal_code_test.go:66-75); one key per case is checked at :101-105. Fails on base: 17 `error JSON: unexpected end of JSON input:` lines (all keys). |
| 2 TestIlGuestSettingsSites | HOLDS | Same run: guest and external × routine add, project relink, setup --yes, setup --remove, agent sync PASS (exit 1 plain and json, stderr equal, assertErrorOutput guest_refused, GRID_HOME and HOME trees unchanged); `known_open/signal_add` PASS (exit 1, empty stdout). Fails on base: 10 `il_guest_refusal_code_test.go:166: error JSON: unexpected end of JSON input:` lines. |
| 3 TestIlGuestSiteList | HOLDS | Same run: `package` and `reports_a_site` PASS. The walk counts calls inside function bodies only, so the sentinel's own declaration (cli.go:27) is no site; the second case feeds a source with that declaration plus one call in a function and expects 1. Fails on base: `the walk finds 8 sites, want the 4 known open: map[agent_sync.go:1 budget.go:1 project_repo.go:1 projects.go:1 routine.go:1 settings.go:2 signal.go:1]`. |
| 4 pins and README | HOLDS | guestParityEarlyAnswer gives Stdout `ilGuestEnvelope(stderr)` in json mode for the guest cases and keeps "" for `project repo` and `signal add` (guest_parity_test.go:411-483 after the merge); guest_leaks_test.go:59-64 and :87-92 use assertErrorOutput for json rows and keep `h.out.Len() != 0` for plain ones; tree checks unchanged. README row contract/README.md:85, sentence :23. After the merge: `go test ./internal/cli -count=1 -run 'TestGuestProjectParity|TestGuestProjectSettingsWrites|TestGuestClaudeSetupRemove|TestFvGuest|TestFyGuest|TestGhSession|TestStableErrorCodes|TestErrorCodeIgnoresWording|TestHfFs|TestIlGuest'` exit 0 (`ok … internal/cli 6.925s`); `go test ./internal/contract -count=1` exit 0 (`ok … internal/contract 2.400s`). TestIlGuestReadme fails on base: `README code table has 0 guest_refused rows, want 1`. |
| 5 lint, cross, diff | HOLDS | After the merge: `make lint` exit 0 (prints only `go vet ./...`); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, every package `ok`); `git diff origin/main --stat`: contract/README.md, docs/lanes/C-il-guest-refusal-code.md, internal/cli/{agent_sync,cli,error_codes,guest_leaks_test,guest_parity_test,il_guest_refusal_code_test,projects,routine,settings}.go, internal/question/withdraw_cli_test.go: the files in Touches, the report, and the one line fix round 1 added to Touches. |
| Fix round 1: withdraw_cli_test.go:114 | HOLDS | The clause that already allowed `"error":"person_required"` now also allows `"code":"guest_refused"`; the feed-record and question-file checks on that line stay. Before the fix: `--- FAIL: TestPersonWithdrawCLI/guest (0.04s) withdraw_cli_test.go:115: refused withdraw wrote data`. After: `go test ./internal/question -count=1` exit 0 (`ok … internal/question 1.963s`, merged tree). Commit 84ace00e. |
| Rule: tests of changed packages and importers | HOLDS | `go list` importers of internal/cli: cmd/grid, contract/formats, internal/account, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib (plus internal/contract for the README). After the merge, `go test ./... -count=1` prints no line other than `ok` and `[no test files]`, so no package fails (filtered with `grep -v '^ok\|no test files'`: empty output). |
| Rule: new tests fail on base, pass on lane | HOLDS | Failing lines quoted in rows 1-4. |
| Rule: no existing test edited to pass | HOLDS | Only the pins row 4 names and the withdraw_cli_test.go:114 line that fix round 1 added to Touches were changed. |
| Rule: no guard, gate, cap or hold added | HOLDS | Which roles are refused is unchanged; the gate only records a code (cli.go:202). |
| Rule: small commits, no attribution, no personal paths | HOLDS | `git log origin/main..` shows one-sentence messages with no trailers. |
| Rule: helper prefix | HOLDS | Test helpers and the production sentinel `ilGuestRefused` (cli.go:27) use the `ilGuest` prefix the brief sets for package-level vars; lint is vet+gofmt, so no name linter asks for an `err` prefix. |
| UI | n/a | No UI. |

## What changed

- internal/cli/error_codes.go: `CodeGuestRefused = "guest_refused"`; errorCode maps `ilGuestRefused` to it.
- internal/cli/cli.go: sentinel `ilGuestRefused = errors.New(guestSettingsRefusal)`; the early guest gate calls `env.recordError(ilGuestRefused)` once before its branches, so finish prints the envelope with `--json`. Exits and stderr are unchanged.
- projects.go:105, routine.go:66, settings.go:176, agent_sync.go:44 return the sentinel (same message bytes).
- contract/README.md: one sentence under shape 1, one table row.
- Tests: new il_guest_refusal_code_test.go; pins moved in guest_parity_test.go and guest_leaks_test.go.
- Fix round 1: internal/question/withdraw_cli_test.go:114 accepts the `guest_refused` envelope next to `person_required`; main 31824184 merged in.

## Open items

- Known open, unchanged: signal.go:65, settings.go:48 (store opener), budget.go:137, project_repo.go:76 (TestIlGuestSiteList keeps the list both ways).
- contract/README.md:87 "The first six constants" (main's wording after workflow_file_changed) no longer counts the constants in error_codes.go exactly: there are now seven. I left it as it is because the brief says to edit that paragraph only where TestHfFsCodeTable needs it, and TestHfFsCodeTable passes.

NOT VERIFIED on a live screen: nothing (CLI only).

DONE
