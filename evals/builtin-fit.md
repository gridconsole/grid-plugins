# Can `claude plugin eval` run the 19 original eval cases?

Research note for decision v-11 ("Keep the 19 cases. Check first whether the built-in command can run them before keeping the harness"). It changes no code.

## Header

- Plugin main read: `b561251f`. It contains `dc301fce`, the main the brief names (`git merge-base --is-ancestor dc301fce main` exit 0).
- Claude Code version: 2.1.319.
- The 19 cases: the folders in `evals/tasks` at plugin commit `1f1e8200`. Their checks are read from `evals/tasks/<id>/checks.json` on main.
- Sources:
  - **docs**: "Test plugins with evals", https://code.claude.com/docs/en/plugin-evals (fetched 2026-11-11). Below, `docs#<anchor>` means a section of that page, for example `docs#grader-types` is https://code.claude.com/docs/en/plugin-evals#grader-types. The page is the current one. It does not say which Claude Code version it describes; it names v2.1.269 as the minimum. Behaviour specific to 2.1.319 is therefore UNVERIFIED wherever this note relies only on the docs.
  - **help**: `claude plugin eval --help`, Claude Code 2.1.319. **Not read.** The lane sandbox refused every read of the captured help text (`cat`, `grep -n` and `ls` of its folder were all refused). No `help:<line>` is cited. The docs page lists the help-only flags as `--case`, `--tag`, `--eval-dir`, `--no-scaffold`, `--report` and `--verbose` (docs#command-options). Anything only the help text would settle is marked UNVERIFIED.
  - **code**: the converter `ExportSuite` (internal/evalcmd/suiteexport.go:46) and its check mapping `exportGrader` (suiteexport.go:102-142). The offline grader `GradeCase` (internal/evalcmd/suitegrade.go:71) and its grader keys (suitegrade.go:59-64). The Go runner's environment (internal/eval/runner.go:243-270). The mapping table is in docs/lanes/C-ic-suite-convert.md:34-41.

Mapping rules used in the table. Each comes from suiteexport.go and is checked against the docs:
- `file_exists` maps to `file_exists` (suiteexport.go:109-113). The docs add: "Only files created during the run count" (docs#grader-types). A file the scaffold made is invisible to it (docs#everything-scores-zero-although-the-right-files-were-produced).
- `text_contains` maps to `regex` with `target: {source: file, path}` (suiteexport.go:114-115). The docs say this target reads "the contents of one file in the workspace after the run" (docs#what-a-grader-can-look-at).
- `command_passes` is dropped: "the built-in command has no command grader" (suiteexport.go:107-108). The docs say: "There are no custom-code graders." (docs#choose-and-weight-graders).
- None of the 19 has a `top_level`, `llm_judge`, `tool_used` or `tool_order` check.

Two facts apply to 16 of the 19 cases, every one except clean-csv, fix-total and release-notes:
- **Their setup cannot run as a scaffold.**
  - Each of the 16 setup.sh files opens with `: "${GRID_HOME:?}"` (for example evals/tasks/agent-reviewer-read-only/setup.sh:3; `grep -c` finds the line in all 16).
  - The Go runner sets `GRID_HOME` (runner.go:256-259).
  - A scaffold script gets only `PATH`, `HOME` (the run's temporary home), `TMPDIR` and a few constants. "Nothing else from your shell reaches it" (docs#seed-the-workspace-or-conversation).
  - So the script exits 1, and "that run scores 0 with a `scaffold failed` error" (same section).
- **Their checks read files outside the workspace**, under `$GRID_HOME/projects/...`. See (b).

## Fit table

| id | checks on main (id: type) | built-in grader per check, or dropped with reason | fit (full, partial, none) | still tests a feature on main (skill or verb path:line, or "removed: <docs line>") |
|---|---|---|---|---|
| agent-reviewer-read-only | verdict-rejected: command_passes; code-untouched: text_contains; project-files-unchanged: command_passes | code-untouched → regex, target file work/notes-app/src/store.js (suiteexport.go:114-115). verdict-rejected and project-files-unchanged are dropped: no command grader (suiteexport.go:107-108). verdict-rejected also reads under `$GRID_HOME`, outside the workspace | none | agents/reviewer.md:1 still exists. The case asks for the old `grid-reviewer` agent and reads an old `subagents/NN-*.md` brief (docs/old-plugin-map.md:81, :86) |
| agent-reviewer-verdict | code-untouched: text_contains; outcome-untouched: command_passes; verdict-rejected: command_passes | code-untouched → regex, target file work/notes-app/src/store.js (suiteexport.go:114-115). outcome-untouched and verdict-rejected are dropped: no command grader (suiteexport.go:107-108), and both read `$GRID_HOME/.../subagents/02-tag-filter.md`, outside the workspace | none | agents/reviewer.md:1 still exists. The case uses the old `grid-reviewer` agent and `subagents/` brief layout (docs/old-plugin-map.md:81, :86) |
| clean-csv | csv: file_exists; rows: command_passes; summary: text_contains | csv → file_exists clean.csv (suiteexport.go:109-113); the agent creates the file, so it counts (docs#grader-types). summary → regex, target file summary.txt (suiteexport.go:114-115). rows is dropped: no command grader (suiteexport.go:107-108) | partial | No Grid feature. It is a plain file task (evals/tasks/clean-csv/prompt.md:1) that tests the model, not the plugin |
| deliver-refuses-closed | history-kept: command_passes; updated-kept: command_passes; status-closed: command_passes; step-kept: command_passes; outcome-kept: command_passes; artifacts-empty: command_passes | All 6 are dropped: no command grader (suiteexport.go:107-108). All 6 read under `$GRID_HOME`. No grader is left, and a case with no grader fails to load (docs#eval-suite-reference) | none | skills/deliver/SKILL.md:1 still exists. The case uses the old `status:`/`subagents/` layout (docs/lanes/D-plugin-skills-b.md:19) |
| deliver-refuses-rejected | no-delivered-line: command_passes; outcome-kept: command_passes; source-kept: command_passes; status-kept: command_passes; step-kept: command_passes; verdict-kept: command_passes | All 6 are dropped: no command grader (suiteexport.go:107-108). 5 of them read under `$GRID_HOME`. source-kept greps the workspace file work/notes-app/docs/release-note.md, which could be rewritten by hand, see (a). No grader is left, so the case fails to load (docs#eval-suite-reference) | none | skills/deliver/SKILL.md:1 still exists. The case uses the old layout (docs/lanes/D-plugin-skills-b.md:19) |
| deliver-skill | artifact-copied: command_passes; history-line: command_passes; intent-untouched: command_passes; no-approval-lines: command_passes; not-closed: command_passes; outcome-set: command_passes; source-kept: command_passes | All 7 are dropped: no command grader (suiteexport.go:107-108). 6 of them read under `$GRID_HOME`; source-kept reads a workspace file. No grader is left, so the case fails to load (docs#eval-suite-reference) | none | skills/deliver/SKILL.md:1 still exists. The case uses the old layout; the new-layout case is evals/tasks/skill-deliver (docs/lanes/D-plugin-skills-b.md:19, :54) |
| execute-approve-waits | no-approval-lines: command_passes; no-grid-fields: command_passes; step-kept: command_passes; verdict-written: command_passes; waiting: command_passes | All 5 are dropped: no command grader (suiteexport.go:107-108). All 5 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: the old `execute` skill (docs/old-plugin-map.md:88); main's skills/ has no execute |
| execute-check-after-make-together | project-file-unchanged: command_passes; no-approval-lines: command_passes; status-reworking: command_passes; round-two: command_passes; no-grid-fields: command_passes; rework-names-person: command_passes; check-brief-written: command_passes; step-sent-back: command_passes | All 8 are dropped: no command grader (suiteexport.go:107-108). 7 of them read under `$GRID_HOME`; project-file-unchanged greps work/site/pricing.html in the workspace. No grader is left (docs#eval-suite-reference) | none | removed: the old `execute` skill (docs/old-plugin-map.md:88) |
| execute-check-fails-out-of-scope | question-names-files: command_passes; no-approval-lines: command_passes; question-to-owner: command_passes; question-recommends: command_passes; no-grid-fields: command_passes; round-unchanged: command_passes; step-unchanged: command_passes; status-waiting: command_passes; next-names-who: command_passes; needs-question: command_passes; failure-line: command_passes | All 11 are dropped: no command grader (suiteexport.go:107-108). All 11 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: the old `execute` skill (docs/old-plugin-map.md:88) |
| execute-check-fails | no-approval-lines: command_passes; status-after-failure: command_passes; round-two: command_passes; no-grid-fields: command_passes; failure-line: command_passes; step-sent-back: command_passes | All 6 are dropped: no command grader (suiteexport.go:107-108). All 6 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference; docs/lanes/C-ic-suite-convert.md:8 shows "0 graders, 6 dropped") | none | removed: the old `execute` skill (docs/old-plugin-map.md:88) |
| execute-review-changes-past-limit | question-has-recommendation: command_passes; no-approval-lines: command_passes; waiting: command_passes; no-currency-task: command_passes; round-kept: command_passes; question-blocking: command_passes; needs-question: command_passes; no-currency-question: command_passes; step-kept: command_passes; question-names-items: command_passes; no-new-brief: command_passes | All 11 are dropped: no command grader (suiteexport.go:107-108). All 11 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: the old `execute` skill (docs/old-plugin-map.md:88) |
| execute-review-changes-rework | no-approval-lines: command_passes; status-after-verdict: command_passes; step-build-or-review: command_passes; round-two: command_passes; no-currency-task: command_passes; no-open-need: command_passes; history-sent-back: command_passes; no-currency-history: command_passes; no-question-file: command_passes | All 9 are dropped: no command grader (suiteexport.go:107-108). All 9 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: the old `execute` skill (docs/old-plugin-map.md:88) |
| fix-total | script: file_exists; sum: command_passes; zero: text_contains | script → file_exists total.sh (suiteexport.go:109-113), but this grader can never pass: setup.sh writes total.sh, and "Only files created during the run count" (docs#grader-types). zero → regex, target file total.sh (suiteexport.go:114-115). sum is dropped: no command grader (suiteexport.go:107-108) | partial | No Grid feature. It is a plain shell fix (evals/tasks/fix-total/prompt.md:1) that tests the model, not the plugin |
| memory-skill | memory-written: command_passes; dated-line: command_passes; heading: command_passes; updated-changed: command_passes | All 4 are dropped: no command grader (suiteexport.go:107-108). All 4 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: the memory skill was folded into deliver's brain proposal step (docs/lanes/D-plugin-skills-b.md:51) |
| preview-skill | approve-step: command_passes; no-grid-fields: command_passes; spec-written: command_passes; waiting: command_passes; brief-written: command_passes; no-approval-line: command_passes; spec-has-intent: command_passes | All 7 are dropped: no command grader (suiteexport.go:107-108). All 7 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: `grid preview` is gone and nothing replaces it (docs/lanes/S6c3.md:18) |
| questions-skill | sam-file: command_passes; answer-empty: command_passes; block-heading: command_passes; recommendation: command_passes | All 4 are dropped: no command grader (suiteexport.go:107-108). All 4 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | skills/questions/SKILL.md:1 still exists (docs/lanes/D-plugin-skills-b.md:52). UNVERIFIED: whether `grid question add` on main still writes the `## Q-1 · to: sam ·` heading the checks expect. The new-layout case is evals/tasks/skill-questions (docs/lanes/D-plugin-skills-b.md:11) |
| release-notes | notes: file_exists; heading: text_contains; fix: text_contains; feature: text_contains | notes → file_exists RELEASE.md (suiteexport.go:109-113); the agent creates the file, since setup.sh writes only changes.txt (evals/tasks/release-notes/setup.sh:2), so it counts (docs#grader-types). heading, fix and feature → regex, target file RELEASE.md (suiteexport.go:114-115). Nothing is dropped | full | No Grid feature. It is a plain file task (evals/tasks/release-notes/prompt.md:1) that tests the model, not the plugin |
| routing-skill | exactly-one-new-row: command_passes; members-intact: command_passes; old-rows-intact: command_passes; row-added: command_passes; topics-intact: command_passes | All 5 are dropped: no command grader (suiteexport.go:107-108). All 5 read `$GRID_HOME/projects/notes-app/routing.md`. No grader is left (docs#eval-suite-reference) | none | removed: the old `routing` skill (docs/old-plugin-map.md:88); main's skills/ has no routing |
| views-skill | progress-written: command_passes; links-tokens: command_passes; no-literal-colours: command_passes; no-placeholders: command_passes; reads-workflow: command_passes; shows-plan-items: command_passes; tokens-copied: command_passes; views-css-copied: command_passes | All 8 are dropped: no command grader (suiteexport.go:107-108). All 8 read under `$GRID_HOME`. No grader is left (docs#eval-suite-reference) | none | removed: the `views` verb was removed (docs/lanes/M-plugin-held-f-e.md:27), and so was the old `views` skill (docs/old-plugin-map.md:88) |

Why the two agent-reviewer cases are `none` and not `partial`: code-untouched does map to a grader. The fit column, however, judges the case as it stands. As it stands, its scaffold exits 1 because `GRID_HOME` is not set (setup.sh:3; docs#seed-the-workspace-or-conversation), so every run scores 0 before any grader runs. The same holds for the other 14 `$GRID_HOME` cases, on top of their having no grader left.

## Answers

### (a) Can any built-in grader stand for `command_passes`, and what does that cost in meaning?

No built-in grader runs a command. The six grader types are regex, tool_used, tool_order, file_exists, llm and baseline, and the docs state: "There are no custom-code graders." (https://code.claude.com/docs/en/plugin-evals#choose-and-weight-graders). The converter drops every command_passes check for this reason (internal/evalcmd/suiteexport.go:107-108). The three possible stand-ins:

- **A script grader.** It does not exist (same docs line).
- **A scaffold step after the run.** It does not exist. `context.scaffold_script` "runs in the empty workspace before Claude starts" (docs#case-yaml-fields). Nothing in the case format runs after the agent.
- **A regex over the transcript of a command the agent ran.** This is possible, and the docs recommend it for builds and tests: "have the prompt ask Claude to run it and write the outcome to a file, grade that file, and assert the command ran with a `tool_used` grader whose `input_match` names the command" (docs#choose-graders-that-give-a-stable-signal). It changes what is measured:
  - it proves the agent ran a command and what the agent reported, not that the end state meets the check;
  - the check moves into the prompt, so the agent is told what will be checked, which most of the 19 must not do (for example "no-approval-lines" or "no-grid-fields").

A fourth route needs no command: most command_passes checks are a `grep` over one file, and those can be rewritten by hand as `regex` with `target: {source: file, path}`, using `match: not_contains` for a `! grep`. This keeps the meaning when the file is in the workspace. In the 19, that applies to rows (clean-csv), source-kept (deliver-refuses-rejected, deliver-skill), project-file-unchanged and project-files-unchanged. It does not reach the `$GRID_HOME` checks; see (b).

- Checks that compare a whole file (`cmp`) would need an anchored JavaScript regex.
- Checks for a file's absence (`test ! -e`) have no exact counterpart, because `file_exists` with `exists: false` sees only files created in the run (docs#grader-types).
- Arithmetic checks such as fix-total `sum`, which runs `sh total.sh 2 3`, have no counterpart: only the agent can run the script.
- UNVERIFIED: what the built-in does when a regex file target does not exist. The offline grader fails it (suitegrade.go:248-250).

### (b) Can a `file` target or `file_exists` read a path outside the case workspace, such as `$GRID_HOME/projects/...`?

Not by anything the docs state:
- `{ source: file, path: <path> }` reads "the contents of one file in the workspace after the run".
- `file_exists` matches "a file Claude created" (https://code.claude.com/docs/en/plugin-evals#what-a-grader-can-look-at, #grader-types).

The repo's offline grader refuses an absolute path or a `..` path outright (internal/evalcmd/suitegrade.go:339-347). UNVERIFIED: whether the built-in refuses `../` or an absolute path, or reads it.

Even a path the grader could read would not help, for three reasons:
- **The agent cannot write there.** With Bash granted, "writes are confined to the run's workspace" (docs#grant-tools).
- **`GRID_HOME` never reaches the run.** The child gets only an allowlist and `EVAL_*` variables, and `env` keys must match `EVAL_[A-Z0-9_]*` (docs#prompt-md-frontmatter).
- **The scaffold does not get `GRID_HOME` either** (docs#seed-the-workspace-or-conversation). The Go runner puts the Grid home at `<attempt home>/.grid` and exports it (internal/eval/runner.go:256-259); the built-in has no counterpart.

So a `$GRID_HOME` case would need its Grid home inside the workspace, and the plugin's `grid` would have to find it there with no `GRID_HOME` variable. UNVERIFIED: whether main's `grid` can be pointed at a home through an `EVAL_*` variable or a flag.

### (c) Does the built-in read evals/tasks/*/prompt.md as cases when the eval dir is the default `evals/`, and what does `--eval-dir evals/suite` avoid?

Yes, by the docs' rule. The default eval dir is `evals/`. "A directory counts as a case when it holds a `prompt.md` or a `case.yaml`", and cases may be nested under a directory that is not a case (https://code.claude.com/docs/en/plugin-evals#eval-suite-reference, #write-and-refine-cases). So with the default, each `evals/tasks/<id>/` (43 on main) is a case, and so is each `evals/suite/<id>/`. No `evals/tasks` folder has a `graders/` folder, and "a case without at least one grader fails to load with an `invalid case.yaml` error that names `graders`" (docs#eval-suite-reference). A case file that fails to load makes the command exit 1 (docs#run-evals-in-ci).

`--eval-dir evals/suite` avoids three things (docs#use-a-different-eval-directory):
- it reads only the converted or hand-made cases;
- `results/` is written under `evals/suite/results/` instead of `evals/results/`;
- the 43 grader-less folders never load.

The plugin manifest sets no `experimental.evals` (grep of .claude-plugin/plugin.json finds nothing), so without the flag the default `evals/` applies. UNVERIFIED: whether one case that fails to load stops the whole suite or only scores that case 0.

### (d) May a case folder or the eval dir hold files that are not a case, and are they ignored?

Partly stated. A directory without `prompt.md` or `case.yaml` is not a case and can group cases. Anything inside a case directory "belongs to that case" (https://code.claude.com/docs/en/plugin-evals#write-and-refine-cases). So:
- **A folder such as a Go test folder** is not a case as long as no folder under it holds a `prompt.md` or `case.yaml`; a `testdata/<x>/prompt.md` below it would be read as a case. There is no `evals/suitecheck` on main.
- **A fixture file in a case folder** is fine. The docs show fixtures beside `prompt.md` (#eval-suite-reference), and evals/suite cases keep `fixture.sh` there.
- **A loose file such as `evals/README.md`:** the docs name what makes a directory a case but never say a loose file in the eval dir is ignored. UNVERIFIED (the help text was not readable).
- **A non-grader `.md` inside `graders/`** would be read as a grader, because each file under `graders/` is one grader (#grader-frontmatter); one with no `type` would fail the case. UNVERIFIED.
- **`results/`** is written into the eval dir by each run, and the docs say to add it to `.gitignore` (#eval-suite-reference). The repo's .gitignore has no `results` entry.

### (e) How does `--case <glob>` match, so one live run runs one case?

"Add `--case <glob>` to filter by case name" (https://code.claude.com/docs/en/plugin-evals#choose-what-to-evaluate). The case name is the `name` field, which defaults to the directory name, and "`--case` globs match it" (docs#prompt-md-frontmatter). When both files exist, `prompt.md` front matter overrides `case.yaml` (docs#case-yaml-fields).

For an exported case, the name is the folder name either way. `case.yaml` gets `name: <id>`, and `prompt.md` gets no `name` (internal/evalcmd/suiteexport.go:66, :97-99). So `--case release-notes` selects exactly that case.

Another way to run one case: give the case's `prompt.md` or `case.yaml` path as the target (docs#choose-what-to-evaluate).

UNVERIFIED: the glob dialect, that is, whether `*` crosses `/` and whether matching is case-sensitive.

### (f) Which flags does a run of one case need, and what does `--ablation` do to the run count and cost?

From https://code.claude.com/docs/en/plugin-evals#command-options and #grant-tools:

- `--scaffold`: needed for any case with `context.scaffold_script` (all exported cases). Off by default; without it the fixture never runs.
- `--allow-tools Bash Write Edit`: Bash, Write and Edit are removed from the session unless granted. A case's `allowed_tools` cannot widen this (#security). release-notes and clean-csv need Write; fix-total needs Edit or Write, and Bash for `verify.sh`. A Bash grant runs under the OS sandbox: on Linux, `bubblewrap` and `socat` must be installed, or each run is refused.
- `--trust-plugin`: needed when stdin or stdout is not a terminal or with `--json`. Otherwise the run "is refused with exit 1" (#trust-the-plugin-directory).
- `--no-publish`: keeps `report.html` local instead of publishing it as an artifact (#html-report).
- `--runs 1`: overrides the case's `runs` (default 3). Exported cases already say `runs: 1` (suiteexport.go:98).
- `--eval-dir <dir>`: see (c).

Example: `claude plugin eval . --eval-dir <dir> --case <id> --runs 1 --scaffold --allow-tools Bash Write Edit --trust-plugin --no-publish --ablation none --json <file>.json`.

`--ablation`:
- By default a path target runs two arms, so a suite makes "roughly cases × runs agent runs with the plugin and the same number again for the no-plugin baseline, plus three short judge calls per `llm` or `baseline` grader per run" (#how-a-case-is-scored).
- `--ablation none` runs one arm and "halves the cost", and it also stops graders from being excluded from the score (#compare-against-a-no-plugin-baseline).
- `--ablation with-without` forces the second arm.

None of the 19 has an llm grader after export, so they add no judge calls. For the three model-only cases (clean-csv, fix-total, release-notes), the without-arm is the more telling arm, since they test no plugin feature.

### (g) What does the run write, and where?

Per https://code.claude.com/docs/en/plugin-evals#read-the-results and #command-options:
- Every run with at least one case writes `<eval dir>/results/<timestamp>/` holding `aggregate-result.json` and `report.html`, plus `mock-recordings/` when agent mocks answered. For a path target this is under the plugin; with `--eval-dir evals/suite` it is `evals/suite/results/<timestamp>/`.
- `--output-dir <dir>` moves those files.
- `--json` prints the result document (`schemaVersion: 1`, camelCase) to stdout, and `--json <path>.json` writes it to that path. Either way the run is quiet: no progress lines and no table.
- The report is published as a private artifact when the account allows it, unless `--no-publish` is given. A run started from inside a Claude Code session stays local (#html-report).
- `--keep-temp` keeps each run's sandbox folder and prints its path.
- The exit code is 0 when every case reaches `--threshold` (default 1.0). It is 1 for a low score, a load error or a refused trust, and 2 for a partial run (#run-evals-in-ci).

UNVERIFIED for 2.1.319: whether `--output-dir` and `--keep-temp` exist in that version. The docs page is not pinned to it, and the help text was not readable.

## Decision line for v-11

- **Full:** release-notes is the only case `claude plugin eval` can run in full.
- **Partial:**
  - clean-csv, without `rows`.
  - fix-total, without `sum`. Its carried `script` file_exists grader also always fails, because the scaffold makes total.sh, so the case cannot pass at the default threshold until that grader is dropped.
- **Not at all, 16 cases:** both agent-reviewer cases, the three deliver cases, the six execute cases, memory-skill, preview-skill, questions-skill, routing-skill and views-skill.
  - Their fixtures exit 1 without `GRID_HOME`, and their checks are commands over `$GRID_HOME`, which no built-in grader can read.
  - 14 of them have no grader left at all.
  - 10 of them test features main has removed (execute ×6, memory, preview, routing, views).

That is enough to plan the harness removal as its own scope, provided that scope first decides what happens to those 16 cases: rewrite them for the workspace, retire them, or keep them outside the built-in. One live run of release-notes on 2.1.319 would confirm the format the docs describe.
