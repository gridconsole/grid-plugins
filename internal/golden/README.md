# Go golden replay

Run `make golden` from the repository root for a per-case table and totals.
`make test` runs the harness unit tests (including tiny pass and deliberate-diff
cases) and replays the corpus. `make golden` adds the verbose case report.
Neither path invokes Node, Python, the reference plugin or a shell command.

Each subtest uses `t.TempDir()`. `Replay` copies `fixtures/`, expands tokens,
applies `dirs`, `files` (null deletes) and `links`, then reads the exact `env.txt`
and `input.json`. It runs all supported `pre` commands, applies `after_pre`, takes
a raw snapshot, and invokes the main command. The CLI adapter calls `cli.Run`
with isolated stdin, stdout, stderr, environment lookup and cwd. The fixture root
is the resolver ceiling and temporary root; ancestor repositories and process
configuration do not leak into the result. Nothing changes the process cwd.
The CLI test adapter sets process TMPDIR to a separate `t.TempDir()` so commands
that use the operating system's temp location are cleaned up with the test;
`t.Setenv` restores it afterward. Paths are symlink-resolved before fixture writes.

The original corpus stores invocation identity in `setup.json` as `hook` and an
optional exported-function `call`, rather than a literal argv array. The explicit
`legacyArgv` table in `corpus_test.go` maps implemented replacements to Go argv.
The implemented SessionStart command replaces `grid-env` and `agent-routing`.
The remaining session scripts use component entry points in `internal/hook`,
and lib helpers call the existing shared Go functions through those entry points.
Component calls isolate effects; runtime events compose the same handlers.
Unsupported scripts are skipped with their names. Do not infer support from
exit zero: unknown hook events deliberately succeed silently.
When a replacement is implemented, add its mapping and review its cases. New
cases may instead record `argv`, excluding the `grid` executable. Explicit hook
argv also needs a recognized event. Unsupported setup commands skip the case
before any command runs. Setup and golden artifacts are validated even for skips.

`normalize.go` is the complete list of recorder normalization rules. Output JSON
is compared as bytes. `diff.go` mirrors Python's SequenceMatcher/unified_diff
format, including three context lines, popular repeated lines, mode/type changes,
symlink targets, absent final newlines, and changes only to normalized values.
IDs are shared across file diffs, sorted environment values and output streams
in the same order as the recorder. Empty directories and timestamp-only metadata
changes are excluded from snapshots.

A running case with an intended difference may be listed in
`golden/known-differences.txt` with a reason citing `docs/lanes/L<n>.md:<line>`.
A listed case that matches again fails, requiring removal of the obsolete entry.
These known differences are reported separately from unsupported-command skips.
To pin alternate byte expectations instead, use
`golden/<hook>/<case>/go-diff.md`, with a specific `doc.md:line` (or source-file:line)
citation and an explanation. Put all four
artifacts (`stdout`, `stderr`, `exit`, `files.diff`) in that case's `go-expected/`.
They replace the four legacy expectations together; missing files or citations
fail. There is no automatic acceptance or update mode for corpus expectations.
The original artifacts remain intact. The existing `Assert` helper still uses
`testdata/golden`; update those separate CLI snapshots with
`UPDATE_GOLDEN=1 go test ./...` when deliberately changing them.
