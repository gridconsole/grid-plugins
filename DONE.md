# Lane L42 review follow-ups

Checked the reported locations with `grep -n`, including the original revision
where necessary. Items 1, 2, 3 and 5 still applied and are fixed. Item 4 has no
implementation in this checkout; it is unavailable, not already fixed.

## Changes and evidence

1. Merge leases now write the project, generation, fixed-millisecond UTC times
   and renewal time using `contract.MergeLease`. The CLI passes the resolved
   project (`internal/cli/liveness.go:55`); the writer uses the contract shape
   (`internal/lease/lease.go:225`). Fence allocation refuses values beyond the
   contract's signed integer representation. Take, renew and release files are
   validated through `contract.Validate` in
   `internal/lease/lease_test.go:322`; the CLI-written file is validated in
   `internal/cli/liveness_test.go:177`. The schema was not edited.
2. Stored session records use `contract.FormatTime`, including optional end and
   sleep times (`internal/session/session.go:450`). Intent requests, claims and
   completions use it at both persistence paths
   (`internal/intent/files.go:114`, `internal/intent/files.go:137`,
   `internal/intent/files.go:207`). Artifact creation uses it at
   `internal/artifact/artifact.go:88`. Tests cover zero fractions, trailing zeros,
   sub-millisecond input and non-UTC input. The requested wider writer scan also
   found and fixed task creation/history, brain notes, signal notes and migration
   lock timestamps (`internal/task/create.go:94`, `internal/task/write.go:150`,
   `internal/brain/store.go:42`, `internal/signal/signal.go:234`,
   `internal/signal/signal.go:289`, `internal/migrate/recover.go:65`). Task and Job
   input goldens reflect the new bytes and resulting source hash. Parsing remains
   lenient. Human-readable CLI formatting is unchanged. The separate session
   heartbeat writer was already fixed at `internal/session/session.go:82`.
3. Eval cleanup is deferred immediately on entering the workspace owner
   (`internal/eval/runner.go:103`, `internal/eval/runner.go:109`). Early errors and
   panics remove the root; a panic still propagates and does not save an
   incomplete result. Cleanup failures remain visible in saved normal results.
   The existing chmod retry remains. Regression tests are at
   `internal/eval/runner_test.go:406` and `internal/eval/runner_test.go:423`.
4. There is no MCP `feed_types` handler, associated test or golden to update.
   `README.md:37` explicitly lists `grid mcp` as unfinished, and
   `internal/cli/cli.go:91` registers it with a nil handler. The references to
   `feed_types` at `docs/old-plugin-map.md:53` and `docs/old-plugin-map.md:55`
   describe the old JavaScript plugin. A compatible compact/default versus
   full/type-specific response needs the missing MCP implementation first.
5. Dev reservations record `pid_start` beside the server pid
   (`internal/ports/ports.go:52`). Shutdown, release and GC check that identity;
   each shutdown signal rechecks it (`internal/ports/process_unix.go:37`). The
   shared process identity helper includes the Linux boot ID and uses a bounded,
   locale-stable `ps` query on macOS (`internal/processid/start_unix.go:19`). The
   existing migration helper reuses it while retaining its previous lock token
   format. Child commands receive an explicit environment allowlist
   (`internal/env/env.go:152`). Health polling occurs after the startup lock is
   released; a fresh lock confirms the exact reservation and process identity
   before success or cleanup (`internal/env/env.go:58`). Tests exercise reused
   pids, legacy records without identity, down/GC, real child environments, and
   reservation replacement during both first and repeated up calls
   (`internal/ports/ports_test.go:361`, `internal/env/env_test.go:263`,
   `internal/env/env_test.go:362`, `internal/env/env_test.go:401`).

## Validation

Run from the repository root, without overriding TMPDIR:

```sh
make test
make lint
go test -race ./internal/artifact ./internal/brain ./internal/cli ./internal/env ./internal/eval ./internal/intent ./internal/lease ./internal/migrate ./internal/ports ./internal/processid ./internal/session ./internal/signal ./internal/task ./internal/job
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

`make test`, `make lint`, all listed race tests, and both cross-platform builds
passed on Linux. The default-temp-directory leak check reported no new matching
entries. Acceptance also exposed an existing CLI fixture that checked only one
port; it now checks the complete ten-port block before starting the test server
(`internal/cli/environment_test.go:76`).

## Intended differences from the old plugin

References beginning with `docs/` below refer to the supplied read-only spec;
`docs/lanes/` and `docs/old-plugin-map.md` refer to this repository.

- Merge coordination remains the new project-wide fifteen-minute lease, with
  project identity now actually persisted: `docs/feed-contract.md:115` and
  `docs/lanes/L7.md:55`. This change does not alter grant/refusal text or the
  accepted holder inputs.
- Stored timestamps consistently use UTC with three fractional digits, following
  the contract convention at `docs/feed-contract.md:52` and
  `internal/contract/types.go:219`. This intentionally changes timestamp bytes
  and hashes derived from newly written files; it does not rewrite old records.
- Eval workspace cleanup now covers errors and panics as well as normal runs,
  applying the throwaway-home requirement at `docs/plugin-design.md:378` and
  the existing Go harness difference described in `docs/lanes/L11.md:33`.
- Dev processes are identified by start time as well as pid, following
  `docs/feed-contract.md:101`. API keys and arbitrary parent variables are not
  inherited; project secrets remain explicit local configuration as described
  at `docs/go-architecture.md:382`. Port operations can proceed while health is
  pending, with ownership confirmed afterward. These close the three reported
  L30 gaps at `docs/lanes/L30.md:160`, `docs/lanes/L30.md:162` and
  `docs/lanes/L30.md:163`, while retaining the dev-server behavior specified at
  `docs/go-architecture.md:246`. The old environment hook only discovered and
  exported Grid directories (`docs/old-plugin-map.md:31`).
- No MCP behavior changed because the new server is absent; the old tool mapping
  remains documented at `docs/old-plugin-map.md:55`.

## Limitations

The MCP follow-up remains unimplemented for the reason above. Native macOS and
Windows execution was not available here; cross-compilation checks the builds,
not runtime behavior. Windows dev-process supervision remains unsupported.
Existing dev records without a start token, or live groups whose leader cannot
be identified, fail explicitly instead of signalling an unverified process.
macOS process tokens use the existing second-resolution `ps lstart` mechanism.
Shutdown grace periods still hold the allocator lock; health polling does not.
Existing short merge-holder inputs remain accepted; schema-validation tests use
contract-form holder IDs.
