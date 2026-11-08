---
name: plan
description: Write or revise a task's plan.md, or tick its steps. Use when the person asks to "plan it", "write the plan", "revise the plan", "add a step", or when a plan step is finished and needs ticking. The plan explains where things stand, the options and the choice, the risks, then the checklist of steps with a revision line; research comes first unless the task is Small.
---

# Plan

`plan.md` sits in the task folder (`<projects folder>/<project>/tasks/T-NN-slug/`; the projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects`, and `GRID_HOME` is the Grid data folder `~/.grid/<name>`, never the workspace folder; if both are empty, Grid is not set up for this folder: say so and write nothing). If the person does not say which task, take the only open one, else ask. The planner is its only writer. Read `intent.md`, `task.md` and every file in `attachments/` and `inputs/` first, by the task skill's Attachments rule; the plan answers the intent in the person's words, not yours.

## Research first

A person reads the plan to decide whether to let the work go ahead, so it must show what is true today, not what you guess. Unless `size` in task.md is `small`, get the facts before you write a line of the plan:

1. Run the research skill: `grid-scout` maps the files when they are not obvious, then `grid-researcher` writes `research.md` in the task folder, each finding with its source.
2. Read `research.md` and plan from it. The plan cites it by section (`research.md, "Store"`); a fact you did not find stays out, or becomes a question.

If `research.md` already exists and still answers the intent, use it; research again only for what it does not cover. A Small task may skip research and read the code itself.

## Format

```
# Plan, rev 1 · 3f9a1c

## Where it stands
Research is done and the plan waits for your approval. Nothing is built yet. Once you approve, step 1 adds the filter to the store.
As of: plan

## How we approach it
Notes already carry tags, so one filter goes in the store, where the list and the coming search both call it.

- **One filter in the store**: it ignores case and is tested on its own.
- **A tag box on the list**: the list narrows as you type.
- **Tests for both**: the filter alone, then the list through it.

| Without | With |
|---|---|
| Every note shows, whatever its tags. | Typing a tag narrows the list. |
| "Home" and "home" are two tags. | Case is ignored. |
| Search would need a filter of its own. | Search reuses the store's filter. |

Figure: views/approach.svg — the list and search both call one filter in the store

## Where things stand now

Notes keep their tags in `src/store.js:3`, but nothing filters by them (research.md, "Store"). The list view renders every note on each keystroke (research.md, "List view").

## Approach and options

- A filter in the store: one place, tested on its own; the list and a later search reuse it.
- A filter in the list view: fewer files, but it can only be tested through the view.

Chosen: the store, because search is the next task and will need the same filter.

## Risks

- medium: tags differ in case, so "Home" misses "home". Retired by step 1.
- low: a long tag list crowds the toolbar. Retired by step 2.

## Steps

- [ ] 1 Add a tag filter function to the store
  The list and search both need it, so it lives in one tested place and ignores case.
  verify: `node --test` passes for the filter
  needs: none
- [ ] 2 Add the tag box to the list view
  People asked to narrow the list as they type; the box calls the store's filter.
  verify: typing narrows the list
  needs: 1
- [ ] 3 Write the release note for the new filter
  Support asked for it; it only needs the decision, not the code.
  verify: the note names the filter and how it treats case
  needs: none
- [ ] 4 Cover both with tests
  verify: `node --test` passes
  needs: 1, 2

Verify: `node --test` passes and the list narrows by tag
```

- Line 1 is the revision line. Never type it by hand; the script writes it (below).
- `## Where it stands` and `## How we approach it` are optional at every size; they come first, in this order and this shape, because the task page's status block and Overview read them. Without them the page falls back (`task-page-data.md`, "Overview text"). The hash leaves `## Where it stands` out, so keep it current as the work moves: rewriting it keeps rev and hash, needs no `--bump` and keeps an approval. `## How we approach it` is plan text the hash covers: rewriting it is a revision (`--bump`) and voids an earlier approval.
- `## Where it stands`: two to four plain sentences for the status block above every tab: where the task is, what waits and on whom, what comes next. Words a guest understands, no file paths. It is not `## Where things stand now`, which holds the findings with their sources. Its last line is `As of: <step>`, the workflow step it was written at (`closed` once closed). Rewrite the section and that line at every step change (the approve, the build start, each later step, the ship and the close), in the same turn as the task.md change: the status block hides sentences from an earlier step, so a section left behind shows nothing of yours.
- `## How we approach it`: the explanation for someone who did not read the research.
  - The lead: one or two sentences, the idea of the whole approach.
  - 3 to 6 cards, `- **Card title**: one or two lines.`: each a part of the approach a reader can point at, never a step list copied.
  - 3 to 6 rows under `| Without | With |`: each a concrete outcome before and after, in plain words, not file or hook names.
  - Optional last line `Figure: views/approach.svg — <caption>`: the caption says in one sentence what the drawing shows. Draw it in the task's `views/` folder: the parts, arrows labelled with what moves, old parts dashed, the new part in the accent colour. It must read at 400px: a `viewBox` about 360 wide, text 12 to 13px (11px for arrow labels), top to bottom when it has more than three parts. A standalone file: `role="img"` and an `aria-label` that says what the caption says; lines and text in `currentColor`, the new part's outline and tint in the accent `#2bd7f2`, all as attributes; no `<script>`, `<style>`, `style=`, `<foreignObject>` or outside references.
- `## Where things stand now`: the findings that shape the plan, each with its source (a `file:line`, a `research.md` section, a link). Short paragraphs or bullets; a reader who skips the code still knows the situation.
- Sources (a last bullet list inside `## Where things stand now`, not a heading): every file read from `attachments/` and `inputs/` as `attachments/<file>` or `inputs/<file>`; a file that could not be read is listed as unread, with the reason.
- `## Approach and options`: two or three real options, each with its trade-off, then `Chosen:` with the reason. A choice that is the person's to make is an open question (below); name its `Q-n` here with the option you recommend.
- `## Risks`: one bullet each, `- <high|medium|low>: <the risk>. Retired by step <n>.` A risk that no step retires needs a step, or a question.
- `## Architecture` and `## Effort` are optional: add them when the shape of the change or its hours are a decision the person makes.
- `## Steps`, then the steps: `- [ ] <n> <title>`, numbered from 1, each a change someone can check. Under the title, indented, a short paragraph (one or two lines) saying what the step does and why; then the `verify:` line saying how its result is proved, then the `needs:` line. The paragraph comes before `verify:`, and no blank line falls inside a step.
- `needs: <n>, <n>` lists the step numbers whose output this step truly uses (its code, its file, its decision); `needs: none` says it uses nothing from another step. Every step has the line, and it never leans on list order: step 3 does not need step 2 because it comes after it. Write only direct needs (a step that needs 2, which needs 1, writes `needs: 2`). Grid runs every step whose needs are ticked at once (the execute skill), so each missing line costs time and each invented one holds work back; ask of every pair "does this one read what the other makes?".
- Steps that change the same files must not run together. Say so in the later step's paragraph ("changes the same files as step 3") and the execute skill runs them one after the other; add the step to `needs:` only when it also builds on the other's output.
- The last line is `Verify:`, the check for the whole task. Nothing follows it: put assumptions under `## Where things stand now`, never in a block after it.
- Grid reads these headings to lay out the Plan tab, so keep their names exactly. Never write an `Open questions:` paragraph; questions live in `questions/`.
- The steps are the plan; the task's `workflow` in task.md says how the work moves, and is not repeated here. Steps run on their own until a workflow step that needs a person, so a step never says "check with the person"; that is a question.
- The plan may show that the workflow is wrong: a deploy needs a `schedule` step, the scope needs an `approve`. Then update `workflow` in task.md in the same shape (`  - {step: <name>, kind: <kind>}`, kinds from the task skill: check, ask, approve, make-together, run, schedule) and keep `step` on one of its names.

## Depth by size

`size` in task.md sets how much the plan says:

| size | research | sections | steps |
|------|----------|----------|-------|
| `small` | optional | `## Where things stand now` in one or two lines; options and risks only when there is a real choice or risk | 1 to 5 under `## Steps` |
| `normal` | required | where things stand, approach and options, risks | 3 to 9 under `## Steps` |
| `large` | required | the Normal sections, plus `## Architecture` and `## Effort` | in chapters: one `## <chapter name>` heading per chapter instead of `## Steps`, 3 to 9 steps each, numbered on across chapters |

## Open decisions

Every decision the person has to make goes to them through the questions skill, as a block in `questions/<person>.md`, never only in the plan or on its page. The plan names it (`Q-2, recommended: the store`) under `## Approach and options`. When the work cannot go on without the answer, ask it as `blocking`; its `Q-n` goes into task.md's `needs` and `next` only after the review (After writing, step 5).

## Revision line

Write or revise plan.md with the Write or Edit tool, never a shell heredoc or redirect (Read a file in this session before you Edit it; a subagent's read does not count); then run:

```
node "${CLAUDE_SKILL_DIR}/scripts/plan-hash.mjs" "<absolute task folder>/plan.md"          # new plan, or only boxes ticked
node "${CLAUDE_SKILL_DIR}/scripts/plan-hash.mjs" "<absolute task folder>/plan.md" --bump   # you changed the plan
```

Run it as a single command on its own: the absolute path to plan.md, no `cd`, `&&`, `$PWD` or anything chained before or after, so a person's allow rule for `node` matches it.

It writes line 1 as `# Plan, rev <n> · <hash>`. The hash is the first 6 hex digits of the SHA-256 of the file without line 1, with the `## Where it stands` section left out (its heading up to the next `## ` heading) except any unindented step line written there and the indented lines under it, which still count, every checkbox read as unticked, every `evidence:` and `needs:` line under a step left out, and trailing spaces cut. An evidence or needs line is an indented line that starts with `evidence:` or `needs:`, in the lines right under a `- [ ]` or `- [x]` step up to the next blank or unindented line. So ticking a step, adding its evidence or changing its `needs:` does not change rev or hash; changing a step's words, order or `verify:`, or any section above the steps, does (so does an `evidence:` or `needs:` line that is not indented under a step), and then you must use `--bump`. A new plan is rev 1. An approval names a rev and a hash, so a bump voids any earlier approval; say so to the person.

## Items

The task page shows the plan's steps as subtasks, with their state, read from `items.md` beside plan.md (`task-page-data.md`, "Items"). Write it with the plan and keep it moving, through the script, as a single command on its own like plan-hash:

```
node "${CLAUDE_SKILL_DIR}/scripts/items-sync.mjs" "<absolute task folder>/items.md"
```

- Run it after every plan write or bump, after every tick, and when you start or end a brief that has `item:`. It writes one block per step, keeps every state and line you wrote, moves a ticked `open` or `working` step to `built` and an `open` step whose brief runs to `working`, takes the titles from the plan and drops blocks of steps the plan no longer has.
- The rest is yours, with the Edit tool on that block: `in-test` when a check run starts on the item, `live` when it ships, `closed` with a `why` when it is done or not needed, `why` for anything not done yet, `waits` when the item waits on a person, `ships-with` and `ship-approved: yes` from the item's `ship` posts. Read the item's `close`, `reopen`, `pickup` and `ship` posts on every turn and set these.
- items.md is not part of the hash, so it never needs a `--bump`.

## After writing

1. plan.md carries its brain recall under `## How we approach it`: one `Used: <path>` line for each recalled note the plan uses, or `Recall: nothing found` (the plugin puts the recall hits in your context when planning starts; `grid-brain recall --task T-NN` widens it). Without one of these lines the plan ready line below is refused.
   Append to `history.md` with the Edit tool, never a shell `>>`: `- <time> · plan ready · rev <n> · <hash> · by grid-planner` (use the numbers the script printed; `<time>` is the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess). Only a new plan or a bump needs this line.
2. Write every open decision (above) as its `## Q-n` block in `questions/<person>.md` through the questions skill, with its history line, but leave task.md for step 5, so the reviewer sees the questions and the task does not wait before the review.
3. Write `views/plan.html` through the plan-page skill, in the same turn, and again after every revision. It shows what research found, the options and each step's reason as well as the steps. A plan without its page is not ready. Links from the page at `views/plan.html` to task files need `../` (the page sits one folder below the task folder), for example `../research.md`.
4. Before you set `status: waiting` with a new or bumped plan, start `grid-reviewer` on the plan and its page, as the execute skill does for every approve step: the plan has no worker brief, so ask `grid-brief-writer` for a brief with role `reviewer`, start the reviewer on it, and wait for its Verdict. Fix what it must-fixes, bump (with its history line) and rewrite the page.
5. Last, in task.md: add the `Q-n` of each blocking question to `needs`, set `status: waiting`, `updated` (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess), `next` (for example "The person reads the plan and accepts it", or "Alex answers Q-2" when a blocking question is open), and add a body line `- <D Mon> · <step> · plan written, rev <n>`. Leave `step` where it is; the person moves the task on by accepting, because Grid moves `step` off an approve step when the person approves and no skill does.
6. Never write `plan accepted`. A person accepts the plan in Grid; you stop at "plan ready". The plan is this file and its history line, never a tool call: plan in normal mode, and once task.md is written, end your turn. Grid holds the plan at the gate and starts a new turn on this session when the person approves it or sends it back; the resume skill carries on from there.

## Ticking a step

Tick `- [x]` only with evidence the step is done: a command's output saved under `checks/<run id>/` (for example `checks/R-1/store-tests.txt`, the `output:` of a point with `item: i-<n>` in `checks.md`; see `task-page-data.md`), a file you can name, or a reviewer's Verdict. Add the path under the step, after its `verify:` line, as `  evidence: checks/R-1/store-tests.txt`. No evidence, no tick. A step you started but cannot prove stays `[ ]`. Do not run the plan-hash script for a tick or an evidence line; the hash ignores both. Do run items-sync (above), so the subtask reads `built`. Then set `updated` in task.md (the output of `date +%Y-%m-%dT%H:%M`, run just before this edit, never a guess) and add a body line.

## Revising

A plan without `needs:` lines (written before they existed) is read as "each step needs the one before it", so nothing in it runs together. Add the lines to every step when you next touch the plan; they are not part of the hash, so adding them alone needs no `--bump` and keeps an approval the person already gave (run the script without a flag: rev and hash stay).

Change only what the person asked. Keep ticked steps ticked if their wording is unchanged; if you reword a ticked step, untick it unless its evidence still proves the new wording. When the revision rests on new facts, add them to `research.md` and `## Where things stand now`. Then `--bump`. When the revision sends the work back to an earlier step (a failed check, a rejected preview), set `step` to that step and `round` in task.md one higher (`2` the first time it goes back).
