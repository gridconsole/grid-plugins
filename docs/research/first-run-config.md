# First-run setup and one config shared by plugin and app

Research for lane R-grid-first-run-config (owner findings 6 and 7, 17 Nov 2026). No code is changed here.
Paths are relative to this repo (plugin) unless written `app:` (the app repo, main as of 17 Nov 2026).
`~/.grid/<name>` stands for a person's data folder one level under `~/.grid`, the layout found on the owner's Mac.

Evidence kinds: `file:line` in this repo or the app repo; Claude Code docs at code.claude.com read on 17 Nov 2026
(pages carry no version label, so minimum versions are quoted where the page gives them); the Claude Code that ran this
lane recorded `"version":"2.1.325"` in its own session record (`claude --version` was refused by the shell allow list).
NOT VERIFIED marks a claim that needs a live interactive Claude Code or a Mac.

## 1. How a Claude Code plugin can run a first-run step

**There is no install or enable hook.** The hooks reference lists 33 events (SessionStart, Setup, UserPromptSubmit, ...,
SessionEnd); none fires on plugin install or enable. `Setup` fires only "When you start Claude Code with `--init-only`, or
with `--init` or `--maintenance` in `-p` mode. For one-time preparation in CI or scripts", and "Claude Code discards a Setup
hook's JSON output fields, such as `systemMessage`, `continue`, and `hookSpecificOutput.additionalContext`"
(https://code.claude.com/docs/en/hooks). `ConfigChange` sources do not include plugins.

The options:

| Option | What the person sees | A question without typing anything? | Plugin uses it today |
| --- | --- | --- | --- |
| `userConfig` in `.claude-plugin/plugin.json` | A configuration dialog: "values Claude Code prompts the user for when the plugin is enabled"; also rows in `/config` and **Configure options** in `/plugin` (https://code.claude.com/docs/en/plugins-reference#user-configuration) | Yes, but only when the plugin is enabled in the interactive `/plugin` panel. The shell install page (https://code.claude.com/docs/en/plugins/install, "Install from your shell") describes no dialog; values come from `claude plugin install --config key=value` or `claude plugin configure <id> --values-stdin` (v2.1.285 or later, https://code.claude.com/docs/en/plugins/cli-reference). Dialog timing NOT VERIFIED live | No: `.claude-plugin/plugin.json:1-11` has no `userConfig` |
| First SessionStart noticing no config | `systemMessage`: "Warning message shown to the user"; `additionalContext`: inserted "at the start of the conversation, before the first prompt" for Claude only; SessionStart is "Context only ... No blocking or decision control" (hooks page). `initialUserMessage` starts the session with a user turn "in non-interactive mode with the `-p` flag" only, and is ignored from a plugin installed after the hooks ran | No. A hook cannot ask the person; it can show one line and tell the agent to offer setup at the first prompt | Yes, `additionalContext` only: `internal/hook/hook.go:152-155`; registered at `hooks/hooks.json:69-80`. No `systemMessage` is written. With no Grid home it prints nothing (`hook.go:114-116`) |
| A skill `/grid:<name>` driving AskUserQuestion | AskUserQuestion questions with choices, once the skill runs | No: the person types the command, or the agent runs the skill after the person's first message (prompted by the SessionStart context) | `/grid:setup` exists (`skills/setup/SKILL.md:38-43`) but asks in prose, not AskUserQuestion, and covers project setup plus `grid setup` |
| `grid ...` in a terminal | Whatever the command prints | No | `grid setup` (`internal/cli/settings.go:196-221`: preview of Claude Code settings entries, `--yes`, `--remove`, `--sandbox`, `--json`) and `grid setup <project> --answers` (`settings.go:223-280`) |
| The Grid app runs the step | The app's onboarding page | Yes, on first open (see section 2) | The app installs the plugin but asks nothing about the data folder (section 2) |

Limits of `userConfig` that rule it out as *the* config: defaults are static values in the manifest (no default computed
from the machine); non-sensitive values are stored under `pluginConfigs` in Claude Code's `settings.json`; they reach
hook processes as `CLAUDE_PLUGIN_OPTION_<KEY>` and skill text as `${user_config.KEY}`, but not `grid` run through the
Bash tool ("The variables aren't present in the environment of commands Claude runs through the Bash tool"), not Codex,
not the app. At most it is one more place to ask, and a hook would copy the answer into the shared file.

**Recommendation.** Use the first SessionStart: when no config exists and no data is found, the hook writes a
`systemMessage` line to the person ("Grid is not set up on this computer yet. Type /grid:first-run, or open the Grid app.")
and an `additionalContext` line telling the agent to offer `/grid:first-run` at the first prompt. When data is found
(for example the older layout), the hook resolves it silently and says nothing. This is an offer; every command and hook
keeps working on the defaults (no gate). The skill runs `grid init --detect --json`, asks only the gaps with
AskUserQuestion, and writes with `grid init --answers -`. Do not add `userConfig` now (see the optional lane P4).

## 2. The Grid app path

Today:
- The app installs the plugin with Claude Code's own commands, user scope, after an explicit confirm:
  `app:internal/slices/setup/plain.go:77-80` (`claude plugin marketplace add gridconsole/grid-plugins`, then
  `claude plugin install grid@grid --scope user`), served as `install_plain_plugin` (`plain.go:94-96`); update at
  `plain.go:85-88`. Codex install is not built (`plain.go:95`).
- Detection only displays: `app:internal/slices/agentreads/plugin_hosts.go:14` ("Nothing here installs anything"),
  Claude at `:77-118` (`InstallCmd` at `:106`), Codex at `:47-61`.
- The app runs the plugin through `app:internal/kernel/plugincli/client.go:139` (`Home`) and `:176-179` (`settings()`):
  the launcher `<Home>/bin/grid` (`app:internal/kernel/plugincli/vn_launcher.go:14-29`, falling back to `GRID_HOME`
  at `:35-37`), else `grid` on PATH. The person check runs `grid person check --json`
  (`app:internal/kernel/plugincli/person_check.go:55-87`).
- Onboarding steps: `app:internal/kernel/webui/app/pages/onboarding/logic.mjs:9-13` (agents, projects, plugin,
  first-task) with "set up later" steps at `:17-20`. The Start screen asks the handle; the step's write list says it goes to
  "config.json in Grid's app folder" (`logic.mjs:392`), but the server writes `<home>/setup/first-run.json`
  (`app:internal/slices/setup/first_run.go:14-19, 36-38, 221-249`). The page assumes a data folder `~/.grid/<handle>`
  (`logic.mjs:380, 389, 432-433`), while the server serves one home (`first_run.go:124-150`, "This server keeps one
  Grid home").

How the app runs the same setup (proposal):
1. Install the plugin as today (`plain.go:77-80`). Do not depend on a `userConfig` dialog (none on a shell install).
2. Run `grid init --detect --json` through `plugincli.Client`. It prints the question set with the defaults filled from
   the machine (section 5) and the config as it is.
3. Show those questions on the Start step, prefilled; skip the ones `--detect` marks as settled.
4. Send the answers with `grid init --answers - --json` (stdin JSON, like `grid setup <project> --answers -`). The plugin
   is the one writer of `~/.grid/config.json`.
5. Re-read the config (the app's resolver, section 4) and serve the chosen data folder. If the data folder changed,
   the desktop restarts its `grid-server` child with the new home.
When the plugin is not installed yet, the app shows its own Start step and writes the same file with the same Go rules
(shared fixtures, lane A1), so a later plugin install reads it.

## 3. Today's config and home resolution

### Plugin

Home: `internal/gridhome/gridhome.go:17-27` `Dir`: `GRID_HOME`, else `<userHome>/.grid`. Callers:
`internal/cli/cli.go:114` (`OSEnv`, every command), `internal/guard/settings.go:55` and `internal/guard/context.go:42`
(hooks' caller resolver). `--grid-home` maps to `GRID_HOME` (`internal/cli/global.go:15`, made absolute at `:51-56`);
`grid routine` has its own `--grid-home` (`internal/cli/routine.go:58, 105`). Timers bake `GRID_HOME` in
(`internal/routine/execute.go:214`, `timer_plist.go:37`, `timer_systemd.go:28`). Each SessionStart writes
`GRID_HOME` and `GRID_PROJECTS` into `CLAUDE_ENV_FILE` (`internal/hook/hook.go:160-169, 226-228`), so Bash-run `grid`
inherits what the hook resolved.

Brain: `<home>/knowledge/<topic>/<name>.md` and `<home>/projects/<p>/knowledge` (`internal/brain/config.go:29`,
`internal/brain/task_decide.go:195-197`). Projects: `GRID_PROJECTS` (`internal/cli/settings.go:213-216`), default
`<home>/projects` (`internal/settings/claude.go:43-45`). There is no separate brain location.

Member: `GRID_WHO`, `GRID_ACTOR`, `GRID_MEMBER` (`internal/cli/brain.go:41-47`; `--member` at `internal/cli/global.go:16`;
also `internal/cli/task.go:400`, `task_write.go:175`, `signal.go:117`, `internal/task/create.go:203`). No default member
and no file holds it. People records: `internal/people/store.go:16-17`.

Accounts: `internal/account/account.go:115-149` lists `~/.claude`, `~/.codex`, `~/.claude-*`, `~/.codex-*`, and the
folders in `CLAUDE_CONFIG_DIR` and `CODEX_HOME`; record shape at `internal/account/registry.go:20-27` (`id`, `kind`,
`name`, `email`, `dir`, `added`).

Workspaces: none. "workspace" in the plugin means the home-level `workflows.md` (`internal/workflow/files.go:199`).

Files the plugin writes under the home: `VERSION` (`internal/task/create.go:130`, `internal/settings/store.go:425`);
`machine` (`internal/task/create.go:133`, `internal/feed/identity.go:22`); `accounts/registry.json`
(`internal/account/registry.go:88-115`); `settings.json` (`internal/settings/store.go:344, 385`, shared with the app: it
carries the app's keys through, `store.go:24-32, 65-84`); `projects/<p>/project.md`
(`internal/project/project.go:312`); `projects/<p>/setup-answers.json` (`internal/settings/setup.go:72`); brain notes
(`internal/brain/store.go:117`); `knowledge/budget/agent-budget.md` (`internal/settings/budget.go:61`);
`home-allow.jsonl` (`internal/hook/count.go:29`); `workflows.md` (`internal/workflow/write.go:141`); task folders under
`personal/tasks` and `projects/<p>/tasks` (`gridhome.go:178-186`). It reads the launcher `<home>/bin/grid`
(`internal/hook/hook.go:161`, `internal/routine/launcher.go:20-21`). Outside the home: Claude Code `settings.json`
(`internal/settings/claude.go:254` via `grid setup`). Not traced: the write paths of `internal/hookevents` and
`internal/feed/store.go`.

### App

Home resolvers (seven, each its own):
- `app:cmd/grid-desktop/main.go:35-42` `gridHome()`: `GRID_HOME`, else `~/.grid`; again at `app:cmd/grid-desktop/api.go:356`.
- `app:cmd/grid-server/main.go:76` `--home` (empty serves only health); `main.go:156` reads `GRID_HOME` for the person check.
- `app:cmd/grid-server/wire.go:1173-1175` falls back to the user's home folder.
- `app:cmd/grid-worker/main.go:93` `-home` ("otherwise GRID_HOME, then ~/.grid").
- `app:internal/platform/crash/crash.go:17-19`.
- `app:internal/kernel/plugincli/vn_launcher.go:35-37`.

The older `~/.grid/config.json`: no Go code in the app reads or writes it (`git log -S'config.json' -- cmd internal/slices/setup`
in the app repo finds nothing; the agent search of `cmd/` and `internal/` found no reader). Its shape is the earlier
Rust desktop's (`ref rust-app: crates/grid-desktop/src/setup/config.rs:1-127`): `v` (2), `workspaces[]` with `name`,
`folder` (the repositories' folder), `data` (absolute data folder when not `~/.grid/<name>`), `member`, `setup`
(`new|started|done|skipped`), `step`, unknown keys kept; `current`; and the v1 top-level keys `workspace`,
`projectsFolder`, `member`, `setup`, `step`, migrated in memory (`config.rs:181-238`: the name is the last part of an
absolute `workspace`, else `member`, else the login; data `~/.grid/<name>`). That Rust source put the file in the Tauri
app config folder (`crates/grid-desktop/src/lib.rs:380`), not `~/.grid`; the owner's Mac has it at `~/.grid/config.json`
(finding 6). The exact Mac file is NOT VERIFIED here: lane P1 starts by taking a redacted copy as its fixture.

Member: `setup_status.member` from `<home>/setup/first-run.json` (`app:internal/slices/setup/first_run.go:27-34, 69-91`),
saved by `save_setup` (`first_run.go:154-213`), read by the page at `app:.../pages/onboarding.mjs:229`. The person check
passes `GRID_HOME`, `GRID_MEMBER`, `GRID_WHO`, `GRID_ROLE`, `GRID_ACTOR` (`person_check.go:118`). `settings.json`'s
`workspace` key is a settings section (`tasksFolder`, `transcriptsKeep`; `app:internal/slices/settings/order.go:100`),
not a data folder.

Files the app writes under the home (partial; the full writer list was not walked): `setup/first-run.json`
(`first_run.go:221-249`); `settings.json` (`app:internal/slices/settings/store.go`); `pairings.json`, `phones.json`,
`logs/`, `worker/`, `projects/...` (joins of `Home` in `app:internal`); the sample project under `Grid/<sample>/`
(`app:internal/slices/setup/sample.go:82-83, 136`); a licence choice (`app:cmd/grid-server/licence_screen.go:340-346`).
It reads `worker-config.json` (`app:internal/kernel/worker/run.go:100-112`). Per `gridhome.go:192-193` it also writes
`project-roles.json`, `docs/` and `brain/` under `projects/`.

## 4. Proposed config

**Path.** `~/.grid/config.json` (on Windows `%USERPROFILE%\.grid\config.json`), found from the user's home folder with no
env. Reasons: both sides already default to `~/.grid`; the owner's older file is already there; the launcher already
lives at `~/.grid/bin/grid` and works from a Finder launch (finding 1). An OS config folder plus a pointer would add a
second file to keep in step. `~/.grid` becomes the machine folder; the data folder is either `~/.grid` itself (today's
layout, kept) or a folder the config names.

**Which files sit where.** Machine folder `~/.grid`: `config.json`, `bin/grid`. Data folder: everything else as today
(`VERSION`, `machine`, `accounts/registry.json`, `settings.json`, `projects/`, `knowledge/`, `personal/`,
`workflows.md`, `home-allow.jsonl`, `setup/first-run.json`, `pairings.json`, `phones.json`, `logs/`, `worker/`). Moving
`machine` and `accounts/` to the machine folder is out of scope (open item). The launcher lookup changes from
`<data>/bin/grid` to `~/.grid/bin/grid` first (`internal/hook/hook.go:161`, `app:.../vn_launcher.go:14-22`), because
with data in `~/.grid/<name>` the launcher is not under the data folder.

**Fields** (version 3 extends the Rust v2 shape, so the existing file reads without a rewrite):

```json
{
  "v": 3,
  "current": "<name>",
  "workspaces": [
    {
      "name": "<name>",
      "data": "/abs/data/folder",
      "brain": "/abs/brain/folder",
      "member": "<handle>",
      "folder": "/abs/repositories/folder",
      "setup": "done",
      "step": null
    }
  ],
  "choices": { "claudeSettings": "yes", "sandbox": false, "testDefault": "..." },
  "setupAt": "2026-11-17T00:00:00Z"
}
```

`data` empty means `~/.grid/<name>`; `brain` empty means `<data>` (the brain stays `<data>/knowledge` and
`<data>/projects/<p>/knowledge`; a separate brain folder is accepted but no lane needs one now); accounts are not copied
into the config: the account registry in the data folder stays their one record, and setup only registers the ones the
person kept. Unknown keys are written back as read (as both `config.rs` and `internal/settings/store.go:36-63` do).
`v` above 3 is read for the known keys and never rewritten by an older grid.

**Precedence for the data folder.** `--grid-home` > `GRID_HOME` > config (`current` entry's `data`, else
`~/.grid/<name>`) > legacy (v1 top-level keys, migrated in memory with the `config.rs:181-238` rules) > `~/.grid` itself.
For the member: `--member` > `GRID_WHO`/`GRID_ACTOR`/`GRID_MEMBER` > the current entry's `member` > none.
Each resolved value carries its source (`flag`, `env`, `config`, `legacy`, `default`) so `grid init --detect --json`
and `grid doctor` can say where it came from.

**Older file.** Read in place. `~/.grid/<name>` is found because the entry's data folder is `~/.grid/<name>`: no copying,
no move. The file is rewritten in the v3 shape only when setup saves (old keys dropped, as `config.rs:240-246`; the old
file is kept as `config.json.v2` first). If `~/.grid` itself also holds data (`VERSION` or `projects/`), the config wins
and `grid doctor` names both.

**Several workspaces.** `current` picks the one every command and hook uses; `GRID_HOME` or `--grid-home` picks another
for one session or command. Brains are not merged. Switching is `grid init --workspace <name>` (and the app's picker,
which today lists one row, `first_run.go:124-150`). Timers and env files baked with the old home
(`internal/routine/timer_plist.go:37`, `timer_systemd.go:28`) keep it until reinstalled; `grid doctor` names a timer
whose `GRID_HOME` differs from the resolved one. The session env file is rewritten at each SessionStart (`hook.go:160-169`).

## 5. The question set and defaults

`grid init --detect --json` fills each from the machine and marks it settled or asked:

| Question | Filled from | Asked when |
| --- | --- | --- |
| Data folder | `current` in `~/.grid/config.json`; each `workspaces[].data`; every `~/.grid/*` folder holding `projects/`, `knowledge/` or `VERSION`; `~/.grid` itself when it holds data. Default: the config's current, else the candidate with most projects, else a new `~/.grid` | More than one candidate, or none (then the default is offered once) |
| Where the brain lives | `<data>` | Not asked by default; shown as a settled line the person can open |
| Name in Grid (member) | The entry's `member`; else the account registry's `name` for the signed-in Claude account (`internal/account/registry.go:20-27`); else the OS user name; normalized to the app's handle rule (`first_run.go:176` `validName`) | Always shown once, prefilled, so the person confirms it |
| Agent accounts | `internal/account/account.go:115-149` folders; `claude` and `codex` on PATH and in the usual folders (`~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`, `~/.claude/local`); signed-in state as the app's agents check reads it | Shown as a list, all kept by default; asked only when one is found that is not in the registry |
| Claude Code settings entries | The `grid setup` preview (`internal/settings/claude.go:254`) | Asked yes/no with the preview, as `skills/setup/SKILL.md:40` already requires |
| Sandbox | Off | Not asked; `grid setup --sandbox` stays the way to add it |
| Test default, packs | The app's Plugin step (`first_run.go:31-33`) | Asked only in the app, as today |

Skipped on a single-person machine: members and approvers, the workspace picker when one data folder is found,
machine and phone pairing, notifications (the app's "set up later" steps, `logic.mjs:17-20`).

## 6. Lane plan

Plugin first, then app. No lane adds a guard, gate or block: a missing config is always an offer.

| Lane | Repo | Model / effort | Files | Acceptance rows | Depends on |
| --- | --- | --- | --- | --- | --- |
| P1-grid-config-read | plugin | opus, medium | new `internal/gridconfig` (read v1/v2/v3, write v3 by rename, unknown keys kept); `internal/gridhome/gridhome.go` (one `Resolve` with source; `Dir` kept as a wrapper or removed); callers `internal/cli/cli.go:114`, `internal/guard/settings.go:55`, `internal/guard/context.go:42`; launcher lookup `internal/hook/hook.go:161`, `internal/routine/launcher.go:20-21`; `internal/doctor` (source line, baked-timer line); `contract/` fixtures | (1) a fixture copied from the owner's Mac file (redacted) with data in `~/.grid/<name>` makes `grid brain topics` list its notes and `grid project list` its projects, with no copy; (2) `GRID_HOME` and `--grid-home` still win; (3) no config and no data gives `~/.grid` as today; (4) v1 top-level shape resolves by the `config.rs:181-238` rules; (5) `grep -rn 'gridhome.Dir(' internal cmd` lists only the resolver; (6) launcher found at `~/.grid/bin/grid` with data elsewhere; (7) walk tests `TestHsIpWalksEveryCommand`, `internal/job/lint` | none |
| P2-grid-init | plugin | opus, medium | `internal/cli` (new `grid init`: `--detect --json`, `--answers -`, `--workspace`; `grid setup` forms untouched); `internal/hook/hook.go` (SessionStart `systemMessage` plus context line when no config and no data); new `skills/first-run/SKILL.md` (AskUserQuestion, asks only gaps) | (1) `--detect --json` on fixtures prints each question with default and source; (2) `--answers -` writes `~/.grid/config.json` v3 and registers kept accounts; (3) a second run changes nothing; (4) SessionStart with no config prints the offer once and still exits 0 with context; with data found prints no offer; (5) `TestHsIpWalksEveryCommand` covers `grid init`; (6) `grid setup` and `grid setup <project>` behave as before | P1 |
| A1-app-config-read | app | opus, medium | one resolver in `app:internal/platform` used by `cmd/grid-desktop/main.go:35-42`, `api.go:356`, `cmd/grid-server/main.go:76,156`, `wire.go:1173-1175`, `cmd/grid-worker/main.go:93`, `internal/platform/crash/crash.go:17-19`, `internal/kernel/plugincli/vn_launcher.go` (launcher at `~/.grid/bin/grid`) | (1) the P1 fixtures (copied from the plugin `contract/`) resolve to the same folder and source; (2) a Finder-style launch (no `GRID_HOME`, bare PATH) serves `~/.grid/<name>` from the config; (3) one grep row: no other `GRID_HOME` read outside the resolver; (4) app walk tests | P1 |
| A2-app-first-open-config | app | opus, medium | `app:internal/slices/setup/first_run.go` (member from config, first-run.json as fallback); onboarding `logic.mjs`, `onboarding.mjs`, new `first-open.mjs`, `frame.mjs`; `plain.go` (after install, run `grid init --detect --json` and `--answers -`) | (1) with a complete config, opening Grid skips Start and lands on the first missing step; (2) with no config, Start shows the `--detect` questions prefilled; (3) answers saved through the plugin appear in `~/.grid/config.json`, and the desktop restarts its server on a new data folder; (4) the write list names `~/.grid/config.json`, not "config.json in Grid's app folder" (`logic.mjs:392`); (5) UI rows proven against the served `setup_status` | P2, A1 |
| P4-userconfig (optional) | plugin | sonnet, low | `.claude-plugin/plugin.json` (one optional `data_folder` directory option); P2's hook copies it into the config when set | Only if the owner wants a dialog in `/plugin`; first prove live that the dialog shows on enable (NOT VERIFIED today) | P2, owner decision |

**U-mac-first-open-setup (withdrawn).** Its partial work (uncommitted `frame.mjs` redirect to `#/onboarding` and
`pages/onboarding/first-open.mjs`: `isFirstOpen`, `readFirstOpen`, `toHandle`, `osUserOf`, `accountNameOf`,
`suggestName`) is replaced by A2: the first-open test becomes "the config has no current entry or no member", and the
handle suggestion comes from `grid init --detect` (one rule in the plugin) instead of a page-side rule. The `frame.mjs`
redirect shape can be reused in A2; the page-side name rules are dropped.

**C-mac-desktop-plugin-home (dd7eefdbd).** That commit is in its lane clone (`git log` there shows `dd7eefdbd` and
`19022c9fb` on `lane-C-mac-desktop-plugin-home`) but not in the app ref repo (`git show dd7eefdbd` there fails), so
whether it landed is not known here. Its retry of a failed person check stays as is. Its `plugincli.Client{Home: home}`
stays too, but A1 changes what it finds: the launcher at `~/.grid/bin/grid` (machine folder) and `home` from the shared
resolver, so a data folder in `~/.grid/<name>` does not lose the launcher.

## Open items
- The exact content of the owner's `~/.grid/config.json` (which shape, which keys): P1 row 1 takes a redacted copy.
- Whether `userConfig` shows a dialog on enable from a shell install or from the app: NOT VERIFIED; matters only for P4.
- Moving `machine` and `accounts/registry.json` to the machine folder: not proposed now.

DONE
