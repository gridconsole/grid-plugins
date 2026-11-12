# C-ya-usage-ready-reset

A Job parked on usage_limit turns Ready when the reset time in its worker-exit `code` has passed or an account has room, not on the next tick.

Base: main 2b47f621. Base check: `internal/orchestrator/tick.go:257` read `s.UsageReady(j.Assign.To)` (behind `j.Reason == "usage_limit" && s.UsageReady != nil`); `grep -rn 'j\.ExitCode' internal/orchestrator` hit only `replan.go:38`; neither UsageReady setter read a time or windows. The lane went ahead.

## Proof table

| Row | Verdict | Proof |
| --- | --- | --- |
| 1. `TestYaUr` cases | HOLDS | `go test -count=1 -v -run 'TestYaUr' ./internal/orchestrator/` exit 0, `--- PASS: TestYaUrReoffer` and 16 `--- PASS: TestYaUrReoffer/...` cases. Cases: `reset_in_the_future,_no_account_numbers` (Parked), `clock_past_the_reset` (Ready, control record with reason "usage capacity restored"), `reset_in_the_future,_account_has_room` (Ready), `no_code,_no_account_numbers,_as_on_main` (Ready), `code_prepare_failed_reads_as_no_code` (Ready), `not_the_holder,_clock_past_the_reset` / `not_the_holder,_no_code` / `not_the_holder,_account_has_room` (Parked), `UsageReady_unset,_clock_past_the_reset` (Parked), and numeric-string and known-and-full variants. |
| 1. red on main | HOLDS | With main's tick.go (`git checkout 2b47f621 -- internal/orchestrator/tick.go`), the new test file and the store field: `--- FAIL: TestYaUrReoffer/reset_in_the_future,_no_account_numbers`, `--- FAIL: TestYaUrReoffer/reset_in_the_future,_account_known_and_full`, `--- FAIL: TestYaUrReoffer/reset_as_a_numeric_string,_in_the_future`, `--- FAIL: TestYaUrReoffer/no_code,_account_known_and_full`, `--- FAIL: TestYaUrReoffer/code_prepare_failed,_account_known_and_full`, `--- FAIL: TestYaUrWalk`; message: `ya_usage_ready_test.go:110: want Parked; assigned 1, restored true, job ... Status:assigned` and `ya_usage_ready_test.go:158: before the reset: assigned 2, job ... Status:assigned`. tick.go restored with `git checkout HEAD -- internal/orchestrator/tick.go`; `git status --short` empty. |
| 2. walk | HOLDS | `TestYaUrWalk` (`ya_usage_ready_test.go:121`) `--- PASS`: J-1 acked, `yaUrExit` with T an hour ahead, tick: Parked; `Ready(...).Waiting` holds J-2 with reason "usage_limit"; tick again and tick at T-30min: J-1 Parked, `Assigned` 0, J-2 still held; clock at T+1h: `Assigned` 2, J-1 `Assign.Fence` above the parked fence, J-2 no longer held. |
| 3. `TestYaRoom` | HOLDS | `go test -count=1 -v -run 'TestYaRoom' ./internal/cli/` exit 0: `--- PASS: TestYaRoomReadsTheWindows/one_claude_account,_5h_window_used_40` (room, known), `/every_window_used_100` (no room, known), `/every_account_source_none` (not known), `/a_codex_account_only` (not known), plus five more cases. |
| 4. docs | HOLDS | `git grep -n 'reset time' -- contract/docs/feed-posts.md` hits `contract/docs/feed-posts.md:735`, in the worker-exit section (:732-763). |
| 4. packages | HOLDS | `go test -count=1 ./internal/orchestrator/ ./internal/account/ ./internal/contract/` exit 0 (orchestrator 172.7s, account 6.7s, contract 2.2s); `TestUsageLimitReoffer` and `TestStreamUsageReadyIsForTheHolderOnly` unedited and in that run. |
| Importers of the changed packages | HOLDS | `go list` import scan: orchestrator is imported by `internal/cli`, `internal/mcp`, `internal/migrate`; cli by `cmd/grid`, `lib`. `go test -count=1 ./internal/cli/ ./internal/mcp/ ./internal/migrate/ ./cmd/grid/ ./lib/` exit 0 (cli 88.2s, mcp 0.9s, migrate 24.6s, lib 24.9s, cmd/grid has no test files). |
| 5. walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0; `go test -count=1 ./internal/job/lint/` exit 0. |
| 5. lint | HOLDS | `make lint` (`go vet ./...`) exit 0; `gofmt -l internal/orchestrator internal/cli` empty. |
| 5. Windows and macOS builds | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok`. |
| 5. diff | HOLDS | `git diff main --stat` names `contract/docs/feed-posts.md`, `internal/cli/orchestrator.go`, `internal/cli/ya_account_room.go`, `internal/cli/ya_account_room_test.go`, `internal/orchestrator/store.go`, `internal/orchestrator/tick.go`, `internal/orchestrator/ya_usage_ready.go`, `internal/orchestrator/ya_usage_ready_test.go`, and this table. |
| Rule: new tests fail on base, pass on lane | HOLDS | See "red on main". No existing test edited or deleted. |
| Rule: no guard, gate, cap or hold on agents | HOLDS | The change only decides when a Job already parked on usage_limit is offered again; it adds no limit and the unknown case (no reset, no numbers) stays Ready as on main. |
| Rule: *RealPlugin* tests | HOLDS | None named by the brief; none run. |
| Rule: commit identity and messages | HOLDS | One-sentence messages, no attribution lines. |

## What changed

- `internal/orchestrator/store.go`: new optional `AccountRoom func() (room, known bool)` beside `UsageReady`, which keeps its type.
- `internal/orchestrator/tick.go:257`: `s.yaUsageReady(j)` replaces `s.UsageReady != nil && s.UsageReady(j.Assign.To)`.
- `internal/orchestrator/ya_usage_ready.go`: the holder check first; then Ready when `j.ExitCode` holds a reset time (a JSON number or a numeric string of unix seconds above zero, read with `gqCauseText`) at or before `s.now()`; else Ready when `AccountRoom` says room; else Parked while a future reset time or known account numbers exist; else Ready.
- `internal/cli/ya_account_room.go` and `configureTickStore` (`internal/cli/orchestrator.go:580`): `AccountRoom` reads `account.Windows(accountCoreEnv(env), "")`; room when some Claude account has Source not "none" and every window is used below 100; known when some Claude account has Source not "none". An unreadable registry answers not known.
- `contract/docs/feed-posts.md:735`: one sentence that a usage_limit `code` is the reset time in unix seconds when the worker knows it.

## Where the lane differs from the brief

- `known` counts Claude accounts only, where the brief says "any account". `windowsFor` gives every Codex account Source "none" today, so the answers are the same; the narrower reading stops a future Codex source from holding a Job with no reset time forever, since room is read for Claude only.
- A Claude account with Source not "none" and no windows reads as room (every window is below 100, vacuously). A Job with no reset time is then offered as on main.
- `Output.Waiting` does not exist on the tick's `Output` (`Rows`, `Wake`, `Assigned`, `Spawns`); the held list is `Ready(...).Waiting`, which `TestYaUrWalk` reads through `Load` and `Ready` as `TestUsageHoldBlocksHostDemand` does.
- The walk posts J-2 after J-1 has parked, as `TestUsageHoldBlocksHostDemand` does, so J-2 is a Ready Job the park can hold. I did not run the brief's order (both posted before the first tick), where the first tick would assign both. After the reset the tick assigns both (`Assigned` 2), not only J-1.
- The brief names `s.Now()`; the code uses `s.now()` (UTC, falls back to the wall clock), which is what the rest of tick.go uses.

## NOT VERIFIED

- Not on a live screen or a live account: `yaAccountRoomFor` (the closure over `account.Windows`) has no test of its own; the pure `yaAccountRoom` is tested, and `account.Windows` has its own tests. No real Claude Code status line snapshot was read.
- The app worker posts no usage_limit today (WAIT-C-ya2-worker-posts-usage-limit), so no end-to-end run of a real usage_limit exit exists; the exit is posted by `yaUrExit` in the tests.
