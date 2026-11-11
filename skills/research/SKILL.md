---
name: research
description: Find facts for a Grid task before it is planned or built, and write them where the plan can cite them. Use when a task needs to know how something works, where it is in the code, or what the options are ("research X", "where is Z handled", "look into it first").
---

Facts first, then the plan. Research changes no code and asks the person nothing it can read for itself.

Rules that apply here:
- "Every code fact has a path:line read that session": a fact from memory or an earlier session is a guess; read the file again and cite `path:line`.
- "No Grid-made guards, caps or limits": use the Claude Code built-ins for limits and permissions.
- "No plan mode."
- "Everything is a Job, the Job is the brief": research is a Job (`grid job show`, `grid job result`) with one question; a wider ask is split into one Job per question before launch.
- "Split a step before launch if it will not fit in about 60 calls."
- "Every link carries a why."

Steps:

1. Write the question as one sentence, from `grid job show` or the task intent. If it holds two questions, split it.
2. Look in the brain first: `grid brain search <words> --task <task>` and `grid brain show <id>`. Cite a note by id.
3. Read the sources. For code, read the file and note `path:line`. For a Grid verb, run `grid <verb> --help` and cite what it printed. For the web, name the URL and the date you read it.
4. Write `research.md` in the task folder (`grid where` shows it), one section per question: the answer in one line, then each fact with its source, then what you could not find. A fact with no source is marked UNVERIFIED.
5. If a finding makes the ask unclear, put one question through the questions skill; otherwise carry on.
6. Post the result: `grid job result --task <task> --job <job> --body <json>` naming `research.md` and listing what is UNVERIFIED. If a fact would help other tasks, say so in the result; the deliver step proposes brain notes.

Done looks like: `research.md` exists, every code fact has a `path:line`, every unverified fact is marked, and no project file changed.
