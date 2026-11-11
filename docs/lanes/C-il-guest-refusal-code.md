# C-il-guest-refusal-code: a guest's refused write prints the envelope with `guest_refused` under `--json`

Base: plugin main ee8a96c1. `git log --oneline -1 lane-C-hf-cli-failure-shapes` gives `161396e7 Reword the undo meaning so the README command check passes, and add the lane report.`, and `git merge-base --is-ancestor 161396e7 HEAD` holds. Merge note: `git fetch origin main && git merge origin/main` gives "Already up to date"; merged main sha ee8a96c1.

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
| 4 pins and README | HOLDS | guestParityEarlyAnswer gives Stdout `ilGuestEnvelope(stderr)` in json mode for the guest cases and keeps "" for `project repo` and `signal add` (guest_parity_test.go:411-470); guest_leaks_test.go:59-64 and :87-92 use assertErrorOutput for json rows and keep `h.out.Len() != 0` for plain ones; tree checks unchanged. README row contract/README.md:84, sentence :23. `go test ./internal/cli -count=1 -run 'TestGuestProjectParity|TestGuestProjectSettingsWrites|TestGuestClaudeSetupRemove|TestFvGuest|TestFyGuest|TestGhSession|TestStableErrorCodes|TestErrorCodeIgnoresWording|TestHfFs|TestIlGuest'` exit 0 (`ok … internal/cli 4.205s`); `go test ./internal/contract -count=1` exit 0 (`ok … internal/contract 10.144s`). TestIlGuestReadme fails on base: `README code table has 0 guest_refused rows, want 1`. |
| 5 lint, cross, diff | HOLDS | `make lint` exit 0 (prints only `go vet ./...`); `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, no error line); `git diff main --stat` before the report: contract/README.md, internal/cli/{agent_sync,cli,error_codes,guest_leaks_test,guest_parity_test,il_guest_refusal_code_test,projects,routine,settings}.go, all in Touches. |
| Rule: tests of changed packages and importers | NOT DONE | `go list` importers of internal/cli: cmd/grid, contract/formats, internal/account, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib (plus internal/contract for the README). All `ok` except internal/question: `--- FAIL: TestPersonWithdrawCLI/guest (0.09s) withdraw_cli_test.go:115: refused withdraw wrote data`. Cause below; the fix is outside Touches. |
| Rule: new tests fail on base, pass on lane | HOLDS | Failing lines quoted in rows 1-4. |
| Rule: no existing test edited to pass | HOLDS | Only the pins row 4 names were moved; internal/question/withdraw_cli_test.go is left as it is. |
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

## Open items

- internal/question/withdraw_cli_test.go:114 pins a guest's `question withdraw --json` stdout to empty or the `person_required` object. Rows 1 and 4 require the `guest_refused` envelope there. The brief's "Not touched, checked" list misses this test, and it is outside Touches. The fix is one clause that admits the envelope (`"code":"guest_refused"`) the same way the line already admits `person_required`. The feed and question-file checks on that line still hold: the gate returns before any handler runs. The whole-module run at landing fails here until Touches covers that line.
- Known open, unchanged: signal.go:65, settings.go:48 (store opener), budget.go:137, project_repo.go:76 (TestIlGuestSiteList keeps the list both ways).
- contract/README.md:86 "The first five constants" no longer counts the constants in error_codes.go exactly (there are now six). It was left as it is because the brief says not to edit :83-86.

NOT VERIFIED on a live screen: nothing (CLI only).

NOT DONE: every acceptance row 1-5 holds, but the importer test internal/question TestPersonWithdrawCLI/guest fails until its pin at withdraw_cli_test.go:114 moves. That line is outside Touches.
