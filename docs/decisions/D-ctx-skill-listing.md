# D-ctx-skill-listing: agents Grid launches list only the skills their Job needs

Base: plugin main `6cc26809`. Paths are repo-relative.

## The problem

Every skill in Claude Code's listing adds its description to the context of every turn (Claude Code docs, skills page: "Every skill in the skill listing adds to your context on every turn, whether or not Claude ever uses it"). A Grid agent session carries the person's own skills, Claude Code's bundled skills and the plugin's skills. The orchestrator measured about 27k characters of listing per session. The owner wants the listing cut through the plugin, so an install on another computer gets the cut too, without a Grid-made limit on agents.

## What Claude Code offers (docs read 16 Nov 2026: code.claude.com/docs/en/skills, cli-reference)

- `skillOverrides` is a settings key that maps a skill name to `on`, `name-only`, `user-invocable-only` or `off`. `name-only` keeps the skill callable and lists its name without its description. It matches user, project and local skills by name only. Plugin skills are not affected by it.
- `disableBundledSkills: true` turns Claude Code's bundled skills off.
- `claude --settings <file or inline JSON>` lays settings over the person's `settings.json` files for that session only. Keys it omits keep their file values.
- A plugin cannot ship settings. Frontmatter `disable-model-invocation: true` keeps a skill's description out of context, but Claude then cannot load the skill on its own, and it is meant for skills called only as `/name`. Frontmatter `paths:` limits when a skill loads automatically, by file globs. The docs do not say that it shortens the listing.

## Decision

1. **Grid-launched agents get a per-run settings layer, and the person's sessions are never touched.** Every `claude` run that the plugin starts gets `--settings` with inline JSON. The layer sets `disableBundledSkills: true`. It also gives each of the person's own skills `skillOverrides: "name-only"`. Those are the folders under `<home>/.claude/skills` that hold a SKILL.md, the files `<home>/.claude/commands/*.md`, and the skills in `.claude/skills` of the folder the agent starts in. A skill the Job or workflow step names stays fully listed: `SkillLayer(home, dir, keep)` leaves the names in `keep` out of the overrides, so they read as `on`. `grid:` is stripped first. No settings file is written, in the person's home or in the Grid home, so the person's interactive sessions keep their full listing. Nothing is turned `off`: an agent can still call a name-only skill by name. This is a Claude Code built-in. It is not a Grid cap, gate or limit.
2. **What the plugin launches today: skill routines** (`internal/routine/execute.go:201-210`, `internal/routine/skill_layer.go`). A skill routine runs `grid:<skill>`, which is a plugin skill and is not touched by the overrides, so its `keep` is empty. The layer is added whenever the executor has the person's Home, as `grid routine run` always passes it (`internal/cli/routine.go:185`). Without a Home the agent starts with an empty HOME and has no person's settings to lay a layer over, so its argv stays the one `TestProcessRoutineLimits` pins. That follows the `tools` precedent (lane P-r23-signals-routine): a routine without the new input keeps its argv.
3. **Workflow routines** run `grid task new ... --workflow`, not claude. The Jobs of that task are started by the session or app that runs them, not by the plugin. When the plugin or the app starts a Job's agent, it should pass the same layer with `keep` set to the skills the Job names: the Job's `Skills:` line, which the workflow step's brain note gives it. That launch side is not in the plugin today, so it is not built here.
4. **Plugin skills.** `skillOverrides` cannot reach them, so they are cut by their own descriptions. The person-facing skills setup, task, questions and signals keep a description of at most 200 bytes, held by `TestCtxPersonSkillDescriptionsShort`. Each keeps what the skill does and the words that call it. None of them gets `disable-model-invocation`. The person calls setup and task in plain words ("set up Grid", "start a task"), not only as `/name`. Agents use questions (`skills/research/SKILL.md:22`). Routines run signals by asking Claude to load it, and that would stop working. None gets `paths:` either: none of them is tied to a file pattern, and the docs do not say that `paths:` shortens the listing.

## Not decided here

- Measuring the real listing size of a routine run needs a live `claude` and was not done.
- A person may already have set one of their own skills to `off` or `user-invocable-only` in their settings. The layer's `name-only` for that name may then list it again in a routine run. The docs read here do not say whether `--settings` merges `skillOverrides` name by name or replaces the whole map. A follow-up can carry the person's stricter value into the layer.
- On Windows a `claude.cmd` shim may quote the inline JSON differently from `claude.exe`. That was not tried. If it proves a problem, the same layer can go in a temporary file whose path is passed to `--settings`.
