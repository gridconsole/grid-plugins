# Task page data: Overview text, items, posts, check runs, proposals

The files behind the task page's status block and approach, item cards, threads, Result checks and a guest's proposals. They
sit in the task folder (`<projects folder>/<project>/tasks/T-NN-slug/`), never in a repo. Agents and skills write them; the app
and the Rust core read them and write only posts. This sits with the plan skill because items are the plan's steps.

## Overview text: two plan.md sections

The status block above every tab and the Overview's "How we approach it" read two optional sections of plan.md, in the shape the
plan skill's Format gives. The planner writes them with the plan. The hash covers `## How we approach it` like any other plan
text, so rewriting it is a `--bump`; it leaves `## Where it stands` out, so the planner keeps that current as the work moves
without a bump, and an approval stays.

```
## Where it stands
Two to four plain sentences: where the task is, what waits, what is next.
As of: <step>

## How we approach it
One or two sentences: the lead.

- **Card title**: one or two lines.

| Without | With |
|---|---|
| what happens today | what happens after |

Figure: views/approach.svg — caption text
```

- `## Where it stands`: the status block shows the sentences as written, while they are current. The last line, `As of: <step>`,
  names the workflow step they were written at (`closed` once the task is closed). The planner rewrites the section, and that line,
  at every step change: approve, build start, ship, close. The status block shows the sentences only while `As of` is the card's
  current step (or `closed` for a closed task); a section without the line counts only at the first step. Otherwise it falls back
  to task.md's `next`, as below.
- `## How we approach it`: the paragraph before the first card is the lead; each `- **Title**: text` line is a card (3 to 6); each
  row under `| Without | With |` is a before and after pair (3 to 6); `Figure: <path> — <caption>` names an SVG in the task's
  `views/` folder and its caption. The figure line is optional.
- Fallback when a section is missing, so older plans still show:
  - no `## How we approach it`: the lead comes from intent.md, the cards from the plan's steps, and there is no figure;
  - no `## Where it stands`: the status block is built from task.md's `summary`, its current `step` and `next`.

## Items: `items.md`

One block per plan step, beside plan.md, not inside it: plan.md's hash covers every step line, so a state that changes on each
turn would change the plan's rev and put the plan back at its Approve gate.

```
# Items

## i-4 · working · Price rules
why: the bulk prices are not written yet
waits: guest · Ava
ships-with: i-3
ship-approved: yes
```

- Heading `## i-<n> · <state> · <title>`: `n` is the plan step's number, the title its title.
- `state`: `open` (planned, not started), `working` (an agent is on it; the page says Building), `built`, `in-test`, `live`,
  `closed` (done or not needed; `why` says which).
- `why`: one sentence, why it is not done yet, or why it was closed.
- `waits`: who it waits on, `owner`, `team`, `guest` or `third-party`, then ` · <name>` when it is one person. Not a state.
- `ships-with`: the id of the item it goes live with.
- `ship-approved: yes`: someone pressed Ship this; the item goes live when the task lands.
- A step with no block is `open`. Lines other than these keys are kept out of the record.
- No items.md: the task read takes the plan's steps as the items, so every task shows its subtasks. Step `n` is `i-<n>`, its
  title the step line's text, its state `built` when the step is ticked, `working` when the brief that serves it (the rule
  below) has `status: running`, else `open`; no `why`, `waits` or ship fields. `brief`, `progress` and `checks` are computed
  for these items exactly as for written ones, so a check point with `item: i-<n>` lands on step `n`.
- items.md wins whole when it is there: a step it has no block for is `open`, not read from the plan. So the planner writes a
  block for every plan step, and writes the file at all only when a state, `why`, `waits` or ship field says more than the
  plan's tick does (see the plan skill).
- The planner keeps the file: it reads the item's `close`, `reopen`, `pickup` and `ship` posts and sets the state and fields. The
  core does not fold posts into items.

### What the task read adds to each item

A brief's `item:` line is the link from a brief to its items: `item: 4`, or `item: 4, 5`, names plan steps 4 and 5, so items
`i-4` and `i-5` (see the brief skill). The task read (the desktop's `task` command and `GET /api/tasks/<project>/<id>`) serves
`items`, `posts` and `checks` beside the card, and adds three fields to each item, computed from the files only. Each is left out
when it is not known.

- `brief`: `{stem, role, status}` of the newest brief (last in file-name order) whose `item:` lists the item's number; `stem` is
  the file name without `.md`, the name usage records the brief by. A step no `item:` line names falls back to the Plan tab's
  rule, over the briefs without an `item:` line: when the workflow has one step per plan step, the newest brief named for that
  step (`NN-<step>.md`); otherwise the brief whose file name starts with the step's number.
- `progress`: `{done, total}` over the items that brief serves (its `item:` list, else only this item). An item is done once its
  state is past `working` (`built`, `in-test`, `live` or `closed`); a step with no block is `open`.
- `checks`: `{ok, total}`, the item's points (`item: i-<n>`) in the newest check run that has any; `ok` counts the ones that `works`.
- An `open` item gets no `brief` or `progress`.
- Who works on it now, since when and at what cost is not here: the page joins those from the usage read by the brief's `stem`.

## Posts: the Feed, type `post`

A message on the task, an item or a question is a Feed post (see the feed skill): one JSON line in
`feed/<member>@<host>.jsonl`, the writer's own file, only ever appended.

```
{"id":"01JC...","task":"T-98","type":"post","type_v":1,"author":{"member":"ava","host":"mac","role":"guest"},
 "worker":null,"at":"2026-11-07T10:41:00.000Z-0000","re":null,"job":null,"epoch":0,
 "body":{"ref":"item:i-4","kind":"message","text":"Can the bulk price start at 10?","attachments":["attachments/posts/01JC.../shot.png"]}}
```

- `id` a ULID; `at` the Feed's clock, `<UTC time to the millisecond>Z-<4-digit counter>`, after the newest `at` in the folder.
- `author.member` is who wrote it, `author.role` one of `owner`, `member`, `guest`, `planner`, `agent`.
- `body.ref`: `task`, `item:i-<n>` or `question:Q-<n>`.
- `body.kind`: `message`, `pickup`, `ask-pickup`, `ship`, `close`, `reopen`. Every kind but `message` shows as a one-line event.
- `body.text`: at most 4000 characters; a `message` needs text, a `close` may carry the reason.
- `body.attachments`: paths relative to the task folder, under `attachments/posts/<post id>/`.
- A guest never posts `pickup` or `ship`; the core refuses them.
- The reader merges every `feed/*.jsonl`, keeps `type: post`, and orders by `at`, then `id`.
- The Feed lists `post` as a core type with a type note in `feed/types/post.md` (version 1, required `ref` and `kind`).

## Check runs: `checks.md`

Proof has one place: `checks/<run id>/` in the task folder. `checks.md` lists it, one block per run. `artifacts/` holds what the task
delivers, never its proof, and nothing is written into a project's repo.

```
# Checks

## R-2 · 2026-11-07T10:12 · partly · Test shop run: 5 work, 1 partly, 1 fails and is being fixed. · by grid-checker
- works · Search · The list narrows as you type · shot: checks/R-2/search.png · item: i-4
- works · Total · The total follows the price rule · shot: checks/R-2/total-after.png · before: checks/R-2/total-before.png · diff: checks/R-2/total.patch · output: checks/R-2/total-tests.txt · item: i-5
- partly · Checkout flow · Pay works, the receipt is late · video: checks/R-2/checkout.mp4 · gif: checks/R-2/checkout.gif · url: https://shop.test/checkout · item: i-6
- fails · Checkout · The Pay button does nothing · shot: checks/R-2/pay.png
- open · Mail · Not run yet
```

- Heading `## R-<n> · <YYYY-MM-DDTHH:MM> · <state> · <title>`, then optionally ` · by <agent>`: the title is the checker's one-line
  summary, and `by` is the agent that ran the check (its own name, such as `grid-checker`), which the Result tab shows as "Checked by". A
  heading without `by` is valid.
- One line per point: `- <state> · <title> · <text>`, then the optional keys, each ` · <key>: <value>`, in any order:

  | key | value | shows |
  |---|---|---|
  | `shot:` | a path under `checks/<run id>/` | the screenshot; the after, when `before:` is given |
  | `before:` | a path | the screenshot before the change |
  | `video:` | a path, `.mp4` or `.webm` | a recording of the flow |
  | `gif:` | a path | a short loop |
  | `diff:` | a path, `.patch` or `.diff` | the change as a diff |
  | `output:` | a path to a text log | the test or command output; the app shows its last 20 lines |
  | `url:` | `http://` or `https://` | the live page |
  | `item:` | `i-<n>` | the item the point proves |

- A path is relative to the task folder, starts with `checks/`, has no `..`, no segment that starts with a dot, no backslash and no
  empty part. The reader drops a path that breaks this, and a `url:` that is not http or https; the point stays. A point with none
  of the keys is the old line, read as before.
- `state` of a run and of a point: `works`, `partly`, `fails`, `open`.
- The reader lists runs newest first, by the number in the id.

## Proposals: a proposed task

A guest's "Something new" is a task folder like any other, with `status: proposed` and `proposed_by: <guest>` in task.md, and the
guest's words, as typed, in intent.md. The owner starts it or declines it on that task's page.
