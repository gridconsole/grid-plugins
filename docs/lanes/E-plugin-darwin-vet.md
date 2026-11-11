# E-plugin-darwin-vet: `make cross` vets and compiles the tests for darwin and windows

Base: main 38a3d1b2. Branch lane-E-plugin-darwin-vet. Three commits: the target, a comment wording fix, this file. `make cross` and `make lint` were rerun on the final Makefile (exit 0 both); the other checks ran before the comment fix, which changed no command.

## Proof table

| Row | Status | Evidence |
|---|---|---|
| 1. `make cross` runs `go vet ./...` and `go test -run XXX_NONE -count=1 -exec /bin/true ./...` with GOOS=darwin GOARCH=arm64 and GOOS=windows GOARCH=amd64, CGO off | HOLDS | Makefile:`cross` (a loop over `darwin/arm64 windows/amd64`; each pass sets `CGO_ENABLED=0 GOOS GOARCH`, runs the vet, then the compile-only test; `|| exit 1` stops at the first failure). The log shows `cross: darwin/arm64 vet`, `cross: darwin/arm64 test compile`, `cross: windows/amd64 vet`, `cross: windows/amd64 test compile`, and 112 `ok` lines (56 packages, twice). `.PHONY` lists `cross`. |
| 1b. It sees the per-OS files named in the brief | HOLDS | `GOOS=windows go list` shows `internal/workflow` with `open_windows.go` and `internal/eval` with `process_other.go`. `GOOS=darwin go list` shows `open_unix.go` and `process_unix.go`, and `internal/routine` with `native_darwin.go`. |
| 1c. The target fails when a per-OS file breaks (the new check fails on a break, passes without one) | HOLDS | A temporary `internal/workflow/zz_probe_darwin.go` with `var _ = onlyOnDarwinUndefined`: `make cross` printed `internal/workflow/zz_probe_darwin.go:3:9: undefined: onlyOnDarwinUndefined` (vet and the test build), then `make: *** [Makefile:34: cross] Error 1`. `go vet ./internal/workflow` on linux with the same file present exit 0, so only the cross target sees it. The probe was removed (`git clean -f`); it is not in the diff. |
| 1d. On base the target does not exist | HOLDS | `make -f <base Makefile> cross` printed `make: *** No rule to make target 'cross'.  Stop.`, exit 2. |
| 2. Every error it finds is fixed with the right build tag or a per-OS file; each fix listed | HOLDS, with no fixes | `make cross` found no error on base: vet and the test compile are clean for darwin/arm64 and windows/amd64 across all 56 packages. So there is no build-tag fix to list. The 55 files that carry a `//go:build` line (and the `_darwin`, `_windows` named files) already compile for both OSes. |
| 3. `make cross` exits 0 on the VM | HOLDS | `make cross > log 2>&1 && echo ok`: printed `make cross exit 0`. `grep -E 'FAIL|error|cannot|undefined|vet:'` on the log prints nothing. |
| 4. `go test ./...` (linux) and `make lint`: 0 fail | HOLDS | `go test ./...`: exit 0, 64 lines, all `ok` or `no test files`. `make lint`: exit 0 (go vet and gofmt -l clean). |
| Packages changed and their importers | HOLDS | No Go package changed (Makefile and this doc only), so no importer set. `go test ./...` above covers every package. |
| Windows build | HOLDS | `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build ./...` exit 0 |
| macOS build | HOLDS | `CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...` exit 0 |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat`: Makefile and this doc only. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | The change is a build-time make target; nothing runs on an agent. |
| No attribution lines, no personal paths, hosts or emails | HOLDS | Commit messages are one sentence with no trailers. The Makefile comment and this file name no person, host or home path. |
| internal/packaging not touched | HOLDS | `git diff origin/main..HEAD --stat` lists no file under internal/packaging. |
| Nothing pushed | HOLDS | Bundle only. |

## What changed

- `Makefile`: new `cross` target (and `.PHONY` entry). It loops over `darwin/arm64` and `windows/amd64`; for each it runs `go vet ./...` and then `go test -run XXX_NONE -count=1 -exec /bin/true ./...`, both with `CGO_ENABLED=0`. `-exec /bin/true` compiles each test binary and does not run it, since the binary is for another OS. It is not part of `make lint` or `make test`, so those stay as fast as before.
- `docs/lanes/E-plugin-darwin-vet.md`: this file.

## Notes

- README.md:10-14 lists the make targets and does not list `cross`. The brief allows only the Makefile, so the README is left for the next lane that edits it.
- The vet uses CGO off, as `make dist` does. A file that needs cgo is not seen by it.
- `go test -exec /bin/true` also runs the vet subset that `go test` does, so the test files get a second check per OS.

## NOT VERIFIED on a live screen

- Nothing here is a UI row. The darwin and windows results are a compile and vet from the VM; no darwin or windows binary was run, and no test was run on those OSes.
