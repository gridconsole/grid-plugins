---
name: brain-keeper
description: How a proposal becomes a linked note in the Grid brain, which topic it goes in, and the quality bar for a note, a skill and an agent, and it asks no person. Use when you learn something worth keeping, when `grid brain keep` lists waiting proposals, or when you write a skill or agent for the brain.
---

The brain is the knowledge of a project (`knowledge/` beside its tasks) and of the Grid home. This skill guards what enters it, where it goes, and how good it is.

Owner decision: the brain needs no attention from the person. A subagent curates proposals and never asks the person. Do not ask whether to keep a note and do not wait for an answer: accept it, merge it, or reject it with a reason.

## Propose

1. Look first: `grid brain search <words>`. If a note already covers it, propose a link to it or a note that continues it, not a copy.
2. Pick the topic (see Where it goes); `grid brain topics` lists them.
3. Write it:

```
grid brain propose --project <project> --name <id> --topic <topic> --description "<when to read it>" --summary "<what it holds>" --source <task, Job or person> --body "<text>" --link '{"to":"brain:project/<note>","rel":"explains","summary":"<what that note holds>","why":"<what the reader gains by following it>"}'
```

Always pass `--project <project>` when the person or routine names one: from a folder that is not that project the proposal resolves to home and a `conventions` note fails with `topic conventions is not available at home level`. `grid brain accept` and `grid brain check` take `--project` too. A proposal made this way waits in the project's `brain-proposals`: file it with `grid brain accept --project <project> <id>`; `grid brain decide` is for a proposal in a task's own folder.

`--file <path|->` takes Markdown with front matter instead of `--body`. `--level home` is for what is true in every project; without `--level` a resolved project is used, else home. `--external` quarantines text that came from outside.

## Where it goes

- A choice and its reason: topic `decisions` (home or project).
- A project practice or a lesson: topic `conventions` (project).
- A kind of knowledge that repeats and fits neither: a custom topic, defined by `knowledge/<topic>/TOPIC.md` in the brain with `name`, `kind: knowledge`, `level` and `description` (internal/brain/topics.go:66).
- `environments`, `machines`, `deploy`, `people`, `agents`, `routines` are config topics, not notes. `grid brain check` flags a proposal there as `config: <step>`, and the finding names the step to take.
- `skills`, `workflows`, `design`, `research`, `incidents` are not topics (internal/brain/topics.go:63). A skill lives at `knowledge/skills/<name>/SKILL.md`.

Topics are hooked, not dumped (owner decision): the SessionStart index, which carries one line per note (internal/hook/hook.go), the CLI (`grid brain search --topic <t> <words>`, `grid brain show <id>`), a skill's front-matter description and the brain links of a Job. No hook puts a topic in front of an agent when it starts a skill or touches a file. So the `description` is the hook: it says when to read the note. Never paste one note's body into another; link to it.

## Keep

The keeper is a subagent. It never asks the person.

1. `grid brain keep --task <task>` lists the waiting proposals and writes nothing.
2. `grid brain show <id> --proposal` reads one.
3. Run `grid brain check <id>` and judge the findings: exit 0 clean, 3 findings. A finding means the text changes what runs (a config topic, a skill key that can run code, a shell line, an agent field). When a finding is real, reject with the finding code as the reason. No Grid-made guard, cap or hold is added on top.
4. Judge it against the bar below.
5. Pass: `grid brain accept <id>`. For a task's proposal: `grid brain decide <task> <id> accept --note "<reason>"`.
6. Duplicate or adds to a note: `grid brain decide <task> <id> merge-into <note-id> --note "<reason>"`.
7. Fail: `grid brain accept <id> --reject "<why>"`. A proposal is filed as written and cannot be edited, so if the idea is worth keeping, propose the fixed version under a new name and put the rejected name in `--source`.
8. After a batch: `grid brain index`. `grid brain changes` lists what changed; `grid brain revert <topic>/<name> --to <n>` publishes an older version again.

## Prune

A note that has gone stale is retired, never deleted. The old note stays readable with `grid brain show`; search no longer lists it.

1. On each proposal, run `grid brain search "<its words>"` for an older note on the same fact.
2. The newer fact wins: the proposal carries a `supersedes` link to the old note with a why, then `grid brain accept --project <project> <id>`. Accepting it sets `superseded_by` on the old note.
3. An existing note made stale by another existing note: `grid brain supersede <old> --by <new> --reason "<why>"`. The reason is 8 to 160 characters and more than one word.
4. A bad accept or a wrong supersede is put back with `grid brain revert <topic>/<name> --to <n> --reason "<why>"`. `grid brain changes` shows the reason.
5. Never delete a note and never edit its file by hand: use the verbs above.

```text
grid brain search "deploy pipeline" --project <project>
grid brain supersede <old> --by <new> --reason "the new pipeline replaced the old one" --project <project>
grid brain revert <topic>/<old> --to <n> --reason "the old pipeline is still in use" --project <project>
grid brain changes --project <project>
```

## Quality bar

A note:
- One fact or decision. `description`: when to read it, one line, at most 200 characters. `summary`: what it holds, at most 300 characters. `source`: the task, Job or person.
- Body at most 60 lines (300 is the limit; over 60 needs a `## Contents` list).
- Every code fact carries the `path:line` read in that session. A fact not read is not written.
- No secrets (the store refuses them), no task chatter, no dated event in a person note.

A link: `to` is `brain:<level>/<id>`, `job:<task>/<job>` or `person:<id>`. `rel` is one of depends-on, explains, decided-by, supersedes, duplicates, related, blocks, continues, validates, same-files, uses, owns, decides, approves, knows. `why` is 8 to 160 characters and says what the reader gains by following the link. Every link carries a why. "See also", "related" and the target's own name are refused. A note with a related note and no link is incomplete.

A skill:
- Front matter has only `name`, `description`, `when_to_use`, `metadata`. Any other key can change what runs and is a finding.
- The description says what it is for and when it triggers. The steps use exact `grid` verbs, each checked with `grid <verb> --help` first.
- Only `SKILL.md` and `reference/*.md`. No executable file, `bin/`, `hooks/`, `.mcp.json`, symlink, or line that starts a shell command.
- Short and plain, no emojis.

An agent:
- What it is for, when it triggers, the exact `grid` verbs, what done looks like. Short and plain, no emojis.
- It adds no guard, gate, cap or hold of its own; limits come from Claude Code's built-ins, and approvals and gates come from the workflow in the brain with a default.
- Its model and effort are set with `grid agent`. An agent proposed through the brain is a config entry (topic `agents`), not a note, and `grid brain check` reads it.

## Bash in a Job

- Send one command per Bash call: no `;`, no `&&`, no pipe, no `$(...)` and no `echo $?`. The exit code is in the tool result.
- Never set `dangerouslyDisableSandbox`.

## Done

`grid brain keep` shows no waiting proposal for the task, each one is accepted, merged or rejected with a reason, and `grid brain check` exits 0. Reply in at most 20 lines: each proposal and what happened to it.
