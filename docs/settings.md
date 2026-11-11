# Setup and settings

Run `grid agent --help`, `grid skill --help`, or `grid hook --help` for usage.

```sh
grid agent set grid-builder --model opus --effort high
grid agent set grid-builder --effort low --project example
grid agent list --json
grid skill disable research
grid skill enable research --project example
grid skill list --json
grid hook level home-allow warn --project example
grid hook levels --json
grid setup example --json
```

Writes without `--project` change personal settings. `--project` accepts a
registered project name or id. Lists resolve the current folder with `grid where`
and show effective values: built-in defaults, then personal settings, then project
overrides. Unregistered folders use personal values. A project effort override
does not freeze an inherited model. Commands do not create projects.

| Field | Allowed values |
| --- | --- |
| Agent model | `haiku`, `sonnet`, `opus` |
| Agent effort | `low`, `medium`, `high`, `xhigh`, `max` |
| Skill state | `enable`, `disable` (stored as JSON booleans) |
| Hook level | `off`, `warn`, `block`; any hook takes any level, and the four guards default to `block` |

Model arguments accept any letter case and are stored in lower case. Efforts,
names and levels use the spelling in the tables. `agent set` requires at least
one of `--model` and `--effort`; supplying an empty value is an error.

| Agent | Default model | Default effort |
| --- | --- | --- |
| grid-builder | sonnet | medium |
| grid-checker | sonnet | low |
| grid-curator | opus | medium |
| grid-designer | sonnet | medium |
| grid-library | haiku | low |
| grid-operator | sonnet | medium |
| grid-planner | opus | medium |
| grid-researcher | opus | medium |
| grid-reviewer | opus | medium |
| grid-runner | haiku | low |
| grid-scout | haiku | low |

Supported skills are `brain-keeper`, `browser`, `deliver`, `plan-jobs`,
`questions`, `research`, `review-job`, `run-job`, `setup`, `signals`, `task`,
and `visual-design-review`. The names `plan-jobs`, `run-job` and `review-job`
are the folder names under `skills/`; the catalog follows the folders because
the agents and agent templates already load those names. All default to enabled. These settings are configuration
for consumers of `settings.Store.Effective`; they do not rewrite installed skill
files or change a running agent's model.

## Files and validation

`GRID_HOME` is resolved through `internal/gridhome`. Personal overrides live in
`<grid-home>/settings.json`, project overrides in
`<grid-home>/projects/<name>/settings.json`. Both use this shape, containing only
explicit overrides:

```json
{
  "agents": {"grid-builder": {"effort": "low"}},
  "skills": {"research": false},
  "hooks": {"home-allow": "warn"}
}
```

One more top-level key is accepted: `accounts`, the array of account records the
app's account reads write into the same file. The plugin does not interpret it
(it must be a JSON array); it is carried through every rewrite (`agent set`,
`skill enable|disable`, `hook level`, `grid agent budget apply|undo`) so the
records are not dropped, and it never appears in the effective values or the
budget read.

Unknown keys at every level are rejected with the allowed list. Null values,
wrong types, duplicate keys and additional JSON documents are rejected. Invalid
input leaves the previous file byte-identical. Settings files and lock files
cannot be symlinks; a registered project's settings directory cannot be a
symlink. Symlinks in the Grid home's existing prefix are resolved, including the
normal macOS temporary-directory aliases.

Writes take the stable `<grid-home>/.settings.lock` using `internal/fslock`, read
again while holding the lock, and publish with `internal/atomicfile`. Data is
synced before rename and the containing directory is synced afterward. First
writes also atomically record `VERSION` while holding the lock. No machine
identity is created.

The `--json` lists are objects keyed by name, sorted lexically, with a trailing
newline. Agents have `model` and `effort`, skills have boolean values, and hooks
have level strings. Setup returns `{"project":"example","answers":{...}}`.
Usage errors exit 2; validation and I/O errors exit 1; success exits 0. Hook event
calls exit zero except when an enabled stop guard blocks completion (exit 2).

Settings writes (`agent set`, `skill enable|disable`, `hook level`, `setup`) take
the same lock on every platform; on Windows it is the lock in
`internal/fslock/fslock_windows.go`. Lists and hook events only read, and take no
lock.

## Hook behavior

Runtime hooks and guards share `settings.ResolveHookLevel`, which resolves the
home and project before calling `settings.ReadHookLevels`. `grid hook levels`
uses the same stored levels. Project identity is resolved with `internal/where`;
hook environment task/project selections also participate. Project overrides beat
personal values, and missing values use the catalog defaults.

| Hook | Default | Consumer in this checkout |
| --- | --- | --- |
| brief-lint | block | Legacy brief guard |
| host-guard | warn | Host and Feed guard; warns by default, `block` refuses the write |
| only-writer | warn | Task and brain ownership guard; warns by default, `block` refuses the write |
| plan-guard | warn | Plan guard (folder name and agent role folders); warns by default, `block` refuses the write |
| home-allow | warn | Home permission fallback |
| session-start | warn | SessionStart context |
| stop-check | block | Stop and SubagentStop completion checks, including saved Job verification |
| history-log | warn | File, Bash, subagent, brain-read and brain-recall history entries |
| usage-log | warn | Usage totals and session/subagent transcript references |
| session-end | warn | Worker exit records and SessionEnd terminal state |
| subagent-start | warn | SubagentStart brain recall context |
| user-prompt | warn | UserPromptSubmit brain recall context |
| job-lint | block | No runtime counterpart; has no effect yet |
| needs-you, session-check, topic-trigger | warn | No runtime counterpart; have no effect yet |

No guard level is fixed. brief-lint, host-guard, only-writer and plan-guard read
their level like every other hook; brief-lint defaults to `block`,
host-guard, only-writer and plan-guard to `warn`; `grid hook level`
takes `off`, `warn` or `block` for them. A settings file the guard cannot read
keeps its refusal and reports why. While the host guard is at `block` it denies
agent writes to the settings files and the `grid hook level`, `agent set`,
`skill enable|disable` and `setup` verbs, so an agent does not lower a level; a
person does, in Grid or with the command.

The plan guard does not refuse repo writes because a task's plan is not accepted.
Whether a plan needs a person's approval is a step of the task's workflow (an
`approve` step), and the workflow's default says who answers and when. The
guard's other plan rule (the grid-planner's reach into a task folder) stays.

For every guard, `off` skips the check, `warn` turns a denial or stop
block into a warning, and `block` keeps the original decision. Permission warnings
use `systemMessage`; stop warnings go to stderr and exit zero. Stop checks at
`off` skip saved Job verification and brief marking too; `warn` still performs
those checks and records their evidence. Warnings survive arbitration with other
guards, so another guard can still deny a call. The permission-only `home-allow`
fallback grants at both `warn` and `block`; `off` suppresses that grant.

For context hooks, `off` adds no context; `warn` and `block` preserve it. For log
hooks, `off` writes nothing for that hook; `warn` and `block` preserve logging.
`session-start off` still initializes the environment. `history-log off` also
skips Bash snapshots and brain history entries, while recall context continues.
`session-end off` skips worker exit recording and ending the worker's session,
including the worker-exit path on SubagentStop. Other hooks sharing the event
still run under their own settings. Usage settings do not control the separate
home permission counter or supervisor heartbeat.

Unreadable settings or project resolution errors are reported and cannot disable
a hook. A guard keeps its denial; an unreadable home permission fallback denies
instead of granting. Stop-check retains its exit-2 block when it finds unfinished
work. Context and log hooks run as on and report the settings error on stderr;
SessionStart also retains its context note. The legacy `GRID_PLAN_GUARD=off`
switch still suppresses plan checks; it does not suppress the host guard.

The inactive names above remain accepted and stored for compatibility. Setting
`job-lint`, `needs-you`, `session-check` or `topic-trigger` currently changes no
runtime behavior. See [the L64 lane note](lanes/L64.md) for the complete mapping
and the default policies that an owner may change.

## Setup answers

`grid setup <project>` reads `setup-answers.json` in the current folder and records
the validated answers in
`<grid-home>/projects/<name>/setup-answers.json`. `--answers <file>` selects a
file, and `--answers -` reads stdin. Input is limited to 1 MiB and must be exactly
one JSON object. All values are strings.

This is the lane's explicit interchange format: the old setup skill writes
`proposals.md` and question Markdown, and the supplied new design does not define
an answers-file encoding. A skill or caller supplies accepted setting answers
using the old skill's setting keys:

| Key | Allowed values |
| --- | --- |
| model | `haiku`, `sonnet`, `opus` (letter case normalized) |
| effort | `low`, `medium`, `high`, `xhigh`, `max` |
| lands | `Open a pull request`, `Draft pull request`, `Push the branch`, `Merge, don't push`, `Merge, then push`, `Ask each time` |
| deploys | `Don't deploy`, `Deploy from this machine`, `CI/CD deploys` |
| long | `Any time`, `Overnight only`, `Ask first` |
| account, description, docsFrom, starts, stops, test | Free text |
| deployCommand | Free text; nonempty only with `deploys: Deploy from this machine` |

```json
{
  "description": "Example application",
  "test": "make test",
  "model": "sonnet",
  "effort": "high",
  "lands": "Open a pull request",
  "deploys": "CI/CD deploys"
}
```

The whole input is validated before creating directories, locks, version files or
answer records. A successful call replaces the answer record, rather than merging
it; `{}` records an empty answer set. It does not edit `project.md`, run commands,
install tools, change agent overrides, or apply proposals to brain topics. The
stored record is canonical JSON, so original whitespace is not retained.
