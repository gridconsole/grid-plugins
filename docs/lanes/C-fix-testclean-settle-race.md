# C-fix-testclean-settle-race

Base: cd84f424 (the brief cites main 3a3d99ea / 78b08bc9). Every cited line still shows the cause on cd84f424: `settle` at main.go:23, `deadline` at :57, the break at :71, the sleep at :74, and main_test.go:13 and :17-34. Nothing moved.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1a. On base the new tests do not compile | HOLDS | With base `main.go` checked out over the lane's tests, `go test ./internal/testclean/ -count=1` exits 1: `internal/testclean/settle_race_test.go:15:33: undefined: pause` (also main_test.go:20,21,24) |
| 1b. With only the seam on base the test fails on the assertion | HOLDS (deviation) | Base plus the `var pause` and the `pause(50 * time.Millisecond)` call only: `go test ./internal/testclean/ -count=1 -run 'Settle\|Transient'` exits 1: `settle_race_test.go:39: transient entry reported: status 1: temp leak check: new entry grid-eval-other` and `settle_race_test.go:67: pause was never called before the entry was reported`. **Deviation:** I could not make the scratch copy outside the clone, because Bash and Write to /srv/gridwork/tmp were denied. I applied the seam-only edit inside the clone, uncommitted, ran the test, then ran `git checkout HEAD -- internal/testclean/main.go`. `git status --short` is empty afterwards. |
| 2. A staying entry is still reported, pause called at least once, entry not removed | HOLDS | `TestSettleReportsEntryThatStaysAndDoesNotRemoveIt` (settle_race_test.go:47): settle 0, no-op pause; asserts status 1, `temp leak check: new entry grid-eval-stays`, `calls >= 1`, and `os.Stat` shows the entry present. `go test ./internal/testclean/ -count=1` exit 0 |
| 3. No `go func`, no `time.Sleep` in the tests | HOLDS | `git grep -n "time.Sleep\|go func" -- internal/testclean/main_test.go internal/testclean/settle_race_test.go` prints nothing (exit 1 = no match). The transient test now removes the entry inside the replaced `pause` (main_test.go:17-35) |
| 4. Load proof: 200 runs, `-cpu 1,4`, eight busy loops | NOT DONE (load part) | Starting `sh -c 'while :; do :; done'`, `pkill` and `pgrep` was refused by the allow list, so I could not create the load. Unloaded: `go test ./internal/testclean/ -run 'Transient\|Settle' -count=200 -cpu 1,4` prints `ok  github.com/gridconsole/grid-plugins/internal/testclean 0.129s`, exit 0. The tests use no clock or goroutine: the pause removes the entry, and with `settle = 0` the loop cannot stop before one pause and rescan. Load cannot change their order. The orchestrator can run the loaded command. |
| 5a. Production behaviour unchanged | HOLDS | `settle` is still `10 * time.Second` (main.go:23). `pause` defaults to `time.Sleep` and is called as `pause(50 * time.Millisecond)` (main.go:26, :79). `main()` and `prefixes` are untouched. |
| 5b. `go test ./internal/testclean/ -count=1` | HOLDS | `ok`, exit 0 |
| 5c. `make lint` | HOLDS | prints `go vet ./...`, exit 0. `gofmt -l internal/testclean` prints nothing. |

## Rules

| Rule | Status | Evidence |
|---|---|---|
| Packages that import the changed package | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./...` lists only `internal/testclean` for itself. It is `package main` and no other package imports it, so there are no dependents to test. |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base, pass on lane | HOLDS | rows 1a and 1b above; lane: `go test ./internal/testclean/ -count=1` exit 0 |
| Existing tests not edited to pass | HOLDS | The only existing test edited is `TestTransientEntryOfAnotherProcessIsIgnored`, as the brief directs. `TestMain`'s 100 ms is unchanged. No test deleted. |
| No guard, gate, cap or hold on agents | HOLDS | Only the leak-check test tool changed. It still reports and never deletes. |
| No personal paths, hosts or emails; no attribution lines | HOLDS | The commit message is one sentence with no trailers. |
| Whole-module and cross-repo tests | not run | The orchestrator runs them at landing. |

## What changed

- `internal/testclean/main.go`: new `var pause = time.Sleep`. The settle loop calls `pause(50 * time.Millisecond)` where it slept before. A `paused` flag means an entry seen on a scan is reported only after at least one pause and rescan (`len(leaked) == 0 || (paused && !time.Now().Before(deadline))`).
- `internal/testclean/main_test.go`: `TestTransientEntryOfAnotherProcessIsIgnored` now removes its entry from inside a replaced `pause`. It has no goroutine and no sleep.
- `internal/testclean/settle_race_test.go` (new): the two tests for rows 1 and 2.

## NOT VERIFIED on a live screen

There is no UI. The loaded run (row 4) is not done, as explained above.
