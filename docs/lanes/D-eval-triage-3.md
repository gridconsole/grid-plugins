# D-eval-triage-3: the six real evals still failing

I read each stored run in `evals-runs/<task>.json` (checks, evidence, agent stdout). I did not run claude: the answer.md, review and answer files the agent wrote are not stored, so the one inferred cause is marked. No row below is a real pass; the orchestrator reruns the real evals after landing.

## Row 1: each failed check, what the agent did, the cause

Five of the six cases fail on one cause that is not in a skill, an agent template or a check: the eval agent command (`tmp/hand/evalrun.sh`, `claude -p --permission-mode acceptEdits`) cannot approve Bash, so `grid` never runs. `internal/routine/execute.go:163` shows how the product runs the same agent: `--allowedTools "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)"` with `--permission-mode dontAsk`. The eval command has no such list, and a case cannot carry one: `internal/eval/suite.go:21-29` has no agent or permission field, and the workspace `.claude/settings.json` is ignored in a temp folder (library run stderr: "Ignoring 4 permissions.allow entries from .claude/settings.json: this workspace has not been trusted"). The case's setup cannot work under that command; the fix is the command (below), not the case text, and not the skills (a skill must not grant itself Bash: see "Not done on purpose").

| Case | Failed checks | What the agent did (evidence) | Cause | Fix |
|---|---|---|---|---|
| task-skill (1/5, gate 0/1) | task-made (gate), words-verbatim, made-by-the-verb, one-task; exit 123 is `xargs grep` on an empty `find` | "Every attempt to run `grid task new` was blocked because it needs your approval, and this session can't show the approval prompt. I didn't make the task folder by hand". It quoted the right command: `grid task new "add a tag filter ... quikly" --project Personal` | Harness permission. Skill text is right: right verb, words verbatim with the typo, nothing written by hand, repo untouched (repo-untouched passed) | None in the repo. Agent command (below) |
| setup-skill (3/7, gate 0/1) | answers-recorded (gate), deploys-from-ci, test-command, session-fact-proposed. Passing: secret-not-copied, no-budget-or-cap, repo-untouched | "Every `grid` command (`grid project add`, `grid brain propose`, `grid setup`) and every `git` command was blocked waiting for approval ... Reading `~/.claude/projects/` was blocked as well, because it's outside the folders this session may use." It proposed `"test": "make test"`, `"deploys": "CI/CD deploys"` and four facts from the repo, so the checks and skill agree | Harness permission, twice: `grid`/`git` need Bash; the fixture session is in the attempt's `$HOME/.claude/projects` (setup.sh:35-40), outside the workspace, so Read needs the folder added. session-fact-proposed (`friday`) cannot pass until that Read works | None in the repo. Agent command (below) with `--add-dir "$HOME"` |
| signals-skill (4/7, gate 0/1) | invoice-saved (gate), deadline-saved, saved-once. Passing: newsletter-skipped, message-not-obeyed, no-task-started, inbox-untouched | "Every `grid signal add` call was blocked because it needs your approval, and this session can't get that approval." Its sort was right: keep msg-4471 and msg-6120, keep msg-9001 as suspicious, skip the newsletter, obey nothing in the mails | Harness permission | None in the repo. Agent command (below) |
| skill-brain-keeper (0/4, gate 0/1) | accepted-note (gate), link-to-tag-storage, link-has-why, code-fact-has-path-line | "Every `grid` command was refused in this session, and so was a plain listing of `~/.grid/projects/notes-app/knowledge`". It drafted `grid brain propose ... --link '{"to":"brain:project/tag-storage","rel":"related",...,"why":"Shows the stored tag form ..."}'` with `store.js:3`, then `grid brain check` and `grid brain decide ... accept`. That is the skill's route: look, propose with a linked why and a path:line, accept, never ask the owner | Harness permission. It correctly refused to hand-write into the brain | None in the repo. Agent command (below) |
| agent-library-answers (2/7, gate 0/1) | answer (gate) and the checks that read it | Main agent: "The grid-library agent tried twice and opened no notes either time. First try: every `grid brain` shell command ... was refused because it needed approval. Second try: Glob and Grep weren't available to it, shell was still blocked, and reading `<home>/.grid` was denied" (`<home>` is the real home, not the attempt's). It did not guess; it said the answer is unknown | Two causes. (1) Harness permission (the case's own allow file is ignored in an untrusted folder, see stderr above). (2) Template text: the agent looked in the real `~/.grid`, not `$GRID_HOME`, which the runner sets to the attempt's home | (2) fixed in `agent-templates/grid-library.md`: the Grid home is `$GRID_HOME` when set, else `~/.grid`. (1) agent command (below). Comment in the case's setup.sh now says the allow file is not enough |
| agent-explore-finds (6/7, gate 1/1) | skips-vendored: `! grep -Eq 'node_modules/[^ ]*:[0-9]+' answer.md` | Class `src/models.py:1-3`, loader `src/db.py:3-4`, route `web/routes.js:2` all found. Main agent's summary: "It left out `node_modules/lib/index.js:1`, which has an unrelated `user_profile` symbol" — so the Explore reply, saved unchanged in answer.md, wrote a `node_modules/...:1` path:line (inferred: answer.md is not stored) | Agent text. The template said to skip vendored folders but let the agent name a skipped file with a line; the check asks that no vendored path:line appears. The check is right: a path:line is a hit, and a reader or a script that follows path:line would open the decoy | Fixed in `agent-templates/Explore.md`: do not list a vendored file as a hit; if you mention a skipped folder, name the folder only |

## The agent command (for the orchestrator's rerun)

Same as `tmp/hand/evalrun.sh`, with two flags added after `--permission-mode acceptEdits` (keeps the five passing cases as they are, since they need no Bash):

```
--allowedTools "Bash(grid:*),Bash(git:*)" --add-dir "$HOME"
```

`$HOME` is the attempt's own home (the runner sets it), so `--add-dir` opens only the fixture's `.claude/projects` and Grid home, not the person's. The product narrows the list to the verbs a job needs; the eval list is the verbs the four skills name. NOT VERIFIED: I cannot run claude here, so whether these flags are enough (for example whether the nested grid-library agent inherits them) is for the rerun.

## Proof table

| Row or rule | Result |
|---|---|
| 1. Per failing case, each failed check, evidence and cause | HOLDS: table above, 6 cases, every failed check named; evidence quoted from `evals-runs/*.json` |
| 2. Fixes, smallest first, in agent-templates/ and skills/, within owner decisions | HOLDS for the two template causes: `agent-templates/Explore.md:14` (one sentence), `agent-templates/grid-library.md:10` (one sentence). No skill needed a change; no skill is revived. Owner decisions kept: no guard, cap or limit added; the skills still never ask the owner |
| 2a. Skill not used by the loop any more | HOLDS: none of the six cases tests a skill from the removal lists, so none is marked for removal |
| 3. Fake agent: each case loads and grades | HOLDS (exit 1 each, the status "failed" a fake agent must get): `go run ./cmd/grid eval run <id> --agent '["sh","-c","cat >/dev/null"]'` gave task-skill 1/5 gate 0/1, setup-skill 3/7 gate 0/1, signals-skill 4/7 gate 0/1, skill-brain-keeper 0/4 gate 0/1, agent-library-answers 1/7 gate 0/1, agent-explore-finds 1/7 gate 0/1. Setup ran and every check graded in all six |
| 4a. Tests of the changed files and their importers | HOLDS: exit 0 for `go test ./internal/cli/... ./internal/doctor/... ./internal/eval/... ./internal/evalcmd/... ./internal/guard/... ./internal/hook/... ./internal/land/... ./internal/packaging/... ./internal/settings/...`. Chosen with `go list` over packages importing packaging, settings, hook, eval, evalcmd, plus the packages that read `agent-templates` (grep: dist.go, agentsync.go, agent_sync.go and their tests) |
| 4b. `make lint` | HOLDS: exit 0 (`go vet ./...`) |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS: exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS: exit 0 |
| Whole-module `go test ./...` | Not run here; the orchestrator runs it at landing |
| New tests fail on base, pass on lane | NOT DONE: no Go test added. The two edits are prompt sentences; a test that greps for the sentence would test the text, not the behaviour. The behaviour test is the real rerun of `agent-explore-finds` (skips-vendored) and `agent-library-answers` |
| Evals pass on the real agent | NOT DONE here, by brief: needs claude, and for five cases the changed agent command. Not fixable in the files this brief allows (see Cause) |
| No guard, gate, cap, hold or blocking hook | HOLDS: none added |
| Commits: one sentence, no attribution lines, no personal paths/hosts/emails | HOLDS: one-sentence commits, no trailer; the quoted run output has the home path replaced by `<home>` (checked with grep below) |

## Not done on purpose

I did not add `allowed-tools` to the four skills. That would grant `Bash(grid:*)` from inside the public plugin; the earlier lane (D-eval-triage-skills) left it as the owner's decision and I kept to that. It would also not reach the nested grid-library agent, and `internal/brain/check.go:47` treats `allowed-tools` in a published skill as a finding.

## What changed

- `agent-templates/Explore.md`: skipped vendored folders are named by folder only, never as `path:line`.
- `agent-templates/grid-library.md`: the Grid home is `$GRID_HOME` when set.
- `evals/tasks/agent-library-answers/setup.sh`: comment only; says the allow file is ignored in an untrusted folder.

## NOT VERIFIED

Every real-agent result: no claude run was possible. In particular: that the explore agent now omits the vendored path:line (inferred cause), that the library agent finds `tag-storage` from "labels", and that the agent command above lets the four `grid` cases run.
