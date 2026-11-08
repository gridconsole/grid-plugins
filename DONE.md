# L40 completed: terminal task attach

Built `grid task attach T-NN [--session ID] [--runtime claude] [--json]` on the current branch.

## A: terminal attach

The command resolves IDs exactly as task show does, records/resumes a terminal session with the parent process identity and local machine ID, preserves Started on resume, and prints task ID, title, state, plan approval, open questions and `Attached. T-NN <title>`. Another holder produces text naming the holder and `grid lease take-over`; JSON remains on stdout with advice on stderr. No lease or existing heartbeat changes. Reusing a Job session key is refused to preserve its tenure.

Each successful attach appends one history receipt through the existing task writer without rewriting task.md or plan.md. Concurrent process tests check every receipt and unchanged task bytes. Agent callers cannot attach outside their assigned task. Direct input/attachment writes remain blocked by the unchanged guard rules.

## B: file and note attach

Not built: the final scope correction explicitly excludes `--file` and `--note`. Both are usage errors. Lease take-over, runtime launching and session hook integration are also outside this lane.

## Defaults chosen (default, owner may change)

- Closed tasks refuse attach and name `grid task reopen`.
- Session ID: nonempty `--session`, then `GRID_SESSION_ID`, then `CLAUDE_SESSION_ID`, then an existing-generator ULID.
- Runtime: `claude`, matching the session store's supported runtime.
- No defaults selected for the excluded file/note features.

## Tests

All required checks passed on Linux with the default TMPDIR:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
go test -race ./internal/task ./internal/cli
```

The temporary directory leak check passed. macOS and Windows are compile-checked, not runtime-tested here. Focused tests:

```sh
go test ./internal/task -run 'TestAttach|TestConcurrentAttach'
go test ./internal/cli -run TestTaskAttach
go test ./internal/session -run TestTerminalResumeCannotDropJobTenure
go test ./internal/guard -run TestOnlyWriterBlocksTaskInputsAndAttachments
```

## Intended differences from the old plugin

Reference paths below are relative to the supplied read-only reference tree, except the checkout's `docs/old-plugin-map.md`.

- Explicit ID-based resume, without prompt inference, automatic task creation or hook installation: `prototype/audit/spec-plugin.md:42`, `:52`; `docs/old-plugin-map.md:103`; `old-plugin/hooks/session-attach.mjs:9`; `old-plugin/hooks/terminal-task.mjs:72`.
- Home-wide task.Show resolution, including Personal and other projects; no slug or prompt operand: `old-plugin/hooks/lib.mjs:492`, `:513`; `old-plugin/hooks/terminal-task.mjs:50`.
- Explicit unknown/closed/invalid refusals, reopen guidance and agent assignment checks instead of silently ignoring a hook event: `old-plugin/hooks/session-attach.mjs:4`, `:9`; `old-plugin/hooks/terminal-task.mjs:50`; `old-plugin/hooks/lib.mjs:513`.
- Machine-scoped Go session records, Claude runtime, parent PID/start identity, explicit/environment/generated IDs and safe-component validation instead of ID sanitization: `old-plugin/hooks/lib.mjs:450`, `:457`, `:459`; `docs/old-plugin-map.md:75`.
- Every explicit attach refreshes the session and appends history, without the ten-second throttle. Started and transcript survive resume; liveness starts unknown and no hook transcript is imported: `old-plugin/hooks/lib.mjs:457`, `:471`, `:472`; `docs/old-plugin-map.md:34`.
- No legacy pointer or `.attached` list and no parent-shell environment update; the task session record is the association: `old-plugin/hooks/lib.mjs:539`; `docs/old-plugin-map.md:74`.
- Task facts and holder advice are printed, with no lease takeover; existing Job keys cannot become terminal keys: `prototype/audit/spec-plugin.md:52`; `old-plugin/hooks/session-attach.mjs:9`.
- No runtime launch or resume prompt: the old launch configuration remains a separate responsibility, `docs/old-plugin-map.md:61`.

## Implementation notes and limits

[Lane documentation](docs/lanes/L40.md) explains the command, defaults and coverage. Supporting changes allow history-only journals, refuse Job-to-terminal conversion under the existing session lock, and expose the existing platform process identity helper through `internal/migrate.ProcessStart`. No new lock, atomic writer, machine ID helper, front-matter helper or dependency was introduced.

Session publication and history are separate existing-store writes. A crash or storage failure between session publication and history commit can leave a session without that invocation's receipt; committed history journals recover idempotently. No macOS/Windows runtime was available for execution. Everything in the corrected terminal-attach scope is implemented; file/note attachment and lease take-over are intentionally excluded.
