# S9fixA2: global flags reach every reader

## What changed
- `internal/cli/mcp.go`: GRID_LEASE and GRID_LEASE_EPOCH are read with `env.getenv`, so `--lease-epoch` reaches the MCP writer.
- `internal/cli/feed.go`: GRID_FEED_TASK, GRID_JOB, GRID_WAKE and GRID_ATTEMPT are read with `env.getenv`. `feedDefaults` and `sessionDefaults` now take the Env.
- Tests: `TestMCPLeaseEpochFlagBeatsEnv` (mcp_test.go) fails on the S9fixA head (`--lease-epoch did not win over GRID_LEASE_EPOCH ... "stale lease"`) and passes now. `TestRoleAndEpochFlagsBeatEnv` (hold_test.go) shows `--role`, `--member` and `--lease-epoch` beat set env vars on `feed post`. It also passes on base, because `feed post` already read through `env.getenv`, so it adds coverage only.

## Remaining os.Getenv("GRID_ in internal/cli
Only test helper processes remain: GRID_SIGNAL_TEST_*, GRID_HISTORY_CLI_*, GRID_TEST_*. They carry fixtures into re-executed test binaries and are not product reads. No product code reads GRID_* with os.Getenv in internal/cli. `OSEnv` and `env.getenv` itself read the process environment, as designed.

## Checks
- go test ./internal/... ./cmd/... : exit 0 (importers of internal/cli: internal/cli, cmd/grid)
- gofmt -l internal cmd : no output
- go vet ./internal/cli ./cmd/... : exit 0
- GOOS=windows and GOOS=darwin CGO_ENABLED=0 go build ./cmd/... ./internal/... : exit 0
- make lint, contract/commands.json and goldens: left to the orchestrator.

## NOT VERIFIED
- make lint.
- The whole-module test and the cross-repo set.
