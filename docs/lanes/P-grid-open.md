# P-grid-open: `grid open`

Plugin repo, base main. Builds row PL (P4-grid-open) of P0-lib-map.

## What changed
- `internal/cli/open.go` (new): `grid open [--json] [T-NN | ULID | task folder]`.
  - Finds the task as `grid task show` does (T-NN from anywhere in the home, a task folder, or the enclosing task when no operand is given). A ULID is looked up as `grid where --task` does.
  - Opens `<app URL>/#/t/<project>/<id>/overview` in the browser. `<id>` is the task's ULID (from task.md, both migrated and legacy shapes); only a task with no ULID yet links by T-NN. T-NN is display only.
  - No browser (no display, no `xdg-open`, or the opener fails): prints the URL on stdout, a note on stderr, exit 0.
  - `--json` prints `{"url": ...}`.
- `internal/cli/cli.go`: `open` in the command table; `Env.OpenURL` test seam (nil opens the system browser: `xdg-open`, `open`, `rundll32`).
- `contract/commands.json`: `open` entry (hand-edited, see below).
- Tests: `internal/cli/open_test.go` (new). Two existing test tables got one new row each, no existing row changed: `contract_inventory_test.go` (annotation for `open`) and `guest_parity_test.go` (`open` is a read command, no flags).

## Decisions (map Q4 and what the map left open)
- Q4, the map's recommendation: the app writes its base URL into the Grid home and `grid open` reads it. File: `<grid home>/app-url`, first line, an http(s) address. `GRID_APP_URL` wins over the file. This lane only reads; **the app side (writing `app-url` at start) is not built here** and is open for the app lanes.
- No app address known: prints the path `#/t/<project>/<id>/overview` on stdout and says on stderr to start the app; exit 0 and nothing is opened. With `--json` the `url` is that path.
- A bad `app-url` (not http/https) counts as no address.
- Q5 (no Go library) holds: nothing imports across repos.
- No guard, gate or cap added; `open` only reads.

## Proving tests (fail on base, pass on the lane)
On base, with `open` out of the command table: `go test ./internal/cli -run TestOpen` fails:
```
open_test.go:51: open T-01: 2 grid: unknown command "open". Run: grid help   (migrated, legacy)
open_test.go:77: open: 2 grid: unknown command "open". Run: grid help
open_test.go:90: open: 2 grid: unknown command "open". Run: grid help
open_test.go:105: open: 2 grid: unknown command "open". Run: grid help
```
- Row 1 (resolve and open): TestOpenOpensTaskPageInBrowser (migrated and legacy; by T-NN, by folder, by ULID).
- Row 2 (no browser prints the URL): TestOpenWithNoBrowserPrintsURL; no address: TestOpenWithoutAppAddressPrintsPath.
- Row 3 (`--json`): TestOpenJSONPrintsURL.
- TestOpenUnknownTaskFails: unknown task exits non-zero and opens nothing.

## Checks
| Check | Exit |
|---|---|
| `go test ./internal/cli ./cmd/grid/... -count=1` (importers of internal/cli by go list: cmd/grid only) | 0 |
| `go test ./contract/...` | 0 |
| `make lint` (go vet, gofmt) | 0 |
| `GOOS=windows CGO_ENABLED=0 go build ./...` | 0 |
| `GOOS=darwin CGO_ENABLED=0 go build ./...` | 0 |
| webui-test, ops-check | not run: no web UI and no api/ops.yaml change |

Golden note: env-prefixed `UPDATE_CONTRACT_COMMANDS=1 go test ...` was refused, so `contract/commands.json` was edited by hand; `TestContractCommandsParserInventory` passes against it. No other golden changed.

## NOT VERIFIED
- A real browser opening on Linux desktop, macOS or Windows: only the seam is tested; the `xdg-open` / `open` / `rundll32` calls ran in no test.
- The app side of Q4 (writing `app-url`) does not exist yet, so `grid open` against a running app was not tried.
- Whether the app's route accepts the ULID for a home not yet migrated: legacy homes link by the ULID in task.md.
