# C-gm-question-usage-unknown-task-envelope

`question add|ask|list|sync|tick|withdraw`, `answer` and `usage` answer an unknown task with the
`{"error":{"code":"no_such_task","message":...}}` envelope under `--json`. Before, each printed the
lookup error on stderr and wrote nothing on stdout.

Code head: 5891f58c (test commit 60ed4b47, production commit 5891f58c).

## Proof table

| Row | Result | Proof |
| --- | --- | --- |
| 1a. Test FAILs on all eight cases on the base | HOLDS | `go test -count=1 -v -run TestGmQuestionUsageUnknownTask ./internal/cli/` on the base source with only the new test file added: exit 1. FAIL lines below. |
| 1b. After the change it PASSes: exit 1 plain and `--json`, byte-equal stderr, stdout exactly the envelope | HOLDS | Same command on the lane: exit 0, `--- PASS: TestGmQuestionUsageUnknownTask` and eight PASS subtests. The test asserts exit 1 in both runs (`gm_question_usage_envelope_test.go:46,58`), stderr byte-equal (`:58`), plain stdout empty (`:53`), and `assertErrorOutput(..., "no_such_task")` (`:61`), which requires the object to hold only `error` with `code` and `message` equal to stderr without its newline. |
| 2. HOME, GRID_HOME, CLAUDE_CONFIG_DIR, CODEX_HOME set with t.Setenv under the test folder; Fatal when HOME is not under it; no child process | HOLDS | `gm_question_usage_envelope_test.go:34-39`: the four are set with `t.Setenv` (`:36`) to folders under `h.root` (`gridhome.Real(t.TempDir())`); `t.Fatalf` (`:38-39`) when `os.Getenv("HOME")` is not under `h.root`. The file imports no `os/exec`; it calls only `h.run`, which runs `Run` in process. |
| 3. `ggTaskKnownOpen` no longer holds the eight; `TestGgUnknownTask` PASSes | HOLDS | `go test -count=1 -v -run TestGgUnknownTask ./internal/cli/` exit 0: `--- PASS: TestGgUnknownTaskWalkCoversTheContract`, `--- PASS: TestGgUnknownTaskWalk`, log `known open: 21 of 81` (was 29 of 81). `git diff main -w` of the file shows only the eight removed lines; the rest of the 50-line diff is gofmt re-aligning the map. |
| 4. Existing question answer still works: `go test -count=1 ./internal/question/ ./internal/cli/` exit 0 | HOLDS | Both `ok` (question 3.2s, cli 82.0s) in the importer run below, exit 0. `TestStableErrorCodes` and `TestGuestProjectParity` run by name: both `--- PASS`. |
| 5a. `git diff main --stat` names only the five files in Touches | HOLDS | Five files: `internal/cli/gg_unknown_task_walk_test.go`, `internal/cli/gm_question_usage_envelope_test.go`, `internal/cli/question.go`, `internal/cli/usage.go`, `internal/question/command.go`. This report (`docs/lanes/<ID>.md`) is the sixth file, added by the runbook's Definition of Done in a last commit. |
| 5b. `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings). |
| 5c. Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| 5d. macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| Rule: new test fails on base, passes on lane | HOLDS | 1a and 1b. |
| Rule: no existing test edited to make it pass | HOLDS | The only edit to an existing test file is removing the eight `ggTaskKnownOpen` entries the brief names; the walk itself fails for a listed member that now answers well, so the removal is what the fix requires. No test deleted. |
| Rule: tests of changed packages and every importer | HOLDS | exit 0 for the list below. |
| Rule: no guard, gate, cap, hold or blocking hook on agents | HOLDS | `question.Env.RecordError` only tells the caller of an error already returned; it refuses nothing and changes no exit code or stderr. |
| Rule: no *RealPlugin* test needed | HOLDS | Brief names none; nothing built or run from the plugin. |
| Rule: small commits, one sentence each, no attribution lines | HOLDS | `git log` shows 60ed4b47, 5891f58c plus the report commit; no Co-Authored-By. |

### FAIL lines on the base (new test, base source)

```
=== RUN   TestGmQuestionUsageUnknownTask/question_add
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/question_ask
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/question_list
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/question_sync
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/question_tick
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/question_withdraw
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/answer
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
=== RUN   TestGmQuestionUsageUnknownTask/usage
    gm_question_usage_envelope_test.go:61: error JSON: unexpected end of JSON input:
--- FAIL: TestGmQuestionUsageUnknownTask (0.01s)
    --- FAIL: TestGmQuestionUsageUnknownTask/question_add (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/question_ask (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/question_list (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/question_sync (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/question_tick (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/question_withdraw (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/answer (0.00s)
    --- FAIL: TestGmQuestionUsageUnknownTask/usage (0.00s)
FAIL
FAIL	github.com/gridconsole/grid-plugins/internal/cli	0.013s
```

All eight reached the `assertErrorOutput` line, so each already passed the plain-run exit 1, the
stderr ending `T-99 not found in project example` and the `--json` exit and stderr equality; only
the empty `--json` stdout failed (`:61` is the `assertErrorOutput` call).

### Pass lines on the lane

```
--- PASS: TestGmQuestionUsageUnknownTask (0.03s)
    --- PASS: TestGmQuestionUsageUnknownTask/question_add (0.00s)
    --- PASS: TestGmQuestionUsageUnknownTask/question_ask (0.00s)
    --- PASS: TestGmQuestionUsageUnknownTask/question_list (0.00s)
    --- PASS: TestGmQuestionUsageUnknownTask/question_sync (0.00s)
    --- PASS: TestGmQuestionUsageUnknownTask/question_tick (0.00s)
    --- PASS: TestGmQuestionUsageUnknownTask/question_withdraw (0.02s)
    --- PASS: TestGmQuestionUsageUnknownTask/answer (0.00s)
    --- PASS: TestGmQuestionUsageUnknownTask/usage (0.00s)
```

### Packages tested (every package whose dependencies include internal/question or internal/cli)

From `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...`; `go test -count=1` exit 0:
`cmd/grid` (no test files), `internal/braincmd`, `internal/cli`, `internal/env`, `internal/guard`,
`internal/hook`, `internal/job` (+`frontmatter`, `jobcmd`, `lint`; `jobtest` has no test files),
`internal/jobgraph`, `internal/land`, `internal/lease`, `internal/migrate`, `internal/orchestrator`,
`internal/packaging` (+`cmd`), `internal/ports`, `internal/question`, `internal/signal`,
`internal/task`, `lib`. All `ok`.

## What changed

- `internal/question/command.go:25-27`: `Env` gains an optional `RecordError func(error)`.
- `internal/question/command.go:197-202`: the resolver failure calls it when non-nil, then prints and
  returns 1 as before. `grid answer` reaches this line through `RunAnswer`, so it is covered too.
- `internal/cli/question.go:23`: `questionEnv` sets `RecordError: env.recordError`.
- `internal/cli/usage.go:192`: `env.recordError(err)` before the print, as `internal/cli/cli.go:365-366` does.
- `internal/cli/gg_unknown_task_walk_test.go`: the eight entries leave `ggTaskKnownOpen`.
- `internal/cli/gm_question_usage_envelope_test.go`: new `TestGmQuestionUsageUnknownTask`.

`errorCode` maps `where.ErrTaskNotFound` to `no_such_task`; any other resolver error (for example
`no enclosing task; use --task <path>`) maps to no code, so it keeps its empty `--json` stdout.
The guest refusal for a hidden task is the same resolver error, so a guest now reads the same
envelope; `TestGuestProjectParity` PASSes unchanged.

## NOT VERIFIED on a live screen

Nothing here is UI. No live run of the built `grid` binary was made; every proof is the in-process
test harness.
