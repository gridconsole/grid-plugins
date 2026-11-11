# L-plugin-skill-agent-files: proof table

Lane branch lane-L-plugin-skill-agent-files. New file: internal/packaging/plugin_files_test.go. No .md file was changed.

| Row | Status | Proof |
|---|---|---|
| 1. Front matter of every agents/*.md and skills/*/SKILL.md (closed `---` block of `key: value` lines, read like skill_files_test.go, `name` equals file or folder name, one-line non-empty `description`, agents set `model` and `tools`) | HOLDS | `go test -count=1 -v -run TestShipped ./internal/packaging/` exit 0; check in `frontMatter` and `checkPluginFiles`, plugin_files_test.go:61 and :117; 3 agents and 9 skills read; `: ` in agents/builder.md:3 and skills/browser/SKILL.md:3 is accepted unquoted and unedited |
| 2. Every entry in an agent's `skills:` list is a folder under skills/ with a SKILL.md | HOLDS | same run, exit 0; plugin_files_test.go:117 (`skills: names ...`); builder: run-job, reviewer: review-job, orchestrator: plan-jobs all exist |
| 3. Every backticked `grid <group> [<verb>]` names a group and verb in commands.json, and every `--flag` in the same span is a flag of that subcommand | HOLDS, one flag left out (see NOT DONE below) | same run, exit 0, log line: `3 agents, 9 skills, 84 backticked grid commands checked against contract/commands.json`. A span with a placeholder (`grid <verb> --help`, skills/brain-keeper/SKILL.md:59) and a bare `grid` is not counted. A group with no verb (`grid task`, `grid signal`, `grid agent`) is checked for the group only. |
| 4. Deliberately broken fixture makes the checks fail with file and bad name | HOLDS | `TestPluginFileChecksBiteOnBrokenFixture` (plugin_files_test.go:259), exit 0, and it asserts each message. Messages printed: `agents/broken.md: skills: names "nope", but skills/nope/SKILL.md does not exist`; ``agents/broken.md:9: `grid job fetch <job>` names verb "fetch", which is not a subcommand of grid job``; `agents/noname.md: name is "other", want "noname"`; ``skills/ok/SKILL.md:5: `grid job show --nosuchflag` uses flag --nosuchflag, which grid job show does not have``; ``skills/ok/SKILL.md:5: `grid nogroup x` names group "nogroup"``. A well-formed skill with `: ` in its description is not flagged. |
| 5. `go test -count=1 ./internal/packaging/ ./internal/settings/` 0 fail | HOLDS | exit 0: `ok .../internal/packaging 8.529s`, `ok .../internal/settings 0.569s` |
| New test fails on base, passes on lane | HOLDS | The file is new, so base has no test to fail. On the first run, before the exemption below, the test failed on the real files: ``skills/setup/SKILL.md:35: `grid setup <project> --answers <file>` uses flag --answers, which grid setup  does not have`` (plus a group-only check I then dropped as wrong). The fixture test fails whenever the checks stop biting. |
| No existing test edited or deleted | HOLDS | `git diff origin/main..HEAD --stat` shows one added file plus this report |
| Package tests: changed package and its importers | HOLDS | `go list` importers of internal/packaging: internal/packaging/cmd. `go test -count=1 ./internal/packaging/cmd/` exit 0. Only a _test.go file was added, no non-test code changed, so internal/settings importers (cli, doctor, guard, hook) are unaffected and were not rerun; the orchestrator runs the whole module. |
| gofmt, vet | HOLDS | `gofmt -l internal/packaging` no output; `go vet ./internal/packaging/` exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | test file only; nothing runs at agent time |
| No personal paths, hosts or emails | HOLDS | test and report hold none |
| Commits: small, one sentence, no attribution | HOLDS | `git log -2 --format=%B` |

## NOT DONE

- skills/setup/SKILL.md:35 writes `grid setup <project> --answers <file>` (and `--answers -`). `--answers` is not in contract/commands.json for `grid setup` (flags there: --json, --remove, --yes), but the CLI reads it (internal/cli/settings.go:198, usage at :177). No listed flag fits, so by the brief the text is left unchanged and the assertion is left out: plugin_files_test.go:102 `contractGaps`. The fix belongs in commands.json (add `--answers` to `grid setup`; the contract generator is another lane's package), after which that map entry can be deleted.

## Edits to .md files

None. No skill or agent quoted a wrong verb or flag that commands.json could correct.

## What changed

- internal/packaging/plugin_files_test.go (new): `TestShippedAgentsAndSkillsAreWellFormed` reads ../../agents and ../../skills and the embedded `contractdata.Commands`; `TestPluginFileChecksBiteOnBrokenFixture` runs the same `checkPluginFiles` over in-test broken strings.
- docs/lanes/L-plugin-skill-agent-files.md (this report).

## NOT VERIFIED on a live screen

Nothing here is a UI row. No claude run was made; the checks are static reads of the files.

DONE
