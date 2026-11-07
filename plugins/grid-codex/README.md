# grid-codex

The Grid plugin for Codex. It gives a Codex session the same gates the `grid` plugin gives Claude Code: a subagent does not start on an incomplete brief, a session does not stop with a task half written up, and shell commands that change a task folder land in its `history.md`.

It holds no gate logic of its own. Each hook is a small adapter that turns the Codex hook payload into the input the shared scripts in `plugins/grid/hooks` read, runs that script, and turns its verdict back into Codex hook output. The gates are the same code for both agents.

Written against Codex 0.162.0.

## Layout

```
.codex-plugin/plugin.json   manifest: name, version, skills, hooks
hooks/hooks.json            the hook wiring
hooks/codex.mjs             payload and verdict translation, shared by the adapters
hooks/<name>.mjs            one adapter per shared script
.mcp.json                   the grid-feed MCP server
agents/grid-<role>.toml     the eight Worker role agents (not part of the plugin; see Worker roles)
bin/install-agents.mjs      installs them into the Codex home
```

This folder is the source, not an installable plugin. When Grid stages it for an account (step 18x-i) it adds:

- `hooks/grid/`: a copy of `plugins/grid/hooks`, which the adapters run;
- `skills/`: the Grid skills, which the manifest names;
- `feed/` and `bin/grid-feed`: the Feed server and shell command from `plugins/grid` (copied as they are);
- `.mcp.json` (from this folder) registers the `grid-feed` server, whose tools Codex approves without asking (`default_tools_approval_mode` is set on that server only).

The tests in `tests/hooks-codex/` stage the same layout with `hooks/grid` pointing at `plugins/grid/hooks`.

## Hooks

| Codex event | Matcher | Adapter | Shared script | Codex output |
|---|---|---|---|---|
| SessionStart | any | session-context | `lib.mjs` (read only) | `hookSpecificOutput.additionalContext`: how Grid works, and the tasks this session knows |
| PreToolUse | `^spawn_agent$` | brief-lint | brief-lint | `permissionDecision: "deny"` on an incomplete brief; nothing otherwise |
| PreToolUse | `^Bash$` | history-log | history-log | none (snapshots the task folders the command names) |
| PreToolUse | `^Bash$` | host-guard | host-guard | `permissionDecision: "deny"` on a command that names the host's `.host` folder (the approval queue); nothing otherwise |
| PostToolUse | `^Bash$` | history-log | history-log | none (writes one history line per changed file, by the subagent's role or `codex`) |
| Stop | any | stop-check | stop-check | `{"decision":"block","reason":...}` while task.md or a brief is not written up |
| SubagentStop | any | stop-check | stop-check | same, for the brief that subagent was given |
| PostToolUse | any tool (no matcher) | heartbeat | heartbeat (`hookBeat`) | none (one `heartbeat` post in the Worker's own Feed file, at most once a minute per Job) |
| Stop | any | heartbeat | heartbeat (`hookBeat`) | none (the exit beat, note `stopped`, on the same throttle) |

There is no PermissionRequest hook, and no gate hooks apply_patch: Grid writes the history lines for file edits from the session's `item/completed` events, outside the plugin. The heartbeat is the one PostToolUse hook that sees apply_patch, MCP tools and `spawn_agent`.

## Heartbeat

A Codex Worker beats on its task's Feed like a Claude Worker: the adapter `hooks/heartbeat.mjs` runs `hookBeat` from the shared `hooks/grid/heartbeat.mjs`, so both agents share one throttle (one beat a minute per Job), one lock and one file (`<member>@<host>~<worker>.jsonl`). It acts only when the Orchestrator set `GRID_FEED_TASK`, `GRID_JOB` and `GRID_WORKER` in the Worker's environment.

- Codex's closest events are `PostToolUse` (after each tool call that succeeds, whatever the tool: the beat's note is the tool name) and `Stop` (when a turn ends, not when the session ends; note `stopped`). A Worker that fails a tool call beats on the next one.
- Both are new last groups in `hooks.json`, so the keys and trusted hashes of the earlier hooks (`post_tool_use:0:0`, `stop:0:0`) do not change; the new hooks are `post_tool_use:1:0` and `stop:1:0`.
- There is no `worker-exit` post yet for either agent (plan step 21). Codex 0.162 has a `SessionEnd` event (discovery.rs sets its timeout to 1 s by default and at most 3 s), which is the real "process ended" event for that step; it is not wired here.
- In the staged copy the shared script sits in `hooks/grid/` and `feed/` at the plugin root, so it loads `feed/lib.mjs` from `../../feed/` when `../feed/` is not there.

Every adapter exits 0. A verdict travels as JSON on stdout; an error or a timeout of the shared script (9 s, inside the 15 s `timeout_sec`) gives no output, which Codex reads as "go on".

## Worker roles

A Codex plugin cannot ship agents, so the Worker roles (scout, researcher, builder, runner, reviewer, designer, checker, operator) are TOML agents in `agents/`, installed into the Codex home, never into a repo's `.codex/`:

```
node plugins/grid-codex/bin/install-agents.mjs install [--codex-home DIR]   # DIR, else CODEX_HOME, else ~/.codex
node plugins/grid-codex/bin/install-agents.mjs list
```

A file of the same name that Grid did not install is left alone. The Orchestrator starts a Worker with `workerLaunch` from `plugins/grid/feed/worker.mjs`: for Codex it reads the installed role file and passes its instructions, effort and sandbox to `codex exec`, because `codex exec` cannot take a role file as the top-level agent. The role files pin no model; the Orchestrator passes one.

## Codex facts this relies on

Confirmed in the Codex source at tag `rust-v0.162.0` or in its docs:

- The manifest is `.codex-plugin/plugin.json`, with `name`, `version`, `description`, `skills`, `hooks` and `interface` (`displayName`, `shortDescription`, `developerName`). Paths start with `./` and have no `..`. The loader reads hooks from the `hooks` key, so it is set explicitly rather than relying on the documented `hooks/hooks.json` default. [manifest.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/core-plugins/src/manifest.rs), [agent_plugin_manifest.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/core-plugins/src/agent_plugin_manifest.rs), [Build plugins](https://developers.openai.com/codex/plugins/build)
- hooks.json is `{"hooks": {Event: [{matcher?, hooks: [{type: "command", command, timeout_sec}]}]}}`. `timeout_sec` is in seconds. The matcher is a regex tried on the canonical tool name and then its aliases. [discovery.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/engine/discovery.rs), [Hooks](https://learn.chatgpt.com/docs/hooks)
- A command runs through the default shell with `-lc`, in the session's cwd, with the payload as JSON on stdin and `PLUGIN_ROOT` and `PLUGIN_DATA` in its environment (plus `CLAUDE_PLUGIN_ROOT` and `CLAUDE_PLUGIN_DATA`, per the docs). [command_runner.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/engine/command_runner.rs), [Hooks](https://learn.chatgpt.com/docs/hooks)
- Payloads use the Claude Code field names. Every event carries `session_id`, `transcript_path`, `cwd`, `hook_event_name`, `model`, `permission_mode` and `turn_id`. The tool events add `tool_name`, `tool_input`, `tool_use_id`, `agent_id` and `agent_type`, and PostToolUse adds `tool_response`. Stop adds `stop_hook_active` and `last_assistant_message`. SubagentStop also adds `agent_id`, `agent_type` and `agent_transcript_path`. SessionStart adds `source`. [schema/generated](https://github.com/openai/codex/tree/rust-v0.162.0/codex-rs/hooks/schema/generated)
- The shell tool's hook name is `Bash`, and `exec_command` reports `{command}`. The spawn tool is `spawn_agent`, with the alias `Agent`. Its arguments are `message`, `items`, `agent_type`, `model`, `reasoning_effort` and `fork_context`. [hook_names.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/core/src/tools/hook_names.rs), `codex-rs/core/src/tools/handlers/multi_agents/spawn.rs`
- How Codex reads hook output:
  - PreToolUse blocks with `hookSpecificOutput.permissionDecision: "deny"`, while "allow" without `updatedInput` and "ask" are rejected as unsupported.
  - Stop and SubagentStop continue the turn on `{"decision":"block","reason"}`, and invalid JSON is an error.
  - SessionStart takes `hookSpecificOutput.additionalContext`.

  - Exit 0 with empty stdout completes with no decision for PreToolUse, Stop and SubagentStop. Every adapter's "allow" relies on this.

  [output_parser.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/engine/output_parser.rs), [stop.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/events/stop.rs), [pre_tool_use.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/events/pre_tool_use.rs)
- There is no PostToolUseFailure event, and PostToolUse does not run when the tool fails. [registry.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/core/src/tools/registry.rs)
- A plugin hook runs only once it is trusted (`hooks.state.<key>.enabled` and `trusted_hash` in the config) or managed. [config_rules.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/config_rules.rs), [lib.rs](https://github.com/openai/codex/blob/rust-v0.162.0/codex-rs/hooks/src/lib.rs)

Not confirmed. None of these has been seen in a live Codex run. The tool names and inputs are handled in one place, `claudeTool` in `hooks/codex.mjs`, so a correction stays in that function:

- That PreToolUse fires for `spawn_agent` end to end. This is read from registry.rs, not observed.
- The names and inputs of shell handlers other than `exec_command`. `claudeTool` also accepts `shell`, `local_shell`, `container.exec` and `exec_command`, and `command` or `cmd` given as a string or an argv list.
- Where `PLUGIN_ROOT` and `PLUGIN_DATA` are set in the code, and the plugin data folder's path. The adapters use `CLAUDE_PLUGIN_DATA`, then `PLUGIN_DATA`, and make up no folder when neither is set.
- That Codex sets `stop_hook_active` to true on the turn a Stop block continues. stop-check's loop guard depends on it.
- That a hook sees the `GRID_*` variables of the Worker's own process. command_runner.rs builds a hook's environment from the session's environment snapshot (minus a short list of restricted names), so it should; no live run has shown it (the one real `codex exec` planned for step 20 was refused by the permission check, see Limitations).
- The plugin id format in the trust keys and the input of `trusted_hash`.
- The marketplace format (documented only), and whether `skills` may be a folder path.

## Limitations

- Codex roles (`default`, or a custom `agent_type`) are not Grid's `grid-*` roles. The rules that key on a role, such as the reviewer gate on an approve step, therefore treat every Codex subagent as unknown. The approve-step gate blocks once per brief and then lets the stop through.
- A subagent spawned with `fork_context` carries the parent's history in its transcript, so SubagentStop can find the parent's first brief rather than its own.
- A shell command that fails leaves its `sessions/<id>.bash-<call>` snapshot behind and logs nothing, because there is no PostToolUseFailure.
- Hooks that are not trusted do not run, and Codex says nothing about it. Grid has to trust them when it installs the plugin.
- The host passes no hook trust today (`spawn_codex.rs`, `hooks: &[]`), so in a session Grid starts none of this plugin's hooks run, the heartbeat included, until the host trusts them per run (`-c hooks.state` with the hash `hooks/list` reports). `codex exec --dangerously-bypass-hook-trust` (0.162.0) skips the trust check for one run, but it also runs every other enabled hook from every layer, so it is for a run whose hook sources are vetted.
- Whether a path that sets the trust exists is not confirmed until a live Codex run (step 18x-f). Until then Grid's host runs this plugin's Stop check itself, with the same `stop-check.mjs` and the same payload, for every Codex session, whatever the plugin's state on the account (current, earlier, none). The Stop and SubagentStop rows above are therefore the plugin's own check for a Codex run outside Grid, and a second pass over the same task folder inside it.
- `node` must be on the PATH of the login shell.
- Codex transcripts are rollout JSONL, not Claude Code's format. stop-check finds a brief by its path anywhere in the first 512 KB of the transcript, so the format does not matter to it.

## Tests

```
node --test tests/hooks-codex/*.test.mjs
```

- `contract`: the manifest and hooks.json against the formats above.
- `codex`: the translation functions, and `runShared` against fake scripts.
- `adapters`: each hooks.json command run through a shell against the shared scripts, with Codex payloads.
- `heartbeat`: the heartbeat adapter in a real staged copy under a scratch `CODEX_HOME`: the beat for each tool, the throttle and the exit beat, one throttle shared with the Claude script, parallel hooks, silent failure.
