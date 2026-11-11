# J-open-per-os-test: proof table

Code head: 4d573a05 (the report commit follows it).

| Row / rule | Result | Proof |
|---|---|---|
| 1. The pure func returns the exact argv for darwin, windows and linux, on any host | HOLDS | `internal/cli/open_os_test.go` TestOpenCommandPerOS; `go test ./internal/cli/ -run Open` exit 0. Func at `internal/cli/open.go:43`; `openBrowser` calls it at `open.go:63`. |
| 2. Windows: a target with a space and one with & stay one argv item after "url.dll,FileProtocolHandler"; no cmd.exe | HOLDS | TestOpenCommandWindowsKeepsTargetWhole (`open_os_test.go:51`), same command, exit 0. |
| 3. Linux: no display gives "no display"; xdg-open not found gives the lookPath error; WAYLAND_DISPLAY alone is enough | HOLDS | TestOpenCommandLinuxDisplayAndLookup (`open_os_test.go:71`), same command, exit 0. |
| 4a. `go test ./internal/cli/ -run Open` 0 fail | HOLDS | exit 0, all TestOpen* PASS, including the existing ones, which are unedited. |
| 4b. `GOOS=windows go vet ./internal/cli/` | HOLDS | exit 0. |
| New tests fail on base | HOLDS | With `open.go` from main: `go test ./internal/cli/ -run Open` gives `open_os_test.go:41:15: undefined: openCommand` (and :56, :74, :77, :82, :87) then `FAIL ... [build failed]`. They pass on the lane. |
| Packages changed and those that import them | HOLDS | `go list` importers of internal/cli: cmd/grid. `go test ./internal/cli/ ./cmd/grid/` exit 0 (cli ok 47.5s; cmd/grid has no test files). |
| Lint | HOLDS | `go vet ./...` exit 0; `gofmt -l internal/cli` prints nothing. |
| Windows and macOS builds | HOLDS | `GOOS=darwin go build ./...` exit 0; `GOOS=windows go build ./...` exit 0; `GOOS=windows go test -c -o /dev/null ./internal/cli/` exit 0. |
| No guard, gate, cap, hold or blocking hook | HOLDS | None added; the change factors an existing switch into a pure func. |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` before this report shows only open.go (+21/-11) and the new test file (+94); existing test files are untouched. |
| Public repo: no personal paths, hosts or emails | HOLDS | None in the diff. |
| Commits: one sentence, no attribution lines | HOLDS | One code commit, one sentence, no trailers. |

## What changed

- `internal/cli/open.go`: new `openCommand(goos, target, getenv, lookPath) ([]string, error)`; `openBrowser` calls it with `runtime.GOOS`, `os.Getenv` and `exec.LookPath`, then starts `argv[0]` with `argv[1:]`. Same argv and same errors as before.
- `internal/cli/open_os_test.go` (new): three tests that run the darwin, windows and linux branches on any host.

## NOT VERIFIED on a live screen

Whether a real browser opens on each OS. The tests check the argv only, not that `open`, `rundll32` or `xdg-open` launch.
