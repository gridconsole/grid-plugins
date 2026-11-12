# C-la-host-drop-fix1: drop the heMvProse entries for the removed host guard strings

Verdict: DONE. Test-only change in `internal/contract/he_mv_message_verbs_test.go`; nothing else touched.

## Proof table

| Row | Result |
|---|---|
| Merge `origin/main` | HOLDS. `git merge origin/main` made a clean merge (ort), no conflicts, no file resolved by hand. |
| Base failure | HOLDS. Before the edit, `go test -count=1 ./internal/contract/ -run TestHeMvProseListBothWays` exited 1: `he_mv_message_verbs_test.go:330: heMvProse entry "internal/guard/host.go:feed or" matches no string literal; remove it`, same for `"...host.go:question command"` and `"...host.go:task or"`. Only these three entries were named. |
| 1. TestHeMvProseListBothWays PASS | HOLDS. `go test -count=1 ./internal/contract/ -run TestHeMvProseListBothWays -v`, exit 0. Last lines: `--- PASS: TestHeMvProseListBothWays (0.10s)` / `PASS` / `ok  github.com/gridconsole/grid-plugins/internal/contract 0.107s` |
| 2. Package tests | HOLDS. `go test -count=1 ./internal/contract/ ./internal/hook/ ./internal/guard/... ./internal/cli`, exit 0: contract ok 1.950s, hook ok 9.481s, guard ok 0.911s, cli ok 64.540s. |
| 3. `make lint` | HOLDS. exit 0 (`go vet ./...`, no output). |
| 3. `make cross` | HOLDS. exit 0. darwin/arm64 and windows/amd64 vet and test compile, every package `ok`. |
| Importers of changed package | HOLDS. `internal/contract` is imported by `contract`, `contract/formats` and `internal/cli`; the change is a test file, so no importer sees it. `go test -count=1 ./contract/ ./contract/formats/` exit 0 (ok 0.080s, ok 0.077s); `internal/cli` is in row 2. |
| Walk tests | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0 (ok 0.038s). `go test -count=1 ./internal/job/lint/` exit 0 (ok 0.102s). |
| New tests fail on base, pass on lane | Not applicable: no test added. The existing test TestHeMvProseListBothWays is the one that failed on base (see Base failure) and passes now. |
| Deleted test | None deleted. Three map entries removed from `heMvProse`; the strings they excused were removed with the host guard by the C-la-host-drop lane (simplify.md, the la chain), per the brief. |
| No guard, gate, cap, hold or blocking hook added | HOLDS. Diff touches one test map only. |
| Clean tree | HOLDS. `git status --short` empty after the commit. |
| Real-plugin tests | Not applicable: no *RealPlugin* test in scope. |
| UI rows | Not applicable. |

## What changed

`internal/contract/he_mv_message_verbs_test.go`: removed the three `heMvProse` entries `internal/guard/host.go:task or`, `...:question command`, `...:feed or`. Also removed the one comment line `// Plain English after a verb name.` above them, since it headed only those entries and would have been left dangling above the closing brace. No other line of the test changed. `gofmt -l internal/contract/` prints nothing.

Commit: `Drop the heMvProse entries for the removed host guard strings.`

## NOT VERIFIED on a live screen

Nothing: no UI or live behaviour is involved.
