# C-gr-brain-unknown-project-envelope-narrow

Base: d1c76719044999b8cb0bbff892c105e21801676b (plugin main, contains base-lane 5a6d9af9). On the base the eleven brain entries are in `ggProjectKnownOpen` (accept, changes, check, index, keep, propose, revert, search, show, supersede, topics) and the workflow entries are gone. Brain records no `no_such_project` code on the base: `runBrain` built `braincmd.Env` with no recorder (internal/cli/brain.go).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. `brain show/revert/propose --project absent --json`: stdout `{"error":{"code":"no_such_project",...}}`, stderr the plain line, exit 1 | HOLDS | `go test -count=1 -run TestGrBrainEnvelope -v ./internal/cli/` exit 0; `TestGrBrainEnvelopeUnknownProject/{show,revert,propose,search,topics,changes}` PASS via `assertErrorOutput` (internal/cli/gr_brain_envelope_test.go:64, helper :40-52); plain and `--json` stderr compared equal (grBrainEnvelope, :40-52) |
| 2. Guest asking for a project not granted gets the envelope of an absent one | HOLDS | `TestGrBrainEnvelopeGuestHiddenProject` PASS (gr_brain_envelope_test.go:79): guest `gus` granted only `alpha`; `beta` (registered, not granted) and `absent` both give code `no_such_project`, same stderr shape (only the asked name differs). The guest branch returns `where.ErrProjectUnregistered` or `projectUnavailable` (internal/where/where.go:125, personal.go:97), both carry the sentinel. `revert` and `changes` refuse a guest's role first (exit 3, `caller role is not allowed...`, braincmd/changes.go role check), identically for hidden and absent; the test asserts that |
| 3. Brain failure with no where sentinel keeps its JSON object on stderr, no stdout envelope | HOLDS (example changed) | `TestGrBrainEnvelopeOtherFailureKeepsJSONObject` PASS: revert no-such-version, show no-such-note, accept no-such-proposal give exit 1, empty stdout, `{"error":"..."}` on stderr. The brief's example "`brain revert` with a bad id" is a usage error on base and lane (exit 2, plain line `brain revert:expected <topic>/<name>...`, braincmd/changes.go:227-232), never the JSON path, so it is not used |
| 4. `TestGgUnknownProject` PASS and `ggProjectKnownOpen` holds no `brain` key | HOLDS | `go test -count=1 -v -run TestGgUnknownProject ./internal/cli/` exit 0; log `known open: 8 of 82`; `grep -c '"brain ' internal/cli/gg_unknown_project_walk_test.go` = 0 (13 lines removed: eleven entries, their comment, one blank) |
| 5. `go test -count=1 ./internal/braincmd/ ./internal/cli/` exit 0, eight named tests unedited | HOLDS | exit 0 (`ok internal/braincmd 2.004s`, `ok internal/cli 65.571s`); `git diff d1c76719 -- 'internal/braincmd/*_test.go' \| wc -l` = 0. All eight are in braincmd and run with no RecordError, so their answers are unchanged |
| 6. `make lint` exit 0; Windows build exit 0; diff names only Touches files | HOLDS | `make lint` (`go vet ./...`) exit 0; `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `git diff d1c76719 --stat`: braincmd/changes.go, braincmd/command.go, cli/brain.go, cli/gg_unknown_project_walk_test.go, cli/gr_brain_envelope_test.go |

## Rules

| Rule | Result | Proof |
|---|---|---|
| New tests fail on base, pass on lane | HOLDS | With the three production files checked out from d1c76719 the run printed `--- FAIL: TestGrBrainEnvelopeUnknownProject/show` ... `gr_brain_envelope_test.go:68: --json [brain show note --project absent]: exit=1 stderr="{\"error\":\"project \\\"absent\\\" is not registered\"}\n", want exit 1, stderr "brain: project \"absent\" is not registered\n"` (same for revert, propose, search), and `TestGgUnknownProjectWalk` failed with `brain show [...]: --json changed exit or stderr` for all eleven (the entries were removed). Files restored with `git checkout HEAD --`, `git status --short` empty |
| No existing test edited to pass | HOLDS | only `ggProjectKnownOpen` entries removed, as the brief directs; no test deleted |
| Tests of changed packages and importers | HOLDS | `go list` importers of braincmd/cli: internal/cli, cmd/grid, lib. `go test -count=1 ./internal/braincmd/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no test files) |
| No guard, gate, cap or hold on agents | HOLDS | the change only chooses plain line or JSON object for a failure that is already a failure |
| No attribution lines, no personal paths | HOLDS | commit messages are one sentence; no host, path or email in the diff |
| UI rows | n/a | no UI |
| RealPlugin tests | n/a | none named |

## What changed

- `braincmd.Env.RecordError func(error)` (command.go). `runBrain` sets it to the CLI's `env.recordError` (cli/brain.go), the recorder `commandError` uses.
- `failWith` (changes.go) is now the one failure printer; the `fail` closure in `Run` calls it. For an error that wraps `where.ErrProjectUnregistered` or `where.ErrTaskNotFound` it calls `RecordError` and prints the plain line even under `--json`; the CLI's `commandFailure.finish` then writes the standard envelope on stdout. Every other error keeps the JSON object on stderr. With `RecordError` nil (braincmd used alone) nothing changes.
- `hasCode` mirrors `errorCode` for the two where sentinels. `where.ErrPersonalRefused` has no code in `errorCode`, so it keeps the JSON object.
- walk test: the eleven brain entries, the comment that described them, and one blank line removed.

## Open items for the reviewer

- `hasCode` repeats two lines of `errorCode` (cli/error_codes.go:36-39) because braincmd cannot import cli. If `errorCode` gains a code for another where sentinel, `hasCode` needs the same line. Alternative: change `RecordError` to return whether it recorded a code; that departs from the brief's design.
- Task not found inside brain (for example `brain show x --task T-99`) goes through `TaskOperandError`, unchanged. A `Resolve` failure with `ErrTaskNotFound` through `fail` now also gets the plain line and `no_such_task`, as `errorCode` defines.

## NOT VERIFIED on a live screen

Nothing on a live screen; the app's reading of the envelope was not run (no live app in this sandbox).
