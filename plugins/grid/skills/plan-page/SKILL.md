---
name: plan-page
description: Use whenever a Grid task has a plan.md that is written or revised, and every time you write or revise an HTML page for a task: the plan page (`views/plan.html`), or a mock or preview kept in `artifacts/`. A task with a plan.md always gets a `views/plan.html`, even when the plan was written through the plan skill and nobody asked for a page. Enforces the task-page contract (R1–R6, in order) and builds every part from the Grid plan-and-task component library (C-01…C-67) with Grid Console tokens, so every plan reads the same way.
---

# Plan page

A plan is an HTML page the planner writes, not a list of cards. Every page in `artifacts/`, and `views/plan.html`, is built from the component library in `reference/`; the other three task views (`progress.html`, `result.html`, `questions.html`) are written through the views skill, as `v-*` page frames over the same `tokens.css` and `components.css`. Codes are stable: refer to components by code, copy their markup from `reference/examples.html`, keep the class names, change only the content.

**Whenever plan.md exists, so does `views/plan.html`.** Write the page in the same turn as plan.md, after the plan skill has written it; a plan without its page is half done. The page shows what plan.md says: what research found, the options, steps, risks and open questions come from there, from `research.md` and from `questions/`, never invented.

## Before writing

1. Read `reference/catalog.md` (one line per component: what it is, when to use it). It is short; read all of it.
2. Take the entry for each part from the table below. Print one with `sed -n '/id="c-10"/,/^    <\/section>/p' "${CLAUDE_SKILL_DIR}/reference/examples.html"` (change the code); the entry shows the live example and a **variant** (skipped phase, parallel lanes, stuck task, absent data…). Copy the markup of the one closest to your situation. Read the entries for R1 to R6 before you write; the page fails the contract when one is guessed.
3. The task folder is `<projects folder>/<project>/tasks/T-NN-slug/`; the projects folder is `GRID_PROJECTS`, else `$GRID_HOME/projects`. Make `<task folder>/ds/` if it is missing and `cp` `tokens.css` and `components.css` from `${CLAUDE_SKILL_DIR}/reference/` into it by absolute path (if `cp` or `mkdir` is denied, as in the e2e driver, still link `../ds/` and say nothing more). Nothing is written relative to the working directory, which is the project's repo. Then link `../ds/tokens.css` and `../ds/components.css` in that order. Open the page with `<body class="g">`: `.g` carries the library's font, text colour and background, and a body without it renders in the browser's default serif on a plain ground. The page then needs no stylesheet of its own: a `<style>` block may lay out the page (`main` width, spacing) and must never redefine a library class (`.hd`, `.sp`, `.rk`, `.btn`, …) or a token. Never inline a second colour system.

## The contract — every task page, in this order

| | Part | Must be | Built from |
|---|---|---|---|
| R1 | Header | id, project, title, one status sentence, workflow track (the task's own steps, the current one marked, each named in words as plan.md names it: "Check the CLI builds", never the task.md id `check-the-cli-builds`), token use against its limit | C-18 (contains C-01 or C-41, C-27) |
| R2 | Answer first | one headline that states the conclusion, one paragraph, at most four stat tiles | C-17 optional, C-23 when one figure carries the status |
| R3 | Where it stands | progress as a visual, never a raw count alone | one of C-02, C-03, C-04 |
| R4 | Needs you | the open decisions, written completely; **empty = leave the section out**, never "nothing yet" | C-09 (>3 open) or C-10 (one blocking; its options are the actions) |
| R5 | Steps | chapters and steps, each step with its reason from plan.md | any of C-05, C-06, C-07, C-08; a table (C-32) only above 40 steps, paged by chapter |
| R6 | Risks | level, the risk, the step that retires it | C-16 |
| O1 | Research and options, architecture, effort, files | research and options are required when plan.md has `## Where things stand now` or `research.md` exists; the rest as the content needs | C-33 findings · C-13 options · C-15/C-43/C-52 architecture · C-08 effort · C-14 files · C-45/C-51 code |
| O2 | Result | appears once the last step is done | C-12 checks, C-14 files, one paragraph; C-53, C-54 when there is a branch |

A page that lacks R1–R3 is not a plan. When the plan has its situation and options (the plan skill's `## Where things stand now` and `## Approach and options`), the page carries two O1 parts after R3 and R4 and before R5: "What research found" in C-33, each finding with its source (`file:line`, a `research.md` section, a link), and "Approach and options" in C-13, the chosen option tinted and its reason in the paragraph before it. A reader decides from these, so never leave them out. Their headings are the name and a count, nothing under them. C-13 is built on `.cmpare` like C-10, which the checker reads as R4, so it must stand before R5, and it never replaces a C-10 for an open question. Each step on the page carries the reason paragraph plan.md gives under its title: in C-05 it is the step's first `.m` line under the `.t` title, before the worker and state line, and in the other R5 components the line under the step's title; a step shown as title and verify alone leaves the reader without the why. R4 and R6 may be absent only when they are empty; say nothing rather than "none". `.rk` (C-16) is for risks only and comes after the steps: assumptions and open points go in an O1 part or in R4, never as `.rk` rows before R5.

How a part is told apart (the repo's contract checker, `tests/contract/plan-page-check.mjs`, reads the class, not the words; a part built with a class of your own does not count):

| Part | Root class on the page | Entry |
|---|---|---|
| R1 | `header.hd`, with the workflow track inside: C-01 (`.track`, one `.ph2` per step with `.bar` and `.lb`) or C-41 (`.pills2`, one `.pill2` per step), put as a full-width row under the title and status line, not in the narrow right column; never `.pills` with `.ph` spans, which the library does not style (the names print as one run of words) | `#c-18` |
| R2 | an `h2` that states the conclusion and a paragraph, before R3 | `#c-17` |
| R3 | `.stack` (C-02), `.ring` (C-03) or `.chaps` (C-04) | `#c-02` `#c-03` `#c-04` |
| R4 | `.cmpare` (C-10, one question with its options) or `table.dt` (C-09, many) | `#c-10` `#c-09` |
| R5 | `.stepper` (C-05), `.cols4` (C-06), `.swim` (C-07), `.gantt` (C-08) or `table.ct` (C-32) | `#c-05` … `#c-08` |
| R6 | `.rk` | `#c-16` |

R4 is required whenever `questions/` holds an open question or plan.md names one, so rewrite the page whenever a question is recorded or answered after it was written: write the question completely in C-10 (with the question's id, `id="q-1"`). The choice is shown once: the C-10 options ARE the actions. Each `.opt2` is titled with what it does ("Approve the import", "Ask for a rev 2"), carries its consequence line and ends with its key hint (`<kbd>a</kbd>`); the recommended one comes first with `.rec` and the Recommended tag. Do not add a button row (`.qa` buttons) or a C-34 block that repeats the options. Use C-34 only when R4 is a C-09 table, or for actions that are not answers to that question. Do not build a callout, a button row or an "ask" box of your own; the library has no such class.

## Live-state markers

A page is written at one moment and read later. Grid replaces the blocks that carry live state, by class, once the task has shipped or closed, with what the header shows (Approved and merged, when; Close · delivered). After a ship approval Grid closes the task, so the header reads closed; a preview approval does not close it, and the preview case stays as it is; the rest of the page stays as written. Put these classes, next to the library's own, on every such block:

| Class | On | Replaced by |
|---|---|---|
| `live-status` | the header's one status sentence (C-18's `.amber.b` span) | "Approved and merged <when>", or "Closed" |
| `live-headline` | put it on the R2 answer `h2`, the headline that states the conclusion | the same line as `live-status` ("Approved and merged <when>"), so it cannot say the build waits |
| `live-next` | any line that says what the task waits on or does next (C-18's wait line, the gate note under the track) | the next step after shipping |
| `live-needs` | a whole block that asks for an approval or an answer now: the R4 wrapper around C-10 or C-09, C-34's `.acts`, a C-11 `.co.you` | one line with the shipped or closed state |
| `live-gates` | C-20's `.apr` grid (every gate and who passes it) | the same rows, each state cell "approved <when>" |

Progress carries live state too: the counts of C-02, C-03 (`.ring`), C-23 (`.bign`), C-04 (`.chaps`) and C-38 (`.dots`) are rewritten by Grid from their classes (keep the library's classes and the "n of m" and "n/m" shapes), so they need no marker. A sentence that says nothing has started or that the build waits for the accept ("No step has started; the build runs once you accept the plan", the ring's `.fg1` line, a C-23 `.txt b`) gets `live-headline`, so it is written in the header's words once the plan is accepted, shipped or closed. Write it in the library's own element, never as free text beside the component.

The header's status line (C-18's `.amber.b` span and the `.dim` span beside it) and the progress section's own `.micro` status line ("research done 19:32 · next: step 1") are rewritten by class as well, while the task builds, reviews or ships, so write them with those classes (or mark them `live-status` / `live-next`); a section made only of C-38 `.dots` counts as a progress section.

Never mark a block that records a decision already taken (C-29), a finding, a step or a risk: those are the plan and stay. When writing or revising a page, and when the deliver skill refreshes it, check every wait and every Approve is inside one of these markers.

## Rules that do not bend

- **One surface ladder.** `--ob-bg` → `--ob-panel` → `--ob-raised` → `--ob-hover`. A card sits on the page; nothing sits in a card except text, bars and chips. No nested cards, no box around a group of boxes, no shadows — borders only (`--ob-line`, `--ob-line2` for emphasis).
- **Three tones for attention.** Amber (`--ob-ask`) = waits on a person. Red (`--ob-err`) = stuck or refused. Accent (`--ob-acc`) = the planner found / the current thing. Green only for proven (`--ob-ok`). There is no fourth tone; C-11 and C-35 are the only vocabulary.
- **States are words and glyphs together** (C-35): `✓ tested/done`, `● running/now`, `! waits on you`, `■ stuck`, `○ planned`, `⊘ skipped`, `+ added`. Never a colour alone, never a word you invented.
- **Numbers carry a sentence.** A percentage, count or usage figure never stands alone (C-03, C-23, C-27). A button never stands alone without its consequence (C-34).
- **Absence is one line** (C-36): "No preview yet · appears after step 12." Never an empty box, a zero or a dash on a first screen.
- **Evidence is verbatim.** Terminal output (C-26), errors (C-55), diffs (C-45, C-47) are copied, not paraphrased; the terminal look is reserved for them.
- **Diagrams are elements, not images.** C-15, C-40, C-43, C-44, C-48, C-49, C-52 are built from divs and SVG lines so they can be read, linked and diffed. Cap: 6 boxes for C-15, 12 nodes for C-43/C-48, else fall back to the table form.
- **Workflows are read, not assumed.** C-19/C-20/C-21 come from task.md's `workflow:` (each step and its kind) and `step:` (the current one); never draw a fixed phase bar. The amber gate is the one waiting on the reader.
- **Every step links.** Steps, files, checks and decisions carry ids (`#s-12`, `#q-3`) so the task view's comment-attach and the questions queue can point at them.

## Choosing a component

| Reader's question | Use |
|---|---|
| Where are we, and whose turn is it? | C-01 (header), C-41 when short, C-42 on a tile, C-20 when more than one approver |
| How much is done? | C-02 one bar · C-03 ring + sentence · C-04 per chapter · C-38 all steps as dots |
| What is stuck and why? | C-06 lanes · C-24 dependency graph · C-11 callout |
| Who does what? | C-07 swimlanes · C-28 roster |
| When? | C-08 chapter timeline · C-22 week strip · C-53 commits |
| What do you need from me? | C-10 one question · C-09 many · C-34 actions with consequences · C-29 what was decided |
| What did you find? | C-33 findings · C-13 options · C-26 evidence · C-12 checks |
| What changed? | C-25 between revisions · C-45 files · C-47 code · C-56 config |
| How is it built? | C-43 components · C-52 layers · C-44 sequence · C-49 states · C-50 contracts · C-48 calls |
| Which files? | C-45 diff map · C-51 tree · C-14 tiles · C-30 previews |
| Is it safe to ship? | C-54 test matrix · C-12 checks · C-16 risks · C-27 meters |

## Writing

- Headline states the conclusion ("Rev 3 is ready to accept; three questions block the build"), not the topic ("Status").
- One paragraph per section before any component; the component proves the paragraph.
- Second person for the reader ("waits on you"), named agents for workers ("grid-worker 2 is drafting step 3").
- Times are absolute with relative in brackets: `07:40 (2 h ago)`. Usage against a limit is in tokens or a share of the plan: `45k of 500k tokens`, never dollars.
- Keep each code snippet under fifteen lines (C-46, C-47); link the file for the rest.

## Check before you finish

Read the page back against the class table above: R1 to R6 each present by their class and in that order, R4 present when a question is open, no `<style>` rule that redefines a library class, `components.css` linked, `<body class="g">`, the step line is a `.track` (or `.pills2`) with one element per step, its step names are words and not ids. Then: every number has its sentence; no nested cards; no colour outside the tokens; every step and question has an id; the page reads at 1100px and at 430px (single column, side panels collapse). If you have a way to render it, look at it at both widths. Fix, then write the file, and say in your report which of these you checked and which you did not.
