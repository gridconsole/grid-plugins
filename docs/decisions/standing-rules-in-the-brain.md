# Standing rules live in the brain; a Job names them

Status: decision (owner request 16 Nov: cut session usage through the plugin, so every install gets it).
Code facts are read at the lane base; paths are plugin paths.

## Why

Sessions spend most of their context on text they did not need: whole files, full-page screenshots, and briefs that paste the same project rules into every Job. Few sessions read the brain on purpose. A rule pasted into a Job goes stale when the rule changes, is paid for in every Job that copies it, and hides which part mattered. A rule kept in one note is written once, found by its description, and read only by the worker that needs it.

## What a standing rule is, and where it goes

A standing rule holds beyond one task. Four kinds:

| kind | example shape | topic |
| --- | --- | --- |
| project convention | how files, tests or commits are written here | `conventions` (project) |
| removal list | names that were removed and must not come back | `conventions` (project), or `decisions` when the removal was a choice with a reason |
| never-rule | a thing no worker does in this project | `conventions` (project); `decisions` at home level, where `conventions` is not available |
| design lead | the layout or visual rule a UI follows | `conventions` (project) |

One note per rule. Its `description` says what work it applies to (that line is what the index, search and a planner see); its body holds the rule and why. No new topic: `conventions` and `decisions` are built in (internal/brain/topics.go:25), and a shipped topic would add a row to the SessionStart index of every session (internal/brain/index.go:175-205). The plugin ships no rule text; each project writes its own notes.

## How a Job points at a rule

The Job contract already carries note ids; no field is added.

- `keep`: ids of notes the worker must hold to (a convention, a never-rule, a design lead).
- `decisions`: ids an acceptance row traces to or drops a requirement by.
- `links`: the same id with `rel: depends-on` or `decided-by` and a one-line why that says what the note rules in this Job. A `keep` or `decisions` id with no link gets a generic reason in the brief (internal/job/render.go:174-179).
- A removal list in a note does not replace plan-jobs check 1: the Job still lists the items it removes, one acceptance row each.
- The planner finds the notes with `grid brain search` or by asking `grid:library` (agents/library.md, step 5), which answers with note ids and one line each, never bodies.
- `grid job lint` warns `job.pasted-rule` (internal/job/lint/pasted_rule.go) when the title, an acceptance row, a link or after why, or review_reason copies a sentence (six words or more) of a note the Job names. It is a warning: the Job still posts, renders and runs. `job.brain-id` already refuses copied rule text in place of an id in keep or decisions (internal/job/lint/lint.go:210).

## What a worker reads from the brain

- The SessionStart index: one row per topic (name, one-line description, entry count, last change, invalid count) and the read step. It never prints a note body or summary (internal/hook/hook.go:146-152 passes only `Index().Text`; internal/hook/sessionstart_index_only_test.go pins it).
- The notes its Job names: the brief's Context shows each with its pointer, summary and an excerpt, in a budget of 2,000 tokens and 8 entries (internal/job/render.go:229-256, :591-597); the ones cut are listed in `not_rendered` of the Job's inputs.json. The worker reads one whole with `grid brain show <id>` when the excerpt is not enough.
- Nothing else by default. A broader question goes to `grid:library` (Haiku), which answers in a few lines with note ids.

## What this does not change

- The render keeps its excerpts and its one-hop traversal from named notes over `depends-on` and `decided-by` links (internal/job/render.go:195-209).
- Recall still adds up to 8 notes matched by the Job's title and intent words (internal/job/jobcmd/recall.go:23-45, ranked last at internal/job/render.go:210-214). These are notes the planner did not name; they share the same brief budget. Whether recall should become pointer-only or be dropped when a Job names its notes is an open question for the owner, not settled here.
- No guard, gate, cap or hold is added on agents; the lint finding is a warning to the planner.
