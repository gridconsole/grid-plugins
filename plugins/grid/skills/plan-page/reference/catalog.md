# Component catalog

Each code is stable. The live example and its variant are in `examples.html` under `#c-NN` (lowercase); copy that markup, keep the class names, change only the content.


## Where we are

- **C-01 Phase track with gates** — The workflow's steps as a bar, gates between them. Use in every header; the amber gate is the one waiting on a person.
- **C-41 Pill timeline** — Phases as capsules whose length is their time; the current one breathes. Use in a header where C-01 would be too tall.
- **C-42 Phase ring** — The workflow as a ring: each arc one phase, sized by effort, the current one in accent. Use as the one visual on a project or task tile.
- **C-19 Workflow strip** — The task's own workflow from workflows.md, not a fixed five phases: each step with its kind, who acts, what moves it on, and where a rejection goes back to. C-01 is drawn from this.
- **C-20 Approvals** — Every gate in this task and who passes it, from team.md and the workflow. Use in the header area of any task with more than one approver; your own gate is amber.
- **C-21 Workflow in words** — The same workflow as one sentence per step, for the Summary of a task whose workflow the reader does not know. Use once, above the steps.
- **C-37 Subway map** — Steps as stations on a line, the reject loop drawn as a loop. Use for a chapter or a short workflow where the reader thinks in "stops".
- **C-40 Gate flow with diamonds** — Boxes do the work, diamonds are the gates a person passes; the "no" branch says where work goes back. Use to explain a workflow with rejections.
- **C-02 Stacked progress** — All steps as one bar split by state. Use when the reader wants one glance, not a per-chapter view.
- **C-03 Progress ring with the one sentence** — Use at the top of Where it stands when the plan is long; the sentence says what the number means.
- **C-23 Big number with a sparkline** — One number set large, the sentence that gives it meaning, and the last seven days underneath. Use in Answer first when one figure carries the status.
- **C-04 Chapter tiles** — One tile per chapter: count, bar, state in words. Use when chapters are the natural unit; each tile links to its chapter.
- **C-22 Week strip** — The task on a calendar: gates, due times and who is away. Use when dates matter more than steps (shipping, customers, schedules).
- **C-27 Meters with limits** — Token use, context and time against their limits, with the warning mark on the track. Use on Sessions and in the header; never a bare percentage.
- **C-39 Arc gauges** — Half-circle gauges for the three things with a limit: tokens, context, waiting time. The amber needle is the alert mark.

## Steps

- **C-05 Stepper** — A vertical checklist with state per step. The default for a chapter of up to 8 steps.
- **C-06 State lanes** — Steps grouped by what they wait for. Use when the question is "what is stuck and why", across chapters.
- **C-07 Owner swimlanes** — Who does what, step numbers as cells. Use when the reader is balancing load or wants to see their own steps.
- **C-08 Chapter timeline** — Chapters on a day axis with gates as amber lines. Use for effort and sequence; replaces the bar chart when order matters.
- **C-24 Dependency graph** — Which steps block which, with the question that unblocks them. Use when the reader asks "why is nothing moving"; at most ten nodes, else C-06.
- **C-32 Chapter table with inline bars** — One row per chapter: progress bar, owners, what it waits on. The table form for plans over 40 steps; steps open per row, never all at once.
- **C-38 Dot matrix** — All 34 steps as dots, chapters separated by a dashed dot. The densest honest progress view; hover a dot for its step.

## What needs a person

- **C-09 Decision table** — Decision, recommendation, why; one row each. Use when there are more than three open; the full question opens on click.
- **C-10 Decision card** — One question, options written out, the recommended one first and tinted. Use for a blocking question, always with the consequence line.
- **C-11 Callouts** — Three tones only: needs you (amber), the planner found (accent), stopped (red). Never a fourth, never a left stripe.
- **C-34 What you can do now** — Two to four actions with their consequence, the primary one tinted. Use at the end of a Needs-you section; a button never stands alone without its consequence.
- **C-29 Decision ledger** — Every decision as one line: when, who, what and why. Use in History and at the end of Questions; it is what history.md reads like.

## Evidence, options, files, risks

- **C-12 Checks** — What passed and what did not, each with its evidence link. Use in Result and on a Ship gate.
- **C-26 Evidence well** — A command and the lines that prove a step, nothing more. Use under a tested step or a passed check; the terminal look is reserved for this.
- **C-33 Findings** — What the planner or a spike found, numbered large, each with its evidence. Use in Research and in Answer first when the plan rests on a few facts.
- **C-13 Option comparison** — Two to four options with pluses and minuses. Use in research and before a decision with trade-offs; the recommended one is tinted.
- **C-25 What changed between revisions** — Added, removed and moved lines between plan rev 2 and rev 3, in three columns. Use after every revision, above the plan; never a raw diff.
- **C-14 File tiles** — Sub-pages, previews and files the planner wrote, with a real preview and a type chip. Use in Sub-pages and Result.
- **C-30 Preview strip** — Screens the planner rendered, side by side with captions, each opening in a light box. Use at a Preview gate and in Result.
- **C-15 Flow diagram** — Boxes and arrows, at most six boxes, the one that matters outlined. Use for architecture; built from elements, never one image.
- **C-16 Risks** — Level, the risk, and the step that retires it. Use in every plan; two levels of tone only.
- **C-17 Facts** — Key and value pairs, mono values. Use for the small numbers that do not deserve a stat tile.
- **C-28 Roster** — Who is on this task, from team.md: people and agents, what each may do. Use once in a task; the Approvals component (C-20) reads from it.
- **C-31 Activity heat** — Agents per hour over the last days, amber where a person was needed. Use on Sessions and Agents to show rhythm, not on a task summary.

## Vocabulary

- **C-18 Task header** — Required, first, always the same shape: id and project, title, one status sentence, phase track, token use. Everything else can vary.
- **C-35 State vocabulary** — The only words and colours for a state, anywhere in Grid. The skill may not invent a fifth tone; the glyph and the word always travel together.
- **C-36 Absence, in one line** — When something does not exist yet, say so in one line with the next step. Never an empty box, a zero, or "nothing yet" on a first screen.

## Architecture and code

- **C-43 Component diagram** — Modules as boxes inside their repo, ports where they connect, the touched ones outlined. Use for "what talks to what" at the level of the plan; at most twelve boxes.
- **C-44 Sequence diagram** — Who calls whom, in order, for one scenario; your own step in amber, a refusal in red. Use for a gate or a hook; built from lifelines and arrows, never an image.
- **C-45 Diff map** — Every file the task changes on one screen: kind, name, size of change as bars, and why in one line. Predicted files are dashed. Use in Code and at a Ship gate.
- **C-46 Annotated snippet** — The lines that matter, with numbered notes beside them saying how it works and why. Use to explain a mechanism once; never paste a whole file.
- **C-47 Before → after** — The same lines before and after, with one sentence on why. Use for a decision that changed code, or to show a worker what to do; keep each side under fifteen lines.
- **C-48 Call graph** — What calls what, left to right, the touched functions outlined, the removed one struck. Use when a change ripples; at most twelve nodes, else C-43.
- **C-49 State machine** — States as circles, transitions with their guard; the current state glows. Use for a task, a session or a gate whose rules the reader must trust.
- **C-50 Contract table** — Commands, hooks or endpoints with what goes in and out; changed rows tinted, new rows green. Use when a task touches an interface other code depends on.
- **C-51 File tree with markers** — The repo tree, collapsed to what the task touches, each file marked A, M, D or predicted. Use when the reader knows the codebase and wants the shape of the change.
- **C-52 Layer diagram** — The stack from the person down to Claude Code, with the layers this task touches outlined and a count. Use in Architecture when the question is "how deep does this go".
- **C-53 Commit timeline** — Commits on the task branch with the step each one closes; predicted commits dashed. Use at Deliver and in Code to connect steps to git.
- **C-54 Test matrix** — Tests down, steps across; green where a step is proven, red where it fails, hatched where a test is still to be written. Use before a Ship gate.
- **C-55 Error → fix** — The error as it appeared, the cause in one line, and the fix as code. Use under a retried step and in Result; the error text is verbatim, never paraphrased.
- **C-56 Config and flag diff** — Settings, flags and environment that change, as key · old → new · why. Use whenever a task touches configuration; never bury it in a diff.

## Pages around the task

Lists, queues and panels for Home, Inbox, Project and Sessions. Red is stuck, amber waits on you, grey can wait.

- **C-57 Task row** — State dot, id, title, project, what it waits on, age; the whole row opens the task, with an optional group head per project. Use when a page lists tasks. Do not use inside a task page; there the steps are C-08.
- **C-58 Needs-you queue item** — What is asked, the recommended answer with what it leads to, and the one action that takes it. Use when Home or Inbox lists questions across tasks. Do not use inside a task; the full decision is C-10.
- **C-59 Project tile** — Name, repo chips, open tasks, what waits on you, tokens this week; a quiet tile when nothing waits. Use when several projects sit side by side. Do not use as the head of the project page; that is C-67.
- **C-60 Live session tile** — Agent, model, task, running time, tokens, context left and the last line; amber when it waits, red when stuck. Use when Home or Sessions shows what runs now. Do not use for a finished run; that is a line in C-21.
- **C-61 Machine panel** — CPU, memory, disk and sessions running, each with a bar, amber near the limit; an offline variant. Use when a page must say whether there is room for another run. Do not use for plan limits; that is C-62.
- **C-62 Usage panel** — Plan limits as C-27 meters, the week so far as one figure, tokens by project as bars. Use when Home or Sessions answers how much is left and where it went. Do not use for one task's usage; that sits in C-18.
- **C-63 Grid health panel** — Plugin version, hooks, skills, agents and the last check; a failing row carries its fix. Use when a page shows whether Grid itself is set up right. Do not use for a task's own checks; those are C-12.
- **C-64 Agent roster card** — Agent, role, model, last run, pass rate with a bar; amber when the pass rate drops. Use on the Agents or Roster page. Do not use for the people and agents on one task; that is C-28.
- **C-65 Empty, absent and loading list** — Calm empty, not set up, filtered out, loading skeleton, failed with retry; each says what comes next. Use whenever a C-57 to C-64 list can be empty. Do not use for a missing section inside a task; that is C-36.
- **C-66 View chips with counts** — Two to five views of one list, each chip with its count, amber when it waits on you. Use for Inbox views such as To decide, Snoozed, Done lately. Do not use for page navigation; that is the rail.
- **C-67 Page head** — Title, one sentence of where things stand, and the counts that matter. Use when Home, Inbox, Project or Sessions opens. Do not use on a task page; that is C-18.
