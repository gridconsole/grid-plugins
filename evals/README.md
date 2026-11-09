# Eval task suites

Run from the repository root with Go and a POSIX shell installed:

```sh
go run ./cmd/grid eval list
go run ./cmd/grid eval run
go run ./cmd/grid eval run fix-total --json
go run ./cmd/grid eval show <run-id>
go run ./cmd/grid eval show <run-id> --json
```

`run` defaults to the embedded fake agent. It deterministically completes the
three sample tasks without network access or model calls. Results label these
runs `fake`; passing them verifies the harness, not model quality. A fake agent
run of any other task fails explicitly.

`list` and `run` accept `--suite <directory>`, `--tier b|c`, and
`--size tiny|small|normal|large`. The suite defaults to `evals/tasks` relative to
the caller's working directory. `run` accepts one optional task id before or
after its flags. Without an id it runs every matching task in folder-name order.
An empty selection is an error for `run`. `run` and `show` accept
`--results <directory>`; the default is `evals/runs` under the Grid home
(`$GRID_HOME`, else `~/.grid`), not the working directory. When no Grid home can be
found, it falls back to `evals/runs` under the working directory. `--json` is
available on all three.

Configure a real agent with an argv array:

```sh
go run ./cmd/grid eval run fix-total --agent '["claude","-p"]' --timeout 10m
go run ./cmd/grid eval run fix-total --agent '["codex","exec","-"]' --timeout 10m
```

The prompt is sent on stdin. An argv element exactly equal to `{prompt}` is also
replaced with the full prompt, without shell interpolation. Commands execute in
the fresh workspace. Use an absolute executable or script path for a custom
agent stored outside it, or an executable on PATH. Shell syntax is interpreted
only when explicitly invoking a shell. No credentials are stored in the suite
or supplied by the harness. Configure authentication through the runtime's
supported environment variables, passed with `--env`; HOME is temporary, so an
agent cannot rely on the usual home-directory login files.
Agent argv is saved in results; keep credentials out of command arguments.
If a runtime requires a Git repository, initialize one in `setup.sh` or supply
that runtime's option for evaluating a directory without a repository.

The default task timeout is five minutes across setup, agent execution and checks.
On Linux and macOS, cancellation kills the command's process group; background
children in that group are also killed when a command finishes. Other platforms
only cancel the direct process and require compatible shell utilities. Ctrl-C
saves the current task's failure and stops scheduling more tasks.

### Agent environment

Setup, the agent and command checks start with an empty environment, not the
caller's. They receive `PATH`, `LANG`, `LC_*` and `TERM` from the caller, plus
per-attempt `HOME`, `USERPROFILE`, `XDG_CONFIG_HOME`, `TMPDIR`, `PWD`, `GRID_HOME`,
`GRID_PROJECTS` (all inside the attempt's temporary folder) and `GRID_EVAL_TASK`.
Nothing else is inherited, so a run started from a Claude or Grid session cannot
reach that session's task folders, `GRID_*` settings, `CLAUDE_CODE_*` variables,
`CLAUDE_ENV_FILE` or messaging sockets and tokens.

A real agent usually needs a login. Name each variable it may receive with
`--env` (repeatable); it is copied only when set in the caller's environment:

```sh
go run ./cmd/grid eval run fix-total --agent '["claude","-p"]' --env ANTHROPIC_API_KEY
```

`GRID_*`, `CLAUDE_CODE_*`, `CLAUDE_ENV_FILE`, `CLAUDE_PLUGIN_ROOT`, names containing
`MESSAGING`, and the per-attempt variables above are refused. Passed values reach
the agent and any code it runs, and the agent's output is saved in the result.

## Folder format

Each immediate directory in a suite is one task:

```text
tasks/
  fix-total/
    task.json
    prompt.md
    setup.sh
    checks.json
```

`task.json` has these required fields:

```json
{
  "schema_version": 1,
  "id": "fix-total",
  "title": "Fix a failing total calculation",
  "tier": "c",
  "size": "small"
}
```

The id must match the folder name and contain only ASCII letters, digits,
hyphens and underscores, starting with a letter or digit. The eval levels are
tier `b` (behaviour suites) and tier `c` (corpus tasks); tier `a` is `make test`.
Size is one of `tiny`, `small`, `normal`, `large`. These fields describe and filter
tasks; they do not impose token or monetary caps.

`prompt.md` is nonempty plain text. `setup.sh` is a nonempty POSIX shell script
executed with `sh -eu -c` in a new workspace. A script containing `true` suffices
for an empty fixture. Setup can create project files and seed `$GRID_HOME`.
The agent sees `$GRID_EVAL_TASK`, the task id. The runner creates a temporary
HOME, USERPROFILE, XDG_CONFIG_HOME, TMPDIR, GRID_HOME and GRID_PROJECTS, and passes
only the environment described under "Agent environment".

All suite scripts, agent commands and command checks are trusted local code.
Temporary directories provide fresh state, not an OS security sandbox: commands run
with the caller's permissions and can read any file that user can. Run untrusted
agents or suites in a separate container or other sandbox. The runner loads the setup, prompt and
checks before execution and does not copy the check definitions into the workspace.
Command checks can still invoke files the agent modified, so keep authoritative
assertions in `checks.json`, as the samples do.

`checks.json` is a nonempty array. Check ids are unique within the task:

```json
[
  {"id":"output","type":"file_exists","path":"answer.txt","gate":true},
  {"id":"content","type":"text_contains","path":"answer.txt","text":"expected","weight":2,"rubric":"S3"},
  {"id":"verify","type":"command_passes","command":["sh","-c","test -s answer.txt"],"rubric":"S3"},
  {"id":"quality","type":"llm_judge","rubric":"S3: answer is complete and correct"}
]
```

| Type | Required fields | Pass condition |
| --- | --- | --- |
| `file_exists` | `path` | A regular file exists inside the workspace. |
| `text_contains` | `path`, nonempty `text` | The file contains the literal, case-sensitive text. |
| `command_passes` | Nonempty `command` argv array | Command exits zero. |
| `llm_judge` | Nonempty `rubric` | Injected `eval.Judge` returns a passing judgement. |

File paths must be relative and stay inside the workspace, including through
symlinks. Text checks read at most 1 MiB; larger files are check errors. Missing
files, directories instead of files, missing text and nonzero command exits are
failures. Command startup errors and timeouts are errors. Each captured stdout
and stderr stream is capped at 64 KiB and marked `truncated` when necessary.
Unknown JSON fields, duplicate check ids, unknown check types and unsupported
schema versions are rejected before any task starts.

The Go `eval.Judge` interface receives the task, check, context and workspace and
returns a pass flag, score from 0 to 5, reason and optional relative `path:line`
evidence. Implementations must respect the context deadline and manage their
own authentication and budget. The CLI has no installed judge. Requesting one
without an implementation produces an error, never a passing or omitted check.

## Results and scoring

Each task attempt is saved atomically to `<results>/<run-id>.json`. Run ids combine
a UTC timestamp with a random suffix. JSON includes schema version, task id,
title, tier, size, task configuration hash, configured agent argv, fake-agent
marker, start and finish times, timeout, duration, setup and agent outcomes,
individual checks with reasons and evidence, and aggregate counts.

Text evidence includes a matching line number. Command checks retain exit code,
stdout, stderr, timing and any error in their `process` field. The configuration
hash covers the task metadata, prompt, setup and checks, but not external runtime
versions or the variables passed with `--env`. Save those separately when comparing
real model runs. The temporary folder is deleted before the result is saved; if
that fails, the result's `cleanup_error` names the folder and `run` exits with 1.
Directories the agent made read-only are made writable again first. Readers ignore
unknown fields, so a result written by a newer build stays readable.
The runner no longer writes a `trace.jsonl`: trace checks read the agent's stdout from
memory (capped at `eval_trace_stdout_limit`), so nothing the agent writes in its
workspace or `$HOME` changes the trace.
Full workspaces and transcripts are not retained; result files can contain
command output and should be reviewed before sharing. A results folder at
`evals/runs` in this repository is ignored by Git.

Code checks score 5 for pass and 0 for fail. Judge checks use the returned 0..5
score. `check_score` is the weighted mean of non-gate checks, with default weight
1; explicit weights must be between 0 and 1000, with 0 meaning the default.
Gates count separately and never change that mean. Any failed gate or check makes
the run fail even if the score is high. Errors make the run an error. If setup or
the agent fails, checks are skipped. Any non-gate check without a score makes the
mean `null` (`not measured` in the table); all-gate tasks also have no mean.

This is a harness check score, not the design's full S1-S6 rubric score or an exit
bar. The optional `rubric` label records intent; it does not claim to measure a
whole rubric line. Neither this harness nor the fake agent claims G1-G5 coverage.

`list --json` emits an array of task definitions (excluding prompt and setup).
`run --json` emits an array of attempted run results. `show --json` emits one
saved result. Human output provides a summary table; `show` also lists check
reasons, evidence and process outcomes. Exit codes are 0 for success, 1 for failed
checks or execution/storage errors, and 2 for command usage errors. `show` exits
zero when it successfully reads a failed run. A failed task does not prevent the
rest of a suite from running; invalid suites are rejected as a whole.

## Tests

```sh
make test
make lint
go test ./internal/eval ./internal/evalcmd ./internal/cli -run Eval
```

`make test` includes every sample with the fake agent plus isolation, failures,
timeouts, scoring, judge hooks, persistence, output limits and CLI tests. The
optional final command selects the CLI integration test; use
`go test ./internal/eval ./internal/evalcmd` to run all harness-specific tests.
