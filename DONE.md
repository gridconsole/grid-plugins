# Lane L36

Implemented `grid routine add`, `list`, `remove`, `run`, `schedule`, `unschedule`, and the read-only `grid worker list`. The existing module and Makefile are unchanged. All commits are on the lane's current branch.

## Commands and files

```sh
grid routine add keeper --every 24h --run brain-keeper
grid routine add inspect --every '0 9 * * 1-5' --run 'grid brain index --project example' --project example
grid routine list --json
grid routine list --project example --json
grid routine run keeper
grid routine schedule
grid routine unschedule
grid routine remove keeper
grid worker list --json
```

Home entries live in `~/.grid/knowledge/routines/<name>.md`; project entries live in `~/.grid/projects/<project>/knowledge/routines/<name>.md`. `GRID_HOME` overrides the data directory. Each file has strict YAML front matter with `name`, `every`, `run`, `machine`, optional `last_run`, and `outcome`. Markdown notes survive state updates. The brain config API reuses its scope checks, front-matter parser, stable brain lock and rooted atomic writer; directory synchronization uses `internal/atomicfile`. Config entries are counted separately from knowledge in the brain index and never enter knowledge search.

Without `--project`, every routine command selects home, even inside a project. An explicit project name or ID must resolve to a registered project. Entries are never silently overwritten. Remove first unregisters an installed timer, then deletes its entry under the same timer lock. Per-entry locks serialize runs, edits and removal through the existing `internal/localdata` / `internal/fslock` implementation. Machine identity comes from the existing Grid home helpers.

`run` executes once immediately. OS timers invoke `run <name> --due`, which applies the interval guard under the entry lock. `last_run` and `running` are written before execution, then `success` or `failed` afterward. An interrupted process leaves a visible `running` outcome rather than falsely claiming success. Failed executions raise a Personal/Home signal through `internal/signal`. No task or Feed record shapes were introduced.

`--run 'grid <verb> ...'` executes the same CLI binary with quoted argv and no shell expansion. A verb with arguments may omit the `grid` prefix. Project scope selects the routine definition; a grid command must include its own `--project` or `--task` flags where needed. Skills receive the project in their prompt. A lone name is a skill; `grid:<name>` also identifies a skill. Recursive routine commands and shell operators are refused. Skills use `claude -p`, a narrow tool allowlist, a one-dollar budget, ten turns and a ten-minute outer timeout. The child environment excludes parent API keys, task leases and arbitrary variables. On macOS and Linux, cancellation and cleanup kill the whole child process group. Child output goes to stderr when the parent uses `--json`.

## OS schedules

Schedules are opt-in and only install for this machine's entries in the selected scope. Each OS owns a one-minute wake; Grid exits after every invocation and keeps no scheduler daemon. Durations must be whole seconds of at least one minute, with up to one minute of wake latency. Cron uses five numeric fields in UTC, including lists, ranges, steps, Sunday 0/7, and the usual day-of-month/day-of-week OR rule. Missed cron occurrences are not replayed after sleep; overdue durations run on the next wake.

- macOS: `~/Library/LaunchAgents/com.gridconsole.routine.<id>.plist`, registered with launchctl in the current GUI user domain.
- Linux: `~/.config/systemd/user/com.gridconsole.routine.<id>.service` and `.timer`, registered with systemctl's user manager.
- Windows: Task Scheduler XML and schtasks argv are generated. Native commands print a preview, including XML, instead of invoking schtasks because the shared Windows lock implementation returns `ErrUnsupported`.

Native backend selection is build tagged. All three generators are testable on any host. Labels incorporate a hash of the local Grid home, scope and routine name. Files include an ownership marker; confirmed installations have a local ownership manifest in `local/routine-timers/`. Existing foreign files, symlinks and unconfirmed OS registrations are never removed. OS registration/removal errors are returned and retain evidence for retry. Unschedule can remove confirmed installations even after someone manually deletes their config entries. PATH and the selected Grid home are carried into installed timers; platform-specific argument escaping is tested.

## Worker presence format

The architecture specifies one process and a lifetime OS lock per machine, but does not fix filenames or a JSON presence schema (`docs/go-architecture.md:109-112,274`). This lane proposes:

```json
{"version":1,"machine":"01ARZ3NDEKTSV4RRFFQ69G5FAV","pid":123,"started_at":"2026-11-08T09:00:00Z","updated_at":"2026-11-08T09:00:30Z","state":"running"}
```

The app writes `worker.json` atomically in the Grid home, refreshing it every 30 seconds. `state` is `running` or `stopped`. The stable `worker.lock` inode contains `version`, `machine`, `pid`, and `started_at`, and the app holds its OS lock for its lifetime. Both files identify the same process incarnation.

`worker list` only reads these files. It filters out other machines and reports `presence: reported` for matching files with a running heartbeat no older than 90 seconds. Missing or mismatched locks and stale heartbeats give `stale`; a lock without status gives `unknown`. This is reported presence, not proof that a PID is alive or that an advisory lock is held. Listing never blocks on the worker's lifetime lock and never writes files. Missing files produce `[]`; malformed or unsafe files produce an error.

The worker requirement requested a new type in `internal/contract`, while the later lane rule explicitly prohibited editing that directory. No exception was confirmed, so the provisional types and wire-format test live in `internal/worker`. Existing contract files were not modified. The closed app must adopt or reconcile this format before the list can show its workers.

## Intended differences from the old plugin

- Routines now have a validated config writer and actual readers at both home and project levels; the old map listed signals but no landed routine reader (`old-plugin-map.md:63,88,115`; `docs/brain-feed.md:9-11,253`). Config cannot be promoted by the knowledge keeper or treated as knowledge text.
- The old application's scheduler is not recreated. Optional local OS entries invoke a short-lived CLI; no desktop dependency, internal clock, SSH copying or worker startup was added (`docs/go-architecture.md:33,346,517`; `docs/plugin-design.md:455,460,465`).
- Each entry records its owning machine and only that machine executes it, following the multi-machine duplicate-run decision (`docs/brain-feed.md:11`).
- Manual `run` always executes once as requested in the lane brief. The interval guard applies to timer calls through `--due`; this is a deliberate clarification of the broader wording in `docs/plugin-design.md:460`.
- Five-field UTC cron and durations of at least a minute are the supported schedule dialect. Minute wakes and no replay of missed cron slots are explicit first-cut choices; the design did not fix a cron dialect, timezone or missed-run policy (`docs/plugin-design.md:455,460`; `docs/findings-brain.md:98`).
- Failure signals use the landed Go signal writer, rather than the old signals skill's Node script. This lane does not change that skill or its source-reading behavior (`old-plugin-map.md:63,115`; `docs/plugin-design.md:461-462`; old `skills/signals/SKILL.md:18-24`).
- `grid worker list` reads machine-local process presence only. It does not launch agent sessions, poll the server, join machines, or infer workers from old Feed heartbeats (`old-plugin-map.md:43,58,71,86`; `docs/go-architecture.md:109-112,155,274`; `docs/plugin-design.md:463-465`).

## Validation

Run from the repository root, with the default TMPDIR:

```sh
make test
make lint
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

All four checks passed in this lane. `make build` and a built-CLI smoke test of add, successful run, due suppression, remove and both lists also passed with an isolated fake home. Tests include CLI flags and JSON, home/project isolation, concurrent due runs, manual runs, failure outcome and Home signal recording, environment scrubbing, process-group timeout, strict config parsing, symlink rejection, worker-format round trips and stale/missing/mismatched presence. All OS generators have fake-home install/remove round trips, ownership refusal, failure handling, preview and escaping tests. No test invokes launchctl, systemctl, schtasks or an agent runtime.

## Limits and deferred integration

- Actual OS service registration was not exercised on this host. Tests use the injected OS-call interface. macOS execution tests were not run on macOS; Darwin cross-compilation passed. Native Windows writers and execution remain blocked by the intentionally unsupported shared file locks, which this lane was forbidden to change. Windows schedule/unschedule provide the permitted command preview.
- There is no cross-system transaction between OS registration and the local ownership manifest. A crash after successful registration but before manifest confirmation can leave a timer requiring manual inspection. To avoid deleting another program's registration after a label collision, unconfirmed registrations are never automatically removed. External manual removal of a confirmed OS timer can also require manual reconciliation if the OS returns an error on a repeated unregister.
- Worker Job handoff is not implemented: the closed worker does not have a landed routine-submission protocol in this checkout, and inventing Feed/task records would bypass the shared contract. Runs use the local one-shot executor (`docs/plugin-design.md:460`; `docs/go-architecture.md:33`).
- Claude is the implemented headless skill runtime. Codex fallback, local MCP server configuration and setup-token provisioning are not added. The requested add command has no runtime/MCP/token options, and no shared provisioning path is present. Unschedule does not delete unrelated credential files; this lane creates none (`docs/plugin-design.md:38,460-462`; `docs/findings-brain.md:98`).
- The automatic first-call-after-24-hours keeper sweep and a default keeper entry are not added. The lane forbids dispatcher edits beyond command registration, and brain upkeep remains a later slice in the references. This implementation supplies the optional scheduled reader without replacing or claiming to implement the lazy sweep (`docs/brain-feed.md:54`; `docs/findings-brain.md:71,92,98`; `docs/plugin-design.md:471`).
- Config index rows count structurally valid front matter; executable routine-schema validation remains in the routine reader. No new config audit log or task Feed post is emitted because routine commands can operate at home without a task (`docs/brain-feed.md:260,300`).
