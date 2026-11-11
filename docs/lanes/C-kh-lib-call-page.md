# Lane C-kh-lib-call-page: a contract page for the lib Call

Base: plugin main `dc301fce757d6374de26d8d1b68de26575341b32`. Call fields on the base (`git grep -n '^	[A-Z]' main -- lib/lib.go`): Args 74, GridHome 78, Home 81, Dir 84, Environ 89, Stdin 93, Stdout 94, Stderr 95, Executable 99, OpenURL 101, Context 109, OwnerPID 116, OwnerStart 117. `contract/docs/lib-call.md` did not exist on the base.

## Proof table

| Row | Result |
| --- | --- |
| 1. `go test -count=1 -v -run 'TestT12Lib' ./contract/` prints `--- PASS: TestT12LibFieldRows` | HOLDS, exit 0. Reflect walk of `lib.Call`: 13 fields, 13 rows, no extra row; the type cell is checked too (contract/t12_lib_call_test.go:112-136). |
| 2. `--- PASS: TestT12LibExitCodes` | HOLDS, exit 0. Table holds 0,1,2,3,4,5,130; `lib.Run` of `t12-no-such-verb` returned 2, as the page gives (contract/t12_lib_call_test.go:139-156). |
| 3. `--- PASS: TestT12LibExample` | HOLDS, exit 0. The test reads Args and Environ out of the page's go block, runs them with a temp GridHome, exit 0, Stdout validates against `cmd-version` with `contract.Validate`. |
| 4. `--- PASS: TestT12LibNegative` | HOLDS, exit 0. A synthetic table without the `OwnerStart` row reports `[OwnerStart]` (and a row for `NoSuchField` is reported as extra). |
| Extra: `--- PASS: TestT12LibCitations` | HOLDS. Every table row and bullet in the three sections cites `lib/lib.go:N` or `N-M` inside the file's line range. This checks the range, not that the line says what the page says. |
| 5. `git diff main --stat` names only the two new files | HOLDS: contract/docs/lib-call.md, contract/t12_lib_call_test.go (319 insertions). The lane report below is also in the commit tree as docs/lanes/C-kh-lib-call-page.md, as the runbook requires. |
| 5. `make lint` | HOLDS, exit 0 (`go vet ./...`). `gofmt -l contract/` prints nothing. |
| 5. `make cross` | HOLDS, exit 0: darwin/arm64 and windows/amd64 vet and test compile pass. |
| Rule: new tests fail without the lane | HOLDS. Both files are new, so the tests do not exist on the base. Mismatch run, renaming the `OwnerStart` row to `OwnerStop` in the page, then `git checkout -- contract/docs/lib-call.md`: exit 1, `--- FAIL: TestT12LibFieldRows` / `Call fields table: no row for [OwnerStart]; row for a field the struct lacks [OwnerStop]`. `git status --short` was clean afterwards. |
| Rule: packages changed and their importers | HOLDS. `contract` is imported by `internal/contract` only (go list). `go test -count=1 ./contract/... ./lib/...` exit 0: contract ok, contract/formats ok, lib ok. `internal/contract` ran inside the `make cross` compile only; the lane changes no Go it reads. |
| Rule: repo-wide walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok. |
| Rule: no guard, gate, cap or hold on agents; docs and tests only; lib/lib.go read only | HOLDS. Nothing in the diff touches production code, schemas or contract/README.md. |
| Rule: *RealPlugin* tests | Not applicable: the lane adds none. |
| Rule: no personal paths, hosts or emails | HOLDS. The page uses `/home/example` and `example`. |
| Rule: no attribution lines in the commit | HOLDS: the commit message is one sentence, with no trailers. |

## What changed

- contract/docs/lib-call.md: the page with `## Call fields`, `## What a call reads and what stays with the host`, `## Exit codes` and `## Example`. Each statement cites `lib/lib.go:<line>`.
- contract/t12_lib_call_test.go: `TestT12LibFieldRows`, `TestT12LibExitCodes`, `TestT12LibExample`, `TestT12LibNegative`, `TestT12LibCitations`; every package-level name starts with `t12Lib`/`T12Lib`.

Notes on the page:
- The Args row's "empty means" cell says the comment does not state it; it is the empty command line (lib/lib.go:138).
- The Stderr row also names the `grid: internal error` line (lib/lib.go:136).
- The host items follow lib/lib.go:23-39 in four bullets.

## NOT VERIFIED on a live screen

Nothing here is a UI. Not run: the whole-module test and the cross-repo set (the orchestrator runs them at landing). The Windows build was compiled and vetted, not run. The claim that each cited line says what the page says is read by hand, not tested.
