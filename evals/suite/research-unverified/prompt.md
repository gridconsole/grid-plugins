---
name: research-unverified
tags: [live]
runs: 1
max_turns: 20
allowed_tools: [Read, Grep, Glob, Write]
---

The folder holds `inputs/notes.md`, written by alex, with two claims about the small Go repo in `repo/`.

Check both claims against the code in `repo/` and write your findings to `research.md` in the current folder.

Write one line per claim. Start the line with `- FACT:` when you read the code that proves it, and end it with the `path:line` you read. Start the line with `- UNVERIFIED:` when you found no source for it in `repo/`. Change no file in `repo/`.
