# P-r23-frontmatter: description and front matter rules hold everywhere and the tests walk every file

Repo: plugin. Branch lane-P-r23-frontmatter, from main (`git fetch origin && git merge origin/main`: Already up to date). Findings: rows 13, 14, 17, 18 of U-chk-plugin-rev-skills.md.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 13 confirm | CONFIRMED | `grep -rn -m1 '^description:' agent-templates agents skills` over 29 files: only `agent-templates/grid-curator.md:3` has three sentences, ending `Asks no person.`; `er_descriptions_test.go` lists 8 files. |
| 13 fix | HOLDS | `agent-templates/grid-curator.md:3` now says `..., asking no person. Use at deliver and ...`. New `TestErDescWalkEveryFile` (`er_descriptions_walk_test.go`) globs `skills/*/SKILL.md`, `agents/*.md`, `agent-templates/*.md` (29 files). On main (fix files stashed): `--- FAIL: TestErDescWalkEveryFile` / `agent-templates/grid-curator.md: 3 sentences, want at most 2` / `last sentence does not start with "Use "`. On head: `ok github.com/gridconsole/grid-plugins`, exit 0. `TestErDescCuratorTemplateKeepsAsksNoPerson` keeps the meaning (passes on both). |
| 14 confirm | CONFIRMED | `skills/run-job/SKILL.md:4` is `topics: delivering`, `skills/review-job/SKILL.md:4` is `topics: reviewing`; `skills/brain-keeper/SKILL.md:75` and `internal/brain/check.go:45,338` allow only name, description, when_to_use, metadata. `topics/README.md:14` documents `topics`. Nothing in Go reads the key, so it starts nothing. |
| 14 fix | HOLDS | `topics` added at brain-keeper:75, `internal/brain/check.go:45` and its message at :338, and `contract/README.md:669` (the check and the doc must agree, or a curator following the doc would meet a `skill_field` finding). Tests `TestCheckSkillTopicsKeyIsClean` and `TestShippedSkillKeysAreAllowedKeys` (`internal/brain/check_topics_key_test.go`). On main: `codes = [skill_field], want []: ... Text:topics` and `../../skills/run-job/SKILL.md: front matter key "topics" is outside the keys the brain check allows` (same for review-job). On head: `ok internal/brain`. The key is inert, so no guard, cap or hold is added or loosened beyond one inert key. |
| 17 confirm | CONFIRMED | `internal/packaging/plugin_files_test.go` on main: `readPluginFiles` reads `agents/*.md` and `skills/*/SKILL.md` only (log: `5 agents, 12 skills, ... backticked grid commands`); `checkPluginFiles` takes only backtick spans that start with `grid `. Unwalked on main: 12 `agent-templates/*.md`, 3 topic files, fenced `grid ...` lines (`skills/brain-keeper/SKILL.md:58-61`, `skills/browser/SKILL.md:19`) and `printf ... \| grid signal add` (`skills/signals/SKILL.md:18,22`). |
| 17 fix | HOLDS | `checkPluginFilesWith` plus `commandSpans` (fenced lines, the command after a pipe); `checkPluginFiles` keeps its signature. New `TestShippedTemplatesAndTopicsAreWellFormed` (log: `12 templates, 3 topic files, 80 grid commands checked`) and `TestPluginFileChecksReadFencesAndPipes` (`internal/packaging/frontmatter_walk_test.go`). On main (walker change stashed): `undefined: checkPluginFilesWith` at `frontmatter_walk_test.go:50` and `:76`, build failed. On head: `ok internal/packaging`. The real walk finds no bad flag, and `grid history add` is in the contract (the "not confirmed" item is now confirmed by the walk). |
| 18 confirm | NOT A FAULT for the behaviour, a wording gap for the text | `internal/braincmd/command.go:575` `proposalID`: a plain id takes the store's level (project when `--project` resolves, else home). So `grid brain accept --project <project> <id>` (brain-keeper:20) is right for a project proposal. `TestPlainIDFindsOnlyAProjectProposalWithProject` (`internal/braincmd/plain_id_level_test.go`) passes on main and head: plain id with `--project example` checks, shows and accepts `plain-p`; the same plain id for the home proposal `plain-h` is refused; `brain:home/plain-h` is accepted. The curator line "The plain id does not find a level proposal" is true for a home proposal and for a run with no project. |
| 18 fix (wording) | HOLDS | `skills/brain-keeper/SKILL.md:20` now says the plain id finds only a project proposal, a home proposal needs `brain:home/<id>`, and a `brain:` row always carries it. `TestBrainKeeperSaysWhenThePlainIDIsEnough`: on main `the plain-id accept line does not say a home proposal needs brain:home/<id>`; on head `ok internal/braincmd`. |
| go test of changed packages and importers | HOLDS, exit 0 | `go test -count=1 . ./internal/packaging/ ./internal/brain/ ./internal/braincmd/ ./internal/cli/ ./internal/hook/ ./internal/job/... ./internal/people/ ./internal/projectenv/ ./internal/routine/ ./internal/task/ ./internal/workflow/`: every line `ok` (list from `go list` of importers of internal/brain: braincmd, cli, hook, job, job/jobcmd, job/jobtest, people, projectenv, routine, task, workflow). |
| make lint | HOLDS, exit 0 | `make lint` prints `go vet ./...` and nothing else. |
| Windows build | HOLDS, exit 0 | `GOOS=windows CGO_ENABLED=0 go build ./...` no output. |
| macOS build | HOLDS, exit 0 | `GOOS=darwin CGO_ENABLED=0 go build ./...` no output. |
| Walk tests TestHsIpWalksEveryCommand\|TestIlGuestSiteList | HOLDS, exit 0 | `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `ok internal/cli`. |
| go test ./internal/job/lint/ | HOLDS | `ok internal/job/lint 3.847s`. |
| Tests *RealPlugin* | not run, not asked | Brief names none. |
| Whole-module test, cross-repo set | not run, not asked | The orchestrator runs them at landing. |

## What changed

- `agent-templates/grid-curator.md`: "asking no person" moved into the first sentence.
- `internal/brain/check.go`, `contract/README.md`, `skills/brain-keeper/SKILL.md`: `topics` joins the allowed skill front matter keys; brain-keeper:20 says when the plain id is enough.
- `internal/packaging/plugin_files_test.go`: walker helper extended (fences, pipes, doc files); old tests unchanged.
- New tests: `er_descriptions_walk_test.go`, `internal/brain/check_topics_key_test.go`, `internal/braincmd/plain_id_level_test.go`, `internal/packaging/frontmatter_walk_test.go`.

## NOT VERIFIED on a live screen

None; no UI. Row 14 went beyond the row's doc-only fix by also changing the `skill_field` allowlist in Go and the contract README, so that the check and the doc agree; the reviewer may judge that.
