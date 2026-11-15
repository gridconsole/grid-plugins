# P-r23-signals-routine: the signals skill under the routine's permission mode

Brief row: `/srv/gridwork/tmp/hand/prev/U-chk-plugin-rev-skills.md` row 1. Only row 1 is this lane's.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. Row 1 confirmed or NOT A FAULT | CONFIRMED, by reading the code and by a Go test (no routine was run) | `skills/signals/SKILL.md:18` gave `printf '%s' "<body>" \| grid signal add ...`, and `:22` gave `cd "$GRID_HOME" && printf ... \| grid signal add ...`. `internal/routine/execute.go:192` starts claude with `--allowedTools "Read,Glob,Grep,Bash(grid brain:*),Bash(grid signal:*)" ... --permission-mode dontAsk`; `:195` sets `cmd.Dir = s.GridHome`. A Bash allow rule matches the start of the command, so a command that starts with `printf` or `cd` is not allowed. On main, `go test -count=1 -run TestR23 ./internal/routine/` exit 1: `r23_signals_allow_test.go:76: SKILL.md:18: printf '%s' "<body, trimmed ...>" \| grid signal add ... does not start with an allowed prefix ["grid brain" "grid signal"], so dontAsk refuses it`; `:81: SKILL.md:18 ... holds "\|"` and `holds "printf"`; the same four lines for `SKILL.md:22`, which also holds `"&&"` and `"cd "`. 8 failing lines. |
| 2. Fix and test | save-command half HOLDS; source-tools half NOT DONE (see Open) | `TestR23SignalsSkillCommandsPassTheRoutineAllowList` (`internal/routine/r23_signals_allow_test.go`): fails on main as above; on the head `go test -count=1 -run 'TestR23\|TestQh' ./internal/routine/ ./internal/cli/` exit 0, `ok internal/routine`, `ok internal/cli`. It runs the real `Execute` argv, parses the `Bash(<prefix>:*)` rules, and walks every `grid signal ...` span of the skill. It also pins that the routine runs in the Grid home, which is why the Personal form needs no `cd`. |
| 2b. Personal form from the Grid home | HOLDS | `TestR23PersonalSignalAddFromGridHome` (`internal/cli/r23_signals_personal_test.go`): `--project personal` exits 1 `project is not registered` from the Grid home and from a project folder; the no-`--project` form the skill gives exits 0 from the Grid home. On main it fails at the span search (main's personal line has no `grid signal add ` span; not run on main). |
| Deleted test | `TestQhPersonalFlagRefusedEverywhere` (`internal/cli/qh_signals_quoted_refusals_test.go`) | Its last half required a span that starts with `cd "$GRID_HOME" && `, the form row 1's fix removes. Its own commit. `TestR23PersonalSignalAddFromGridHome` keeps every other assertion of it (refusal text and exit, "from any other folder" absent, recommended form exits 0) and adds that the line holds no `cd`. `TestQhSignalsQuotedRefusals` is untouched and passes. |
| 3. go test of changed packages and importers | HOLDS | `go test -count=1 . ./cmd/grid ./internal/cli ./internal/job/... ./internal/routine ./lib ./internal/packaging ./internal/contract ./internal/settings` exit 0, every line `ok` (cli 338s, lib 101s, routine 8.1s, job 18.9s, jobcmd 22.2s, packaging 24.8s). The importers of `internal/routine` and the root package, from `go list`: root, `cmd/grid`, `internal/cli`, `internal/job`, `internal/job/jobcmd`, `lib`. |
| 3. make lint | HOLDS | `make lint` exit 0 (`go vet ./...`, no output). |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| 3. Walk tests | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` exit 0, `ok internal/cli 0.458s`; `go test -count=1 ./internal/job/lint/` exit 0, `ok 1.379s`. |
| `git fetch origin && git merge origin/main` | HOLDS | `Already up to date.` before the last proof run. |
| App-repo checks (`make webui-test`, guest walk tables) | not run, not asked | They belong to the app repo. |
| Tests that need the built plugin (`*RealPlugin*`) | not run, not asked | The brief names none. |
| No guard, cap or limit added | HOLDS | The diff adds none: it edits `skills/signals/SKILL.md` text and tests. `execute.go` is untouched, so the existing `--max-turns` and `--max-budget-usd` stay as they were (row 9 of the findings, not this lane's). |
| No attribution lines | HOLDS | Commit messages are one sentence with no trailer. |

## What changed

- `skills/signals/SKILL.md`, step 3: the save command is `grid signal add ...` alone, with the body after it in a quoted here-document (`<<'GRIDBODY'`); the text says never to put `printf`, `echo`, `cd` or a pipe in front.
- `skills/signals/SKILL.md`, the Personal line: no `cd "$GRID_HOME" &&` and no `printf`; a routine already runs in the Grid home (`execute.go:195`).
- `skills/signals/SKILL.md`, step 1: says that a routine run has only Read, Glob, Grep, `grid brain` and `grid signal` open, so the source is the files the routine names; when it names none, say so and stop. This states what `execute.go:192` does.
- Tests: `internal/routine/r23_signals_allow_test.go` (new), `internal/cli/r23_signals_personal_test.go` (new, replaces `TestQhPersonalFlagRefusedEverywhere`).

## Open

- Row 1's second half, "add the tools for the named source to the routine's allow list", is NOT DONE and is not a build error of this lane: a routine has no field that names a source or a tool (`Entry` in `internal/routine/store.go:18` holds name, every, run, workflow, machine), and the routine's prompt (`execute.go:188-191`) names only the skill and the project. A fixed `mcp__...` rule would also allow send and delete tools, against the skill's "read only", and the default argv is pinned by `TestProcessRoutineLimits` (`limits_test.go:38`). The split: a follow-up lane adds an opt-in routine field for read tools (front matter key, `grid routine add` flag, schema and README rows, the schedule-approval hash) and appends them to `--allowedTools`; it needs the owner to say how a routine names its source. Until then a scheduled signals routine reads files it names and nothing else; step 1 of the skill now says so.
- Rows 2 to 18 of the findings file are not in this brief.

## NOT VERIFIED

- That Claude Code accepts a command with a here-document after `grid signal add ...` under `Bash(grid signal:*)` in dontAsk mode. It needs a live routine run; the prefix rule is read from `execute.go:192`, not run.
- No routine was run; the denial of `printf` and `cd` comes from reading the allow list and the mode, as in the findings file.
