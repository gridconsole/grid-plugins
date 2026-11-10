# PLUG-kind: the feed state Job carries its kind

## What changed
- `internal/contract/types.go`: `Job` has an optional `Kind` (`json:"kind,omitempty"`).
- `internal/contract/fold.go`: the fold reads `kind` from the `job` post body, next to title and role, and sets it on the Job. A post with no kind leaves it empty, and it is left out of the JSON.
- `contract/schemas/cmd-feed-state.json`: each `jobs` item has an optional `kind` with the same rule as `job-record.json` `kind` (the job post body): string, `^[a-z][a-z-]*$`, length 1 to 40. It is not in `required`, so old output without kind still validates (`valid/basic.json` and `valid/populated.json` still pass).
- New contract fixtures: `contract/fixtures/cmd-feed-state/valid/kind.json` (two Jobs, kinds `code` and `user-docs`) and `invalid/kind-pattern.json` (kind `Code Change`).
- New tests: `internal/cli/feed_state_kind_test.go` `TestFeedStateCarriesJobKind` posts J-1 (`code`), J-2 (`user-docs`) and J-3 (no kind) with `grid job post`, then reads `grid feed state --json`. It checks the output against `cmd-feed-state`, checks both kinds, and checks that J-3 has no kind. `internal/contract/kind_test.go` `TestStateJobKindLoads` loads old state (`valid/populated.json`, no kind) and state with kinds into `contract.State`.

## Goldens and generated files to regenerate (row 3)
None. `contract/commands.json` names the output schema only (`"output": "cmd-feed-state.json"`) and does not include its body. `contract/fixtures/cmd-feed-state/valid/{basic,populated}.json` (written by `UPDATE_CONTRACT_COMMAND_FIXTURES=1`) do not change: the Job they post (`job-record/valid/basic.json`) has no kind, and kind is omitempty. `testdata/golden/mcp-session.golden` holds the Feed type schemas, not `cmd-*` schemas. `api/ops.yaml` and `routes.gen.mjs` hold no copy of the feed state Job shape (a grep for `needs_lead|lease_expires` finds only the schema, its fixtures, types.go and docs/old-plugin-map.md).

## What the app can drop (row 4)
The app no longer needs to read `job` posts to find a Job's kind. The brief says the app reads the item kind from the job post: the Feed `job` record's `body.kind`, which may mean following `body.ref` to `bodies/<ULID>.json` when the body is stored separately. After this lands it can read `jobs[<id>].kind` from `grid feed state --json` instead. State is folded from the Feed on every call, so Jobs posted before this change get their kind too. A missing `kind` means the job post had none. Keep the old read only while the app supports plugin builds older than this lane. I did not read the app code, so this names the read path, not a file and line.

## Failing on base (new tests)
With `fold.go`, `types.go` and `cmd-feed-state.json` stashed:
```
internal/contract/kind_test.go:15:41: j.Kind undefined (type *Job has no field or method Kind)
FAIL	github.com/gridconsole/grid-plugins/internal/contract [build failed]
--- FAIL: TestFeedStateCarriesJobKind (0.02s)
    feed_state_kind_test.go:75: J-1 kind = <nil>, want "code"
    feed_state_kind_test.go:75: J-2 kind = <nil>, want "user-docs"
```
With `kind_test.go` also stashed (so the contract package builds):
```
--- FAIL: TestFixtures/cmd-feed-state/valid/kind.json (0.00s)
    schema_test.go:37: Validate: $.jobs.J-1: anyOf matched 0 branches: $.jobs.J-1: expected null; fixture is valid
```
`invalid/kind-pattern.json` is also rejected on base (because of additionalProperties false). It guards the pattern, not the new field. All of these pass on the lane.

## Checks
- `go test ./... -count=1`: exit 0. Every package is ok. This covers `internal/contract` and every importer from `go list`: artifact, brain, braincmd, cli, feed, guard, hook, intent, job, job/jobcmd, job/jobtest, jobgraph, land, lease, mcp, orchestrator, ports, question, session, signal, task, usage, where, workflow, plus cmd/grid.
- `go vet ./internal/contract ./internal/cli`: exit 0. `gofmt -l internal/contract internal/cli`: exit 0, no files listed. `make lint` was not run (the brief says not to).
- `GOOS=windows go build ./...`: exit 0. `GOOS=darwin go build ./...`: exit 0.

## NOT VERIFIED
- The app side (the read path above) is not changed or checked here. There is no UI change.
- A `job` post whose body is stored separately (`{"ref":...}`) gets its kind the same way the fold gets its title and role. This lane did not add a test for it.
