# C-gg-cli-unknown-task-walk

Base: `b87fc36458332a9e2bd680ff9e3ed5a896638900` (main). No test on the base walks contract/commands.json for an unknown task (only `TestStableErrorCodes` rows at internal/cli/error_codes_test.go:26,28).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 `TestGgUnknownTaskWalkCoversTheContract`: set read from contract/commands.json at test time; differs by one member -> FAIL | HOLDS | `go test -count=1 -v -run TestGgUnknownTask ./internal/cli/` exit 0: `contract members: 81, walked: 81, skipped: 0`. Set read at internal/cli/gg_unknown_task_walk_test.go:`ggTaskMembers`. Mismatch proof: removed `"where"` from `ggTaskWalked`, run: `--- FAIL: TestGgUnknownTaskWalkCoversTheContract` / `contract members not walked or skipped: [where]; walked or skipped but not in the contract: []`; restored with `git checkout --` (status clean). |
| 2 `TestGgUnknownTaskWalk`: exit != 0, same exit and byte-equal stderr without `--json` | HOLDS | Same run, exit 0, 81 members walked. Members that break it (exit 0, or `--json` changes stderr) are in `ggTaskKnownOpen` (row 4). |
| 3 stderr names T-99 -> stdout is exactly one JSON object, code `no_such_task`, message = stderr minus final newline | HOLDS | Same test. 52 of 81 members pass it. A member whose stderr does not name T-99 also counts as a miss (none on base). |
| 4 `ggTaskKnownOpen`; fails for an unlisted miss, for a listed member that now answers well, and for a listed member gone from the contract; `known open: N of M`; no task folder left | HOLDS | Log line: `known open: 29 of 81`. Proof: listing `where` and a bogus `gone verb`: `where answers an unknown task as asked: remove it from ggTaskKnownOpen` and `ggTaskKnownOpen lists "gone verb", which is no longer in contract/commands.json`; restored. Walk run first with an empty map: it failed on each of the 29 members with a paste-ready line. After each member the test runs the `find projects -name task.md` equivalent (`ggTaskTasksIn`): none found. |
| 5 diff is the one file; `-v` run PASS on base; `make lint` | HOLDS | `git diff main --stat`: `internal/cli/gg_unknown_task_walk_test.go | 328 +` (plus this report, which the runbook asks for). `make lint` exit 0 (`go vet ./...`). Test PASS lines below. |
| Rule: new tests fail on base | n/a | The file is new, so the tests do not exist on base; the brief asks for PASS on base, which holds. The failing lines the walk gave with an empty map are the 29 FINDING rows. |
| Rule: packages that import the changed package | HOLDS | `go list` importers of internal/cli: `cmd/grid`, `lib`. `go test -count=1 ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (`ok internal/cli 57.474s`, `cmd/grid [no test files]`, `ok lib 6.380s`). |
| Rule: Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0; `GOOS=windows CGO_ENABLED=0 go vet ./internal/cli/` exit 0 (compiles the test). |
| Rule: no real home | HOLDS | Per member: `t.Setenv` of HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME (all under `h.root`) and PATH (an empty folder, so no child process can start); `t.Fatalf` when HOME is not under `h.root`; `h.env.Getenv` returns "". No child process is started by the test. |
| Rule: no guard, gate, cap; production code untouched | HOLDS | Only a test file and this report changed. |
| Skip map | HOLDS (empty) | No member needed a skip: with PATH empty and an unknown task, every member answered before any browser, timer, git or network call. `ggTaskSkipped` is empty; the coverage test still checks that any entry names a `.go:` path. |
| RealPlugin tests | n/a | No row names one. |

## Test output (`go test -count=1 -v -run TestGgUnknownTask ./internal/cli/`, exit 0)

```
    gg_unknown_task_walk_test.go:152: contract members: 81, walked: 81, skipped: 0
--- PASS: TestGgUnknownTaskWalkCoversTheContract (0.00s)
    gg_unknown_task_walk_test.go:327: known open: 29 of 81
--- PASS: TestGgUnknownTaskWalk (0.30s)
PASS
```

## FINDING rows: `ggTaskKnownOpen` (member -> exit, what was seen on base)

`known open: 29 of 81`.

No envelope on stdout with `--json` (stderr has the message, stdout empty; the verb does not wrap `where.ErrTaskNotFound` or call `env.recordError`):

| Member | Exit | Seen on stderr |
|---|---|---|
| plan check, plan diff, plan hash, plan items, plan migrate, plan write | 1 | `plan: task T-99 not found in project example` (all six used `--task T-99`) |
| job ack, job blocked, job cancel, job next, job order, job post, job result, job tree, feed tick, usage | 1 | `task T-99 not found in project example` |
| question add, question ask, question list, question sync, question tick, question withdraw, answer | 1 | `question: task T-99 not found in project example` |

Own JSON object instead of the contract envelope: stdout `{"ok":false,"findings":[],"error":"task T-99 not found in project example"}`, stderr empty with `--json`, so `--json` changes stderr (row 2) and there is no `code` (row 3):

| Member | Exit |
|---|---|
| job lint, job render, job show, job verify | 1 |

Exit 0 on an unknown task:

| Member | Exit | Seen |
|---|---|---|
| feed types | 0 | `{"types":[{"type":"ack",...` (ignores `--task T-99`) |
| signal act (`--act add --task T-99`, note exists) | 0 | `{"result":"updated","id":"S-01ARZ3NDEKTSV4RRFFQ69G5FAV","project":"example","note":...` (links the note to a task that does not exist) |

The 52 members that answer as asked include all `task *`, `where`, `lease *`, `feed fold/handover/post/read/state`, `deliver`, `approve`, `land`, `brain *`, `proof *`, `history add`, `person approvers`, `session *`, `intent *`, `usage post/show`, `artifact *`.

## What changed

One new test file, `internal/cli/gg_unknown_task_walk_test.go`: `TestGgUnknownTaskWalkCoversTheContract`, `TestGgUnknownTaskWalk`, helpers `ggTask*`, maps `ggTaskWalked`, `ggTaskSkipped`, `ggTaskKnownOpen`. Argv comes from `guestParityCommands` (read only), with a T-99 operand for members whose contract args hold `<task>`, else `--task T-99`; two members needed their own argv (`question withdraw` uses `Q-1`, `signal act` uses `--act add` with a note written first). The walk writes the grid home `VERSION` file as guestParityFixture does. Production code is not changed.

## NOT VERIFIED on a live screen

Nothing here is UI. The app was not run against these verbs; the walk runs `Run` in process on a temp home.
