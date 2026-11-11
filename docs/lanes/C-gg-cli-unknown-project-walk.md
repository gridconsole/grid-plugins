# C-gg-cli-unknown-project-walk

Base: b87fc36458332a9e2bd680ff9e3ed5a896638900 (main; the brief quotes 4e736654, which is an ancestor). No test on the base walks contract/commands.json for this case (grep of internal/cli: only TestStableErrorCodes, error_codes_test.go:16-74, three absent-project argvs).

Production code is untouched. One new file: `internal/cli/gg_unknown_project_walk_test.go`.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Covers the contract | HOLDS | `go test -count=1 -v -run TestGgUnknownProject ./internal/cli/` exit 0: `--- PASS: TestGgUnknownProjectWalkCoversTheContract`. The walked set is read at test time with the typed loader (`ggProjectMembers`, gg_unknown_project_walk_test.go:94) and compared with a second, untyped reading of the file (`ggProjectMembersRaw`, :117): 81 members. Mismatch proof: with `if member == "where" { continue }` in the walk loop, the run is `FAIL: walked plus skipped = 80 members, contract/commands.json has 81 / contract only: [where]`, exit 1; `git checkout -- internal/cli/gg_unknown_project_walk_test.go` restored it. |
| 2 Exit and stderr | HOLDS | TestGgUnknownProjectWalk (:354): every member runs with `--project absent`, plain and `--json`; exit 0, a changed exit, changed stderr or non-empty plain stdout is a failure (`ggProjectWalkRun`, :216-:316). Members that fail are in ggProjectKnownOpen (row 4). |
| 3 Envelope | HOLDS | Same test: stdout must be one JSON object with top-level `code` or `error.code` equal to `no_such_project`, read as error_codes_test.go:57-67; a new envelope's message equals stderr without its last newline. This is stricter than the brief: the code is required whether or not stderr names `absent` (many verbs say "project is not registered"), so a usage error never passes by saying nothing about the project. |
| 4 Known open | HOLDS | Map `ggProjectKnownOpen` (:31). The test fails for a misbehaving member not in the map, for a map member that now answers right, and for a map member not in contract/commands.json. After the walk `<gridHome>/projects/absent` does not exist and `project.Load` returns only `example` (both checked at the end of `ggProjectWalkRun`; a verb that creates `absent` is reported by name). Log line: `known open: 36 of 81`. |
| 5 Diff, run, lint | HOLDS | `git diff main --stat`: only `internal/cli/gg_unknown_project_walk_test.go | 384 +`. (This report is a second file, added by the runbook; it comes after that stat.) `go test -count=1 -v -run TestGgUnknownProject ./internal/cli/`: `--- PASS: TestGgUnknownProjectWalk`, `ok github.com/gridconsole/grid-plugins/internal/cli 0.301s`. `make lint` exit 0 (`go vet ./...`). |
| Red on base | HOLDS | The file is new, so the base has no such test. With the map empty the first run was red (49 members listed under a stricter first draft; 36 after the stderr-wording rule was dropped); the map is what turns it green. |
| Importers | HOLDS | `go list` shows `internal/cli` is imported by `cmd/grid` and `lib`. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/`: ok internal/cli 59.359s; cmd/grid no test files; ok lib 3.027s. |
| Windows, macOS | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `GOOS=windows go vet ./internal/cli/` and `GOOS=darwin go vet ./internal/cli/` exit 0. |
| No real home, no program | HOLDS | `ggProjectWalkRun` t.Setenv HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME to folders of the harness root (under t.TempDir()), t.Fatal when HOME is not under that root (:226), blanks GRID_ROLE, GRID_MEMBER, GRID_WORKER, GRID_LEASE, GRID_LEASE_EPOCH, GRID_JOB, GRID_WAKE, GRID_ATTEMPT, GRID_OWNER, sets `h.env.Getenv` to return "", and sets PATH to a folder holding only a link to git, so no claude, systemd or browser can start. The test starts no child process of its own. |
| Skip map | HOLDS (empty) | `ggProjectSkip` is empty: with the PATH above every member, including routine run/schedule and session start, failed before any call; none touched the machine. A skip entry must name a `.go:` path:line and a member of the file, else the covers test fails. |
| No guard on agents | HOLDS | Test file only; no production code. |
| Do not edit existing tests | HOLDS | guestParityCommands is read, not changed. `git diff main --stat` shows no other file. |

## FINDING rows: known open: 36 of 81

Seen on the base, `--project absent` (plain / with `--json`):

| Members | What is seen |
|---|---|
| plan check, diff, hash, items, migrate, write; job ack, blocked, cancel, next, order, post, result, tree; feed tick (14) | exit 1; stderr names the project; with `--json` stdout is empty: no envelope. |
| workflow show, workflow write (2) | exit 1, stderr "project \"absent\" is missing or ambiguous"; stdout empty with `--json`. |
| job render, show, verify, trace (4) | exit 1 plain; with `--json` stderr is empty (they print their own JSON), so exit/stderr differ from plain. |
| job lint (1) | plain run prints the lint report on stdout, exit 1, stderr empty. |
| brain accept, changes, check, index, keep, propose, revert, search, show, supersede, topics (11) | with `--json` stderr is `{"error":"project \"absent\" is not registered"}`, plain stderr is `brain: project "absent" is not registered`: stderr differs. |
| task decline, task rename (2) | code `no_such_task`: the task operand is checked before the project. |
| feed types (1) | exit 0, the type list, whatever the project. |

The other 45 members answer `no_such_project` in the `{"error":{"code","message"}}shape (or a verb's own envelope with a top-level `code`), among them where, task delete/list/move/new/restore, lease, feed fold/handover/post/read/state, person, routine, agent set, claude settings show, skill, session, intent, signal.

## What changed

- New: internal/cli/gg_unknown_project_walk_test.go (helpers `ggProject*`, tests `TestGgUnknownProject*`). The argv per member is the guestParityCommands base, else one placeholder per positional; `{task}` -> `T-99`, `{file}` -> a plan file; required flags get `parity`; then `--project absent`.
- New: this report.

## NOT VERIFIED on a live screen

Nothing here is a UI row. Not run: any real browser, systemd timer or network publish (PATH has only git).

Open notes: `task decline`/`task rename` may be right (task before project); the owner decides which of the 36 are bugs. The map must be shortened as they are fixed.
