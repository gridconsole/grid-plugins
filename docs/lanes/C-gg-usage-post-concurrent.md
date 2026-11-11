# C-gg-usage-post-concurrent: two processes post usage to one task book, every delta once and the same key once

Base: plugin main `b87fc36458332a9e2bd680ff9e3ed5a896638900` (lane branch `lane-C-gg-usage-post-concurrent`). Base read as the brief asks: `number` and `testStore` (usage_test.go:14-20), `resultStore` and `TestUsagePostKeyBooksOnce` (result_test.go:84-130), `usValid` and `usLogPath` (usage_store_log_test.go:16-18). `usLogPath` and `number` are reused; nothing else is.

Only file added besides this report: `internal/usage/gg_post_concurrent_test.go`. No production code, no existing test changed.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Distinct: 4 children x 10 deltas, no Key, Dollars 1; all exit 0; 40 complete valid lines; Show total 40 | HOLDS | TestGgUsageConcurrentDistinct, gg_post_concurrent_test.go:325 (children exit 0 via ggReports :186; log via ggLog :221 and ggDistinctWant :247). Each delta carries a unique Tokens value (id*1000+k), and the test compares the 40 values in the log with the 40 expected, so "every delta once" is checked per delta, not only by count. `go test -count=1 -v -run TestGgUsageConcurrent ./internal/usage/`: `--- PASS: TestGgUsageConcurrentDistinct`, exit 0 |
| 2 Same key: 8 children, one Key, Dollars 3; exactly one AlreadyBooked false, seven true; one line with the key; Show total 3 | HOLDS | TestGgUsageConcurrentSameKey, :337. Counts the children whose Booked is 0 (must be 1) and the Booked sum (must be 7); every child's last answer must be Dollars 3, Posts 1; log has one line with the key; Show 3 dollars, 1 post. `--- PASS`, exit 0 |
| 3 Torn tail: log cut as TestFoldAndRepair does (`{"v":1` appended after 3 complete lines), then 4 children Post at once: all exit 0, earlier lines + 4 new complete lines, no torn line | HOLDS | TestGgUsageConcurrentTornTail, :368. Cut line written at :380; after the run the log must start with the exact earlier bytes (`bytes.HasPrefix`), hold 7 lines, end in `\n`, every line valid JSON, 4 distinct new records; Show 10 dollars, 7 posts. `--- PASS`, exit 0 |
| 4 Show while posting: parent calls Show in a loop until every child of row 1 has exited; no error, total never decreases | HOLDS | TestGgUsageConcurrentShowWhilePosting, :414. Same 4 x 10 children as row 1 (ggDistinctEnv :244); the loop at :423-437 fails on any Show error or any drop of the total, and ends when the goroutine that waits on the children hands back their outcomes. Afterwards the same 40-delta checks as row 1. `--- PASS`, exit 0 |
| 5a `git diff main --stat` names only the new file | HOLDS | before this report was added: ` internal/usage/gg_post_concurrent_test.go | 444 +++` / `1 file changed`. With this report: the new test file and `docs/lanes/C-gg-usage-post-concurrent.md` (the lane report the runbook asks for) |
| 5b `go test -count=3 -race -run TestGgUsageConcurrent ./internal/usage/` | HOLDS | `ok github.com/gridconsole/grid-plugins/internal/usage 19.631s` (includes compiling the race build), exit 0, 0 fail. Single `-race -v` run on a loaded VM: Distinct 1.67 s, SameKey 2.06 s, TornTail 1.09 s, ShowWhilePosting 1.57 s, package 7.752 s |
| 5c `make lint` | HOLDS | prints `go vet ./...`, gofmt -l empty, exit 0 |
| Tests fail when the property breaks (mutation proof; the lane changes no production code, so base passes too) | HOLDS | Temporary edits of `internal/usage/usage.go`, each reverted with `git checkout -- internal/usage/usage.go` (`git status --short` empty after each). (a) removed the `fslock.LockExclusive` call in `withData`: exit 1, `--- FAIL: TestGgUsageConcurrentDistinct ... child 1: exit status 1 ... usage.jsonl line 23: invalid character '\x00' looking for beginning of value` and `--- FAIL: TestGgUsageConcurrentShowWhilePosting ... Show 67: usage.jsonl line 36: invalid character '\x00' looking for beginning of value`. (b) lock kept, key check made `... && false`: exit 1, `--- FAIL: TestGgUsageConcurrentSameKey ... child 1: {ID:1 Posts:1 Booked:0 Dollars:6 TotalPosts:2}`. Restored code: all PASS again |
| Importers of `internal/usage` pass | HOLDS | from `go list` (direct importers): `internal/cli`, `internal/hook`. `go test -count=1 ./internal/usage/ ./internal/hook/ ./internal/cli/`: `ok usage 1.377s`, `ok hook 8.945s`, `ok cli 62.269s`, exit 0 |
| Windows and macOS builds | HOLDS | `make cross` exit 0 (vet and test-compile for darwin/arm64 and windows/amd64; `ok ... internal/usage` in both) |
| Two processes, children re-run the test binary as delivery_test.go:425 does; env marker the helper skips without | HOLDS | `exec.Command(os.Args[0], "-test.run=^TestGgUsageConcurrentChild$")` at :132; the child body (:276) skips unless `GG_USAGE_CHILD=1` (one SKIP line in a plain `-run TestGgUsageConcurrent`, as the brief expects) |
| Start barrier: all children started before any wait; stdin pipes closed only after all have started; no sleep, no timing assertion | HOLDS | ggStart (:119) starts every child (`StdinPipe`, `Start`) and then blocks on a channel receive until each child has written its `GGWAITING` line on stderr, which it does just before it reads stdin; only then does `release` (:168) close every pipe. `grep -n 'Sleep\|time.After\|time.Since' internal/usage/gg_post_concurrent_test.go`: no hit. Nothing is timed; the parent holds on exit codes and pipe EOF |
| Each child prints only its own result line | HOLDS | the child's only stdout write is one `GGRESULT {json}` line with its own id (:321); ggReports requires exactly one such line per child and that its id is the child's own. The Go test runner adds its own `PASS` line, which is ignored. The `GGWAITING` handshake goes to stderr, not stdout |
| Whole file under 20 s on a loaded machine | HOLDS | 7.752 s for the package with `-race -v` on this VM; each test 1-2 s |
| Home isolation: HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME set to folders under t.TempDir; t.Fatal when HOME is not under the root; same four in every child's cmd.Env | HOLDS | ggIsolate :66 (t.Setenv of the four, check at :78); every child gets the same four in cmd.Env (:133) and re-checks all four against the parent's root in the child body (:280-283). No test of the file uses Env.Getenv: the usage Store reads no environment, and `git grep Getenv internal/usage` finds nothing. Only variable names are logged |
| No existing test edited or deleted | HOLDS | diff is one new test file and this report |
| No guard, gate, cap or hold added on agents; no production code changed | HOLDS | `git diff main --stat` shows tests and this report only |
| Commits: small, one sentence, no attribution | HOLDS | `Test that concurrent processes post usage to one task book once per delta and once per key.` and the report commit |
| Real-plugin tests | not applicable | no test of the lane is named *RealPlugin*; no built plugin was needed |
| No personal paths, hosts or emails in the public repo | HOLDS | the test file and this report hold none |

## What changed

New `internal/usage/gg_post_concurrent_test.go`: TestGgUsageConcurrentDistinct, TestGgUsageConcurrentSameKey, TestGgUsageConcurrentTornTail, TestGgUsageConcurrentShowWhilePosting, and TestGgUsageConcurrentChild (the body each child process runs; it skips unless a parent started it). Helpers all start with `gg`.

One addition to the brief's barrier: a child also writes one `GGWAITING` line to stderr when it is blocked on stdin, and the parent waits for all of those before it closes the pipes. Without it, `cmd.Start` returning only means the process was forked, so a slow child could still be starting up while the fast ones are released and the writes would not be sure to overlap.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run: a Windows host (the test file compiles and vets for windows/amd64 only; the lock there is a different fslock implementation), and a macOS host.

Not proven by a single run: that the overlap hits the narrow window between two processes reading the log. The mutation runs show the tests go red when the lock or the key check is gone, but with the lock removed the SameKey and TornTail tests passed in one run (Distinct and ShowWhilePosting failed); the 40-line tests carry the lock proof, the key check is proved by mutation (b).
