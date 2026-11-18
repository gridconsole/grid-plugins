---
name: first-run
description: Set Grid up on a new computer: detect the answers, ask only what could not be detected, and write the one config. Use when SessionStart offers a first run, or the person says "set up Grid on this computer" or "first run".
---

# First run

Grid has no config on this computer yet. Detect first, ask only the gaps, write once. Grid does the write; you never hand-edit the config or the Grid home.

## 1. Detect

Run `grid init --detect --json`. It prints `questions`, each with `id`, `question`, `default`, `source` (where the default came from) and `asked`. If `found` is true and `problem` is empty, a config already exists: say so and stop, unless the person asked to change it.

## 2. Ask only the questions marked asked

For each question with `asked: true`, and no other, ask with AskUserQuestion. Offer the `default` first, marked as the recommendation, and say its `source` in the option text. Questions with `asked: false` keep their default and are not shown.

- `data`: the data folder, when more than one was found.
- `member`: the name in Grid, prefilled.
- `accounts`: the agent accounts found; all are kept unless the person drops one.
- `claudeSettings`: yes or no. Before asking, run `grid setup` to show the person the preview of the Claude Code settings entries; it asks before it writes.

Never ask about a budget, a spend limit or a cap; those are Claude Code's own settings.

## 3. Write the answers

Build one JSON object with only the keys the person changed from the default (`workspace`, `data`, `member`, `accounts`, `claudeSettings`). A key left out keeps its detected default. Write it with Write to a file outside the repo and run `grid init --answers <file>`. The command validates before it writes; fix what it names and run it again. A second run with the same answers changes nothing.

## Done

The config is written and `grid init --answers <file>` exited 0. The last message gives the config path, the data folder and the name in Grid, from the command's own output. Nothing was written in the person's repo. If the person declines, the session goes on as before; a decline is not an error.
