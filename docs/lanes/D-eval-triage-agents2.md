# D-eval-triage-agents2: why the four real agent evals failed

Runs read from `evals-runs/<task>.json` (copies of the stored runs). No real run was made here: I cannot run `claude`. Nothing below is a real pass; the orchestrator reruns the four tasks.

## Failing checks (acceptance rows 1-3)

| Task / check | Cause | Evidence | What changed |
|---|---|---|---|
| agent-designer-spec / `no-code` | check (locale-dependent sort) | Check failed with `exit status 1`, while the stdout says only `task/spec.md` was written and nothing else was created. The check pipes `find` through `sort` and compares to a fixed string with `./design/README.md` before `./design/model.md`. That order only holds in the C locale; in `en_US` `model.md` sorts before `README.md`. This is the only task whose expected list mixes upper and lower case. I infer the locale; the stored run does not print the actual file list. | `evals/tasks/agent-designer-spec/checks.json`: `sort` is now `LC_ALL=C sort`. The check still demands exactly the five files, so it still fails if the designer writes code or any other file. |
| agent-explore-finds / `skips-vendored` | check (tests the word, not the behaviour) | stdout: "It skipped one match in `node_modules` as unrelated third-party code." The agent said it skipped the vendored match, and `! grep -q 'node_modules' answer.md` fails on that very sentence. | `checks.json`: now `! grep -Eq 'node_modules/[^ ]*:[0-9]+' answer.md`. It fails when the answer cites a `node_modules` path:line as a place of use, and passes when the answer only says it skipped it. The template (`Explore.md`) already says to skip vendored folders and is unchanged. |
| agent-library-answers / `answer`, `names-note`, `gives-fact`, `gives-why`, `brain-unchanged` | fixture (no Bash permission), not the agent or the checks | stdout: "Every `grid brain` command it tried (index, topics, and searches for "labels" and "tags") was refused because it needed approval." `claude -p` cannot ask, so every Bash call is refused and the agent never read the brain. `answer.md` was never written (`statat answer.md: no such file or directory`), which fails the gate; `brain-unchanged` fails only because its last clause requires `./answer.md` to exist. | `evals/tasks/agent-library-answers/setup.sh` now writes `.claude/settings.json` in the workspace allowing only `grid brain index --read-only`, `topics`, `search` and `show`: the read-only verbs the template names. The checks and the template are unchanged. `no-unrelated-note` and `template-read-only` already passed. |
| agent-operator-no-browser / `blocked-first` | agent template | `result.md:1` contains `No browser` (check `no-browser` passed, evidence `result.md:1`), yet `head -n 1 result.md \| grep -qx BLOCKED` failed. So line 1 held more than the word `BLOCKED` (stdout shows it as `**BLOCKED**`). The template said "post `BLOCKED` with `No browser` as the reason", and only the later line about the result body says the first line is the status word. | `agent-templates/grid-operator.md:14`: "Make the first line of your reply exactly `BLOCKED`, alone on the line. The next line gives `No browser` as the reason, then what you would have done." The check stays as it was: it matches the first-line status word the same template requires for `grid job result`. |

## Rules and checks

| Row | Result | Command / place |
|---|---|---|
| 4. fake-agent path | HOLDS | `go test ./internal/eval/... ./internal/evalcmd/... ./internal/settings/... ./internal/hook/...` exit 0 (these are the packages that read `evals/tasks` or `agent-templates`; no Go source changed) |
| 4. make lint | HOLDS | `make lint` (`go vet ./...`) exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| 4. go test ./... | NOT RUN | The whole-module test is run by the orchestrator at landing; no Go file changed. |
| New checks fail on base, pass on lane | PARTLY | Explore: stand-in agent that writes the three path:line hits plus "Skipped node_modules/lib/index.js as vendored." gives `20261110T234526Z-4bb42fc060cde16f agent-explore-finds ... passed 7/7 1/1 5.00/5`; on base `skips-vendored` fails on that answer (`grep -q node_modules` matches). Designer sort, library permission and operator wording can only be shown with a real run: NOT VERIFIED. |
| Existing tests edited or deleted | HOLDS | None edited, none deleted. |
| No guard, gate, cap or hold added | HOLDS | The only grant is a read-only allow list in a test fixture. The template change adds none. |
| Files touched | HOLDS | Only `evals/tasks/{agent-designer-spec,agent-explore-finds,agent-library-answers}`, `agent-templates/grid-operator.md` and this file. `evals-runs/` is in `.git/info/exclude`. |
| No personal paths, hosts or emails | HOLDS | none added |

## What changed

- `evals/tasks/agent-designer-spec/checks.json`: `LC_ALL=C sort` in `no-code`.
- `evals/tasks/agent-explore-finds/checks.json`: `skips-vendored` tests for a `node_modules` path:line, not the word.
- `evals/tasks/agent-library-answers/setup.sh`: project settings allowing the read-only `grid brain` verbs.
- `agent-templates/grid-operator.md`: first line of a no-browser reply is exactly `BLOCKED`.

## NOT VERIFIED

- All four tasks against the real agent: the orchestrator reruns them.
- Designer: I did not see the real file list, so the locale cause is inferred. If `no-code` still fails, print `find . -type f ! -path './.claude/*'` in a check's evidence and look.
- Library: whether this claude version loads workspace `.claude/settings.json` under `-p --permission-mode acceptEdits`, and whether the haiku agent then finds `tag-storage` from "labels". If settings are not loaded, the same allow list goes into `evalrun.sh` as `--allowedTools` (outside this lane's scope). The agent also looked for the Grid home under `/root/.grid`; it should use `$GRID_HOME`, which the runner sets. That is a template wording point if it persists.
- Operator: whether the model now puts `BLOCKED` alone on line 1 of the text that the outer session saves unchanged.
