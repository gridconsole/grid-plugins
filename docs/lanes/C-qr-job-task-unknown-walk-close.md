# C-qr-job-task-unknown-walk-close

Base: 44c7de5bacbac31560de11e84afd4ff15980c05d (plugin main). On base,
`internal/cli/gg_unknown_task_walk_test.go:59` held `var ggTaskKnownOpen = map[string]ggTaskOpen{` with four
entries (job lint, render, show, verify) and `internal/cli/gg_unknown_project_walk_test.go:31` held
`var ggProjectKnownOpen = map[string]string{` with seven (task decline, task rename, job render, show,
verify, trace, lint). Neither was empty, so the lane went ahead.

## Proof table

| # | Row | Result | Command / place |
|---|-----|--------|-----------------|
| 1a | Red on base with both maps emptied | HOLDS | `go test -count=1 -v -run 'TestGgUnknownTaskWalk$\|TestGgUnknownProjectWalk$' ./internal/cli/` with the two maps emptied and the production edits stashed: FAIL. Lines: `task decline [...--project absent]: code "no_such_task"`, `task rename [...]: code "no_such_task"`, `job lint [...]: plain stdout is not empty`, `job render/show/verify [...]: --json changed exit or stderr: exit 1; stderr ""; plain exit 1, stderr "job: project \"absent\" is not registered"`, `job trace [...]: --json changed exit or stderr ... plain ... "job trace: project not found"`; task walk: `job lint/render/show/verify: --json changed exit or stderr ... stdout: {"ok":false,"findings":[],"error":"task T-99 not found in project example","code":"no_such_task"}`; `--- FAIL: TestGgUnknownProjectWalk`, `--- FAIL: TestGgUnknownTaskWalk` |
| 1b | Green on the lane, both maps empty | HOLDS | same command, exit 0: `--- PASS: TestGgUnknownTaskWalk`, `--- PASS: TestGgUnknownProjectWalk`, `known open: 0 of 80`, `known open: 0 of 85`. `var ggTaskKnownOpen = map[string]ggTaskOpen{}` (gg_unknown_task_walk_test.go:59), `var ggProjectKnownOpen = map[string]string{}` (gg_unknown_project_walk_test.go:31) |
| 2 | TestQr tests pass | HOLDS | `go test -count=1 -v -run 'TestQr' ./internal/cli/` exit 0, `--- PASS:` for TestQrJobVerbsUnknownTaskKeepObjectAndWriteStderr (show, render, verify), TestQrJobLintUnknownTaskAndProject (task, project), TestQrJobTraceUnknownProjectWritesStderr, TestQrTaskDeclineAndRenameUnknownProject (decline, rename), TestQrTaskDeclineKnownProjectUnknownTask. `job show T-99 J-1 --json` stdout is exactly `{"ok":false,"findings":[],"error":"task T-99 not found in project example","code":"no_such_task"}` (keys code, error, findings, ok), stderr `job: task T-99 not found in project example`, equal to the plain stderr. `task decline T-99 --project absent --reason x --json` answers `no_such_project` |
| 2b | New tests fail on base | HOLDS | production files checked out from 44c7de5b, `go test -count=1 -run TestQr ./internal/cli/`: FAIL, e.g. `qr_job_task_unknown_test.go:87: stderr with --json "", without "job: task T-99 not found in project example\n"`, `:114: code = "", want "no_such_project"` (job lint printed the lint report), `:142: stderr = "", want "job trace: project not found\n"`, `:161: code = "no_such_task", want no_such_project`. Files restored with `git checkout HEAD --`; `git status --short` clean |
| 3 | Coverage tests | HOLDS | `go test -count=1 -v -run 'TestGgUnknownTaskWalkCoversTheContract\|TestGgUnknownProjectWalkCoversTheContract' ./internal/cli/` exit 0, both `--- PASS:` (the walk's own check is unchanged: it still fails for a listed entry that answers well and for an unlisted one that does not) |
| 4a | Package tests | HOLDS | `go test -count=1 ./internal/cli/ ./internal/job/... ./internal/task/...` exit 0 (cli, job, job/frontmatter, job/jobcmd, job/lint, task all ok) |
| 4b | Importers of the changed packages | HOLDS | jobcmd is imported by internal/cli only; cli is imported by cmd/grid, contract/formats, internal/account, braincmd, golden, jobgraph, mcp, packaging, question, settings, lib. `go test -count=1` on those: all ok (run before the final narrowing commit; the narrowing only removes output) |
| 4c | Lint | HOLDS | `make lint` (go vet ./...) exit 0 |
| 4d | Windows and macOS | HOLDS | `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile) |
| 5 | Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestGg\|TestQr'` exit 0; `go test ./internal/job/lint/` ok. (App-side walks are the app repo's.) |
| 6 | No guard, gate, cap on agents | HOLDS | none added; the change prints an existing refusal on stderr and orders two lookups |
| 7 | `*RealPlugin*` tests | n/a | none named in the brief |

## What changed

- `internal/job/jobcmd/command.go`: `output` also prints `job: <error>` on stderr under `--json` for the
  `no_such_task` and `no_such_project` refusals (helper `stderrWithJSON`); `job lint --project <absent>` is
  refused with `no_such_project` before the file is read.
- `internal/job/jobcmd/trace.go`: `fail` prints `job trace: <error>` under `--json` for the same two codes.
- `internal/cli/task_write.go`: `task rename` and `task decline` resolve `--project` before the task operand,
  so an unregistered project answers `no_such_project` (`grid task decline: scope: project "absent" is not registered`).
- `internal/cli/gg_unknown_task_walk_test.go`: for the four job verbs (top-level `code`, `error` string) the check
  reads `error` as the message and accepts the `job: ` stderr; map emptied.
  `internal/cli/gg_unknown_project_walk_test.go`: map emptied.
- `internal/cli/qr_job_task_unknown_test.go`: new, `TestQr*`, helpers `qr*`.

## Differences from the brief (open items)

- The brief says "the stderr line also with --json when r.Error is set". Taken literally it breaks existing tests
  outside Touches that pin an empty stderr under `--json`: jobcmd `TestJobFlagAndCommandRefusals`,
  `TestResolverContextRequiredForEveryCommand`, `TestSearchErrorRendersWithoutRecall`, `TestTraceCommandNoHome`, and
  cli `TestGuestProjectParity` (job lint, guest, `guest_parity_test.go:461-463`). I did not edit those tests.
  Instead the stderr line under `--json` is limited to `no_such_task` and `no_such_project` and to a non-guest
  caller, which is what both walks read. Usage, member-context, search and other refusals, and every guest
  refusal, keep an empty stderr under `--json`. If the owner wants all refusals to print on stderr, that is a
  follow-up lane that also updates those five tests.
- `task rename` and `task decline` with a guest caller keep the old order (task first), as the job verbs do for a guest.

## NOT VERIFIED on a live screen

Nothing in this lane has a screen. The app reads job render/show stdout only (`agentbuild.go:241,273`,
`taskcode/build.go:92`); stdout is byte-identical to base for the unknown-task case, checked in TestQr.

Verdict: DONE
