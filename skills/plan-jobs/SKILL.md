---
name: plan-jobs
description: How to write one Grid Job so it works as the brief: fields, acceptance rows, and `after` dependencies. Use when posting or planning Jobs for a task.
---

A Job is the brief. A builder or reviewer reads only the Job, so it must stand alone.

Fields (see docs/job-contract.md in the plugin repo for the full list): `id`, `item`, `title`, `role`, `intent` (with `owner`), `read`, `write`, `out_of_scope`, `acceptance`, `verify`, `budget`, `on_blocked`, `after`.

- `intent`: the goal in a sentence or two.
- `write`: the only files the Job may change; `read`: what to read first.
- `acceptance`: numbered rows, each checkable by a reviewer from files or command output.
- `verify`: the test command and a positive `min_tests`; for research or non-code work, `kind: file-exists` or `kind: reviewer-check`.
- `after`: bare Job ids that must be done first, or objects with `job`, `kind` (`done`, `failed`, `children-done`) and `why` (8 to 160 characters). A gate is `gate:Q-7`. A review Job names the build Job it checks.
- A Job for a person has role Person; its intent says what to decide and what the options are.

Check before posting: `grid job lint <file> --task <task>`; for several Jobs `grid job cycles <items.json>` and `grid job floor <items.json>`; for a plan `grid plan check`.
