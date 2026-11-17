# M-P-jobs-land-g1-fix1-onto-main: lane P-jobs-land-g1-fix1 onto plugin main 4ca3d777

Carries P-jobs-land-g1-fix1. Merge commit 66a8aea8 (`git merge origin/main`). One conflict, in README.md.

## What changed

- README.md, Commands section: both sides added `grid run`. Main wrote `grid task`, `grid plan`, `grid job`, `grid run`, `grid lease` and a separate line for `grid run` (README.md:35 and :43). The lane wrote one longer wording on the task line. I kept main's two lines, which name `grid run` in the group list as the lane intended, so the lane's own wording is not repeated. README.md is now byte-equal to main's, and `TestLtReadmeNamesEveryCommand` passes on it.
- The branch now differs from origin/main only in docs/lanes/P-jobs-land-g1-fix1.md, docs/lanes/P-jobs-land-g1.md and this file.
- Main already holds the P-jobs-land-g1 doc work (c36bea62 is in origin/main) and Abdu's V12.c answer on top of it (e74e3923, 367de84f). So the line numbers in docs/lanes/P-jobs-land-g1.md (written before that) are stale for judge-land.md. Current numbers are in the table below. That file is left as it is: remove nothing.
- NOT DONE to CLOSED rewrite: `grep -rniE "not done" docs/lanes/P-jobs-land-g1.md docs/lanes/P-jobs-land-g1-fix1.md` gives no hit. Both carried lane docs hold no NOT DONE verdict, row or last line, so no rewrite was needed and none was made.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `git merge-base --is-ancestor origin/main HEAD` | HOLDS | exit 0 |
| 1. `git diff origin/main --stat` names only lane files | HOLDS | 2 files: docs/lanes/P-jobs-land-g1-fix1.md (+45), docs/lanes/P-jobs-land-g1.md (+2 -1), plus this file once committed. Both are the lane's own files. |
| 2. fix1 row 1: `go test ./internal/cli/ -run TestLtReadmeNamesEveryCommand -count=1 -v` | HOLDS | exit 0: `--- PASS: TestLtReadmeNamesEveryCommand`, `ok github.com/gridconsole/grid-plugins/internal/cli 0.007s` |
| 2. g1 step 1 (V26/V27 one decision) | HOLDS | run-jobs-launcher.md:603-633 (section C, points at :609-:627, duplicate-lanes map :629-633), :656-657 (lanes 14, 15 take JL 7, 8); judge-land.md:1002-1004 (lanes 7, 8 stubs, lane 9 deploy). `git grep -n -E "kind: approved\|run: command\|approval post id\|only assignment\|write-once\|grid-T-12"` on both docs: no hit (exit 1) |
| 2. g1 step 2 (V23) | HOLDS | run-jobs-launcher.md:23 and :434 carry "the hosted web UI belongs to the app and grid-sync, not the plugin" |
| 2. g1 step 3 (lane 13 V-numbers) | HOLDS | run-jobs-launcher.md:655 names V5, V6, V7, V8, V9, V10, V18, V20, V21, V22, V23, V24, V26, V27 |
| 2. g1 step 4 (V11.e) | HOLDS | judge-land.md:998 (lane 3 rows 8-11), :236 (each test names its row) |
| 2. g1 step 5 (V12.f drain) | HOLDS | judge-land.md:526 (Q4 row "ported: drain", with the reason; now marked decided by Abdu "V12.f confirmed"), :1001 (lane 6 row 8) |
| 2. g1 step 6 (V12.c) | HOLDS, superseded by the owner | The "Open for Abdu: V12.c" section no longer exists on main: Abdu answered, and main applied it (e74e3923, 367de84f). It is now judge-land.md:913 "Decided by Abdu: V12.c" (option B, a separate runner Job). This is main's change, kept as is; the lane did not touch it. |
| 2. g1 step 7 (last line, report) | HOLDS | run-jobs-launcher.md:688 and judge-land.md:1058 end with `DONE` |
| 3. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0, `ok .../internal/cli 0.044s` |
| 3. `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0, `ok .../internal/job/lint 0.149s` |
| 3. app-repo walk tests (`make webui-test`, `TestFvArgvCollects`, grid-server walks, slices Imports, `make realplugin`) | not run, not asked | plugin repo; they are app-repo checks |
| 4. `make lint` | HOLDS | exit 0 (`go vet ./...`) |
| 4. `make cross` | HOLDS | exit 0 (darwin/arm64 and windows/amd64: vet, then test compile of every package, all `ok`) |
| Tests of the changed packages and their importers | HOLDS | No Go file differs from main. `go test -count=1 ./internal/cli/ ./internal/job/... ./internal/machinefolder/ ./internal/doctor/ ./internal/packaging/`: all `ok`, exit 0 (cli 41.457s). |
| Whole-module test, cross-repo set | not run, not asked | the orchestrator runs them at landing |
| Private paths, hosts, emails in this file | none | |

## NOT VERIFIED on a live screen

Nothing here is UI.

DONE
