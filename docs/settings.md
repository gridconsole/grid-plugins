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
| Hook level | `off`, `warn`, `block`; `block` only for brief-lint, host-guard, only-writer and plan-guard |

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

Supported skills are `brain-keeper`, `browser`, `deliver`, `job`, `orchestrate`,
`plan`, `questions`, `research`, `setup`, `signals`, `task`, and
`visual-design-review`. All default to enabled. These settings are configuration
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
calls retain their existing exit-zero contract.

`internal/fslock` has no Windows lock yet, so on Windows every settings write
(`agent set`, `skill enable|disable`, `hook level`, `setup`) fails with its
unsupported-platform error before anything is written. Lists and hook events only
read, and work.

## Hook behavior

Guards, SessionStart and `grid hook levels` read through
`settings.ReadHookLevels`. The existing guards apply the result through one
shared function in `internal/guard/settings.go`. Project identity is resolved
with `internal/where`; hook environment task/project selections also participate.

| Hook | Default | Consumer in this checkout |
| --- | --- | --- |
| brief-lint | block, fixed | Legacy brief guard |
| host-guard | block, fixed | Host and Feed guard |
| only-writer | block, fixed | Task and brain ownership guard |
| plan-guard | block, fixed | Plan guard |
| home-allow | warn | Home permission fallback |
| session-start | warn | SessionStart context |
| job-lint, stop-check | block | Stored for future consumers |
| history-log, needs-you, precompact, session-check, session-end, subagent-start, topic-trigger, usage-log, user-prompt | warn | Stored for future consumers |

The four guards marked fixed always block. They are the person's gates and the
fence around task, brain and project files (`docs/plugin-design.md:90`, `:92`), and a file
an agent can write must not be able to switch them off. `grid hook level` refuses
`off` and `warn` for them with the allowed list, a settings file that says
otherwise is refused as a whole, and the guards do not read the settings files at
all. The host guard also denies agent writes to the settings files and the
`grid hook level`, `agent set`, `skill enable|disable` and `setup` verbs.

For the hooks that may be lowered, `off` suppresses that guard's decision. `warn`
turns a refusal into a `systemMessage` without a permission decision. `block`
preserves the original refusal. Warnings survive arbitration with other guards. A
different guard can still deny the same call. An invalid settings document fails
closed when such a guard would otherwise return a decision. The legacy
`GRID_PLAN_GUARD=off` switch still suppresses plan checks; it does not suppress
the host guard.

For the permission-only home fallback, `warn` and `block` preserve its original
grant, so `warn` is the same as `block` there; `off` suppresses the grant. For
SessionStart, `off` suppresses context but keeps environment initialization;
`warn` and `block` preserve context. If the settings cannot be read, SessionStart
still prints the context, with a one-line note that says so. A level
cannot make an informational hook deny a call. Levels for hooks not implemented
in this checkout are recorded without claiming to implement those hooks: `grid
hook level stop-check off` is stored and nothing reads it yet.

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
