# C-jm-lib-grid-binary-once

| Row / rule | Result |
|---|---|
| 1. `go test -count=1 -v -run TestJmGridBinaryBuiltOnce ./lib/` | HOLDS, exit 0: `--- PASS: TestJmGridBinaryBuiltOnce (2.12s)` |
| 2. `go test -count=1 ./lib/` | HOLDS, exit 0: `ok .../lib 22.441s`, no `jm: grid built` line (TestMain, lib/jm_grid_binary_once_test.go:21-30, fails with exit 1 if count > 1) |
| 3. `go test -list '.*' ./lib/ \| grep -c '^Test'` | HOLDS: base 31, lane 32 = base + 1 (the brief's 30 counted without one file; `git grep -c '^func Test' main -- lib` sums to 31) |
| 4. diff of lib/lib_test.go | HOLDS: `t.TempDir()` gone, build line unchanged at lib/lib_test.go:84, Dir `..`; `git grep -n '"go", "build"' -- lib` prints that one line |
| 5. diff stat | HOLDS: only lib/lib_test.go and lib/jm_grid_binary_once_test.go (plus this report) |
| 5. `make lint` | HOLDS, exit 0 (`go vet ./...`, no output) |
| 5. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64 vet and test compile, lib ok) |
| New test fails on base | The test file does not exist on base: `no tests to run`; base list is 31 tests without it |
| Packages importing lib | none: `go list -f '{{.ImportPath}} {{.Imports}}' ./...` shows only lib itself; lib tests run above |
| No guard/gate/cap on agents; no -p or GOMAXPROCS; nothing removed; no production file | HOLDS |

Timing figure (not a row): `ok .../lib` 81.561s on base, 22.441s on lane (same machine, one run each, load not controlled).

## What changed
- lib/lib_test.go: buildBinary body runs the unchanged `go build` once under sync.Once into a folder owned by TestMain; the error is kept and each caller fatals with `go build ./cmd/grid: %v\n%s`-equivalent text.
- lib/jm_grid_binary_once_test.go: TestMain (MkdirTemp, m.Run, `jm: grid built <n> times` + exit 1 above 1, RemoveAll, exit with m.Run's code) and TestJmGridBinaryBuiltOnce.

## NOT VERIFIED
No live screen involved. The `jm: grid built` failure path was not exercised (it only fires if the count exceeds 1, which sync.Once prevents).
