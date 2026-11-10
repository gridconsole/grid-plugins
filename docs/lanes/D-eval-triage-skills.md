# D-eval-triage-skills: why the real skill evals fail

I read each stored run with `go run ./cmd/grid eval show -json <run id>`. The run files under `~/.grid` could not be opened with cat or Read in the builder shell. The run output carries each check's status and reason, and the agent's stdout. It does not carry the files the agent wrote (`review.md`, `verdict.md`), so the two file-based causes below are inferred from the check, the skill text and the agent's stdout. I did not run claude, so no row below is a real pass.

## Failing checks and causes

| Task, check | Cause | Evidence | What I changed |
|---|---|---|---|
| setup-skill: answers-recorded, deploys-from-ci, test-command, session-fact-proposed | The eval agent session (`claude -p --permission-mode acceptEdits`) cannot run `grid`; the checks are right | stdout: "Every `grid` command (`project add`, `session list`, `brain propose`, `setup`, even `--help`) needs your approval, and this session can't ask for it." Check stderr: `grep: .../projects/notes-app/setup-answers.json: No such file or directory`, exit 2 | Nothing in this repo. NOT DONE here: the fix is in the agent command of `tmp/hand/evalrun.sh` (see open items) |
| signals-skill: invoice-saved, deadline-saved, saved-once | Same: no `grid signal add` could run. The agent picked the right two messages and skipped the newsletter and the injected message | stdout: "Every `grid signal add` command needs your approval, and this session can't ask for it, so nothing was written to your Grid home." | Nothing in this repo (same fix) |
| skill-brain-keeper: accepted-note, link-to-tag-storage, link-has-why, code-fact-has-path-line | Same: `grid brain propose` and `show` held for approval. The agent's drafted command has the link with a why and `store.js:3`, so the skill text works | stdout: "Every `grid` command (`grid brain search`, `show`, `propose`) was held for approval" | Nothing in this repo (same fix) |
| task-skill: task-made, words-verbatim, made-by-the-verb, one-task | Same: `grid task new` held for approval. Exit 123 is `xargs grep` getting an empty `find` result, a consequence of no task existing, not a wrong check | stdout: "Creating it needs your approval... This is the command I tried to run: `grid task new \"add a tag filter ... quikly\" --project Personal`" | Nothing in this repo (same fix) |
| skill-browser: no-pass | Check was wrong. `grep -Eiq '(^\|[^A-Za-z])PASS([^A-Za-z]\|$)'` is case-insensitive and matches the word "pass" anywhere, so a verdict that says "none counts as a pass" fails. The README asks that no row is a pass | stdout: "all three as NOT VERIFIED, and none of them counts as a pass." The other four checks passed (`verdict.md` written, 3 NOT VERIFIED, widths named) | `evals/tasks/skill-browser/checks.json`: the check now fails only when a result cell or `label: value` is PASS (`(^\|[\|:])[[:space:]]*[*]*PASS[*]*[[:space:]]*([\|]\|$)`). Tried on four lines: `\| R1 \| PASS \| seen \|` and `R2: **PASS**` match; `\| R1 \| NOT VERIFIED \| none counts as a pass \|` and `Never write PASS for what you did not see` do not |
| skill-visual-design-review: format, at-most-ten | Skill text. The output format was a bullet list of bold labels, with no template, and no word about the no-browser case, so the agent used its own layout without the literal `Severity` / `Where` / `Fix` labels (inferred: `review.md` is not stored). The checks test what the README asks (fixed format, at most 10 findings) and stay as they are. The agent did find contrast, monospace, small text and tap target and named the three widths | stdout: "The review lists four high-severity problems ... three medium and two low ... the measurements are in a table"; checks `format` and `at-most-ten` exit 1; `found-*`, `three-widths`, `screenshots-not-verified` passed | `skills/visual-design-review/SKILL.md` Output format: a literal block with the five labels, one `Severity:` line per finding, no table, and one sentence for the no-browser case |

## Proof table

| Row or rule | Result |
|---|---|
| 1. One row per failing check | HOLDS: the table above covers all 6 tasks and 18 failing checks |
| 2. Each wrong check fixed, none weakened | HOLDS for skill-browser/no-pass (still fails on a PASS result cell, see the four-line test). The other failing checks are not wrong and were left as they are |
| 3. Skill fixes are the smallest change | HOLDS: one edit in `skills/visual-design-review/SKILL.md`, output format only |
| 4a. `go test ./internal/eval/...` | HOLDS: exit 0 (`ok github.com/gridconsole/grid-plugins/internal/eval`) |
| 4b. Tests of the changed packages and the packages that import them | HOLDS: exit 0 for `./internal/eval/... ./internal/evalcmd/... ./internal/packaging/... ./internal/settings/... ./internal/job/... ./internal/cli/...`. These are the packages that read `evals/tasks` or `skills/` by embed or path (found with grep over `*.go`, not `go list`) |
| 4c. `make lint` | HOLDS: exit 0 (`go vet ./...`) |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS: exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS: exit 0 |
| Whole-module `go test ./...` | Not run here; the orchestrator runs it at landing |
| New tests fail on base and pass on lane | NOT DONE: no Go test was added. The check change is covered only by the four-line grep test above (the old pattern matches all four lines, the new one only lines 2 and 3) |
| Four tasks that need `grid` to run (15 checks) | NOT DONE by this lane: cause is the harness permission, outside the files this brief allows |
| No guard, gate, cap, hold or hook added | HOLDS: none added |
| No attribution lines, no personal paths | HOLDS: commit messages are one sentence; the doc names no host or email |

## Open items for the orchestrator

1. **The big one (15 of 18 failing checks).** The agent command in `tmp/hand/evalrun.sh` runs `claude -p --permission-mode acceptEdits`, which refuses every Bash command not already allowed, so `grid` never runs. Add `--allowedTools 'Bash(grid:*)'` (and `Read` of the Grid home and `~/.claude/projects` for setup-skill) to the agent command, or use a settings file that allows them, then rerun setup-skill, signals-skill, skill-brain-keeper and task-skill. Until then those four say nothing about the skills. I did not add `allowed-tools` to the skill files: that grants permissions in the public plugin and is the owner's decision.
2. Rerun skill-browser and skill-visual-design-review with the real agent; the visual-design-review cause is inferred, so if `format` still fails, read `review.md` from the run workspace.
3. If a `grid eval` run should keep the files the agent wrote, store them with the run; they were not in the stored JSON.

## NOT VERIFIED on a live screen

All six real evals: no claude run was possible here.
