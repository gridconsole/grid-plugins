# C-cb-brain-author-everywhere: a proposal made in a Job names that Job, and revert and supersede name the person, so no curated note's author is a random id

Base: f09dda3a (plugin main). Code head: 3020bc57 (two commits: the production change 85095127, then the tests 3020bc57). All proof below ran on 3020bc57. The report commit after it is doc-only.

Base check: on f09dda3a `git grep -n 'brain.SessionOrigin()' -- internal/braincmd/command.go internal/braincmd/changes.go internal/braincmd/supersede.go` prints the three hits the brief names (command.go:426, changes.go:266, supersede.go:67). The wider `git grep -n 'brain.SessionOrigin()' -- internal/` also lists hy_drop.go:56 (kept). All three are replaced:

- command.go propose: `origin, err = cbAuProposeOrigin(env)` (the `--external` rewrite to kind `import` follows it, as before).
- changes.go revert and supersede.go supersede: `origin, err := hyDrOrigin(env)`.

## What changed

- `internal/braincmd/cb_author_origin.go` (new): `cbAuProposeOrigin(env Env) (brain.Origin, error)`. Order: (1) `Env.Job` is a `brain.ValidName` and `Env.Task` is non-empty and resolves through `env.ResolveTask` to a task whose ID is a `brain.ValidName` gives `Origin{Kind: "job", ID: "<task id>/<job>"}`; (2) `Env.Session` is a `brain.ValidName` gives `Origin{Kind: "session", ID: <session>}`; (3) `brain.SessionOrigin()`. A value that is not a valid name is never written; the next source is used. A nil `ResolveTask` or a resolve error falls through.
- `internal/braincmd/command.go`: `Env` gains `Task, Job, Session string` (doc comment says braincmd reads no environment variable); propose takes its origin from the helper.
- `internal/braincmd/changes.go`, `supersede.go`: one line each, `brain.SessionOrigin()` becomes `hyDrOrigin(env)`. `hyDrOrigin` is untouched, so its order (person, else session) and its users (drop, restore, write, rename) are unchanged; `grid brain write` still gets no `job:` origin.
- `internal/cli/brain.go`: fills `Task` from GRID_TASK, `Job` from GRID_JOB and `Session` from GRID_SESSION_ID, else CLAUDE_SESSION_ID (new `brainSession`, the order `internal/cli/task.go:509-512` reads), all through `env.getenv`.
- `internal/braincmd/cb_author_origin_test.go` (new): the four tests below.
- No new front-matter kind; a person appears in audit lines only. `internal/brain/store.go` and `config.go` are untouched.

## Proof table

| # | Row / rule | Result | Command / place |
|---|---|---|---|
| 1 | `TestCbAuProposeInJob`: propose with `Env{Task: "T-01", Job: "J-3"}` on the decideFixture shape; front matter parses with `Origin{job, T-01/J-3}`; after `decide T-01 fromjob accept --json` the project audit has the accepted row with origin kind `job`, id `T-01/J-3` | HOLDS | `go test -count=1 -v -run 'TestCbAu' ./internal/braincmd/` exit 0: `--- PASS: TestCbAuProposeInJob` (cb_author_origin_test.go:56). Red on base: see below. |
| 2 | `TestCbAuRevertAndSupersedePerson`: with `Env{Member: "bo", Role: "owner"}` a revert and a supersede each add audit rows (revert: 1 `reverted`; supersede: `superseded` and `linked`) whose origin is `person:bo`; with Member empty, or not a valid name (`no good!`), every new row is `session:<24 hex>` | HOLDS | same run: `--- PASS: TestCbAuRevertAndSupersedePerson` with 6 sub-tests PASS (revert/bo, supersede/bo, revert/, supersede/, revert/no_good!, supersede/no_good!), cb_author_origin_test.go:100. Red on base for `bo`: see below. |
| 3 | `TestCbAuFallback`: table of 17 Env values (Job empty; Job `zz/9`; Job `zz-9\n`; Task `T-99` that does not resolve; resolver error; resolved task id `zz/1` and empty; Session `zz 9` and `zz9\n`; nothing valid; valid job beside a bad session; `--external`): each gives the next source, none valid gives kind `session` with a 24-hex id. The proposal printed, the proposal on disk and the audit line agree. A byte search of every file under the Grid home finds no invalid value | HOLDS, with substituted values | same run: `--- PASS: TestCbAuFallback` with 17 sub-tests PASS (cb_author_origin_test.go:160). The brief names Job `J/3`, Job `J-3\n` and Session `s 1`; the table uses `zz/9`, `zz-9\n` and `zz 9` instead (same classes: a slash, a newline, a space) so the byte search for the invalid value cannot match the valid `J-3` that the tests also use. Every class the brief lists is covered. The `--external` row keeps kind `import` with id `T-01/J-3` (an import proposal is quarantined, so that row stops before accept). `TestAllCommandsPlainAndJSON` (command_test.go:49) is unchanged and passes. |
| 4 | `TestCbAuSetWalk`: go/parser over the non-test .go files of internal/braincmd fails when `brain.SessionOrigin` appears in any function but `hyDrOrigin` and `cbAuProposeOrigin`; also fails when either stops calling it (no vacuous pass) and on any `os.Getenv`, `os.LookupEnv`, `os.Environ` | HOLDS | same run: `--- PASS: TestCbAuSetWalk` (cb_author_origin_test.go:260). `git grep -n 'brain.SessionOrigin()' -- internal/braincmd/ ':!*_test.go'` on the lane lists `cb_author_origin.go:12` (a comment), `cb_author_origin.go:27` and `hy_drop.go:56` only: no other code site. |
| 5a | `go test -count=1 ./internal/braincmd/ ./internal/brain/` 0 fail; command_test.go:49, ke_write_test.go:261, lo_write_anyone_test.go:106, hy_drop_test.go:246 green unchanged | HOLDS | exit 0: `ok internal/braincmd 4.676s`, `ok internal/brain 4.996s`. `go test -v ./internal/braincmd/` has 97 `--- PASS` top-level and 0 FAIL/SKIP; `go test -v ./internal/brain/` has 0 FAIL and one SKIP, `TestLockHelperProcess`, a subprocess helper that skips when not spawned, not touched by this lane. No existing test file is edited. |
| 5b | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS | exit 0: `--- PASS: TestHsIpWalksEveryCommand (0.00s)`, `--- PASS: TestIlGuestSiteList (0.02s)`, `ok internal/cli 0.049s`. |
| 5c | `go test -count=1 ./internal/job/lint/` | HOLDS | exit 0: `ok internal/job/lint 0.100s`. |
| 6a | `make lint` | HOLDS | exit 0 (`go vet ./...`, no output). `gofmt -l internal/braincmd internal/cli` prints nothing. |
| 6b | `make cross` (darwin/arm64 and windows/amd64: vet and test compile) | HOLDS | exit 0; both targets print `cross: <os>/<arch> vet` and `test compile`, every package `ok`. |
| 6c | `git diff main --stat` names only the files in Touches | HOLDS | `cb_author_origin.go`, `cb_author_origin_test.go`, `changes.go`, `command.go`, `supersede.go` in internal/braincmd and `internal/cli/brain.go`; `hy_drop.go` is not touched. Plus this report, `docs/lanes/C-cb-brain-author-everywhere.md`, which the runbook's Definition of done requires and which is committed after the code head. |
| R1 | Importers of the changed packages (`go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered on braincmd and cli): cmd/grid, contract/formats, internal/account, internal/braincmd, internal/cli, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib | HOLDS | `go test -count=1` on all of them, exit 0: braincmd ok, brain ok, cli ok 54.526s, lib ok 16.528s, contract/formats ok, account ok, golden ok, jobgraph ok, mcp ok, packaging ok 6.123s, question ok, settings ok; cmd/grid has no test files. |
| R2 | Walk tests, plugin set | HOLDS | 5b and 5c above. The app walk tests (webui, TestFvArgvCollects, TestEveryWiredMount, slices Imports) are for the app repo; this lane is the plugin. |
| R3 | New tests fail on base, pass on the lane | HOLDS | pasted below. |
| R4 | No existing test edited or deleted | HOLDS | `git diff main --stat` shows no test file but the new one, and no deleted line in a test. |
| R5 | No guard, gate, cap or hold on agents | HOLDS | The change only chooses which name an author is stamped with; it refuses nothing and every fallback still writes. |
| R6 | `*RealPlugin*` tests, mismatch proofs, UI rows | N/A | The brief names none. Nothing was skipped and passed off as proof. |
| R7 | No personal path, host or email in the files | HOLDS | The tests use `t.TempDir()` and made-up ids (`bo`, `sess-1`, `T-01`, `J-3`). |
| R8 | CLI fill of Task, Job and Session from the environment | HOLDS, throwaway test (not committed) | Touches allows no test file in internal/cli, so I ran a temporary `internal/cli/zz_throwaway_test.go` (feedHarness plus a full task.md and intent.md, `Env.Getenv` stub) and removed it with `git clean -f` afterwards (`git status --short` is clean). Run: `--- PASS: TestZzThrowawayBrainEnv` with `job_by_path` (GRID_TASK is the task folder, GRID_JOB `J-7`, GRID_SESSION_ID `s-a`) giving `job:T-07/J-7`; `job_by_id` (GRID_TASK is the task's id) giving `job:T-07/J-7`; `grid_session_first` (GRID_SESSION_ID `s-a`, CLAUDE_SESSION_ID `s-b`) giving `session:s-a`; `claude_session` giving `session:s-b`; `job_without_task` (GRID_JOB without GRID_TASK, CLAUDE_SESSION_ID `s-b`) giving `session:s-b`. |

### Red on base (R3)

The three call sites put back to `brain.SessionOrigin()` (the Env fields kept so it compiles), then `go test -count=1 -run 'TestCbAu' ./internal/braincmd/`, with the lane's call sites restored afterwards by `git checkout -- <files>`:

```
--- FAIL: TestCbAuProposeInJob (0.00s)
    cb_author_origin_test.go:75: proposal origin: {Kind:session ID:93f03e59543b4afe2d1977d7}, <nil>
--- FAIL: TestCbAuRevertAndSupersedePerson (0.07s)
    --- FAIL: TestCbAuRevertAndSupersedePerson/revert/bo (0.01s)
        cb_author_origin_test.go:135: member "bo": reverted row checks has origin session:19c5c7ea3bca568fac137775, want person:bo
    --- FAIL: TestCbAuRevertAndSupersedePerson/supersede/bo (0.01s)
        cb_author_origin_test.go:153: member "bo": superseded row deploy has origin session:a88e7a247f85aa2863b97db2, want person:bo
        cb_author_origin_test.go:153: member "bo": linked row deploy-v2 has origin session:a88e7a247f85aa2863b97db2, want person:bo
--- FAIL: TestCbAuFallback (0.03s)
    --- FAIL: TestCbAuFallback/job_with_a_slash (0.00s)
        cb_author_origin_test.go:221: origin: printed {Kind:session ID:5efafb129140348c07074796}, stored {Kind:session ID:5efafb129140348c07074796}, want {Kind:session ID:sess-1}
    --- FAIL: TestCbAuFallback/valid_job_wins_over_a_session (0.00s)
        cb_author_origin_test.go:221: origin: printed {Kind:session ID:6a902a14b2373010e5023704}, stored {Kind:session ID:6a902a14b2373010e5023704}, want {Kind:job ID:T-01/J-3}
    --- FAIL: TestCbAuFallback/external_keeps_the_import_kind (0.00s)
        cb_author_origin_test.go:221: origin: printed {Kind:import ID:b93b8754a1dc0255106db828}, stored {Kind:import ID:b93b8754a1dc0255106db828}, want {Kind:import ID:T-01/J-3}
        (and 6 more Fallback sub-tests in the same shape: job_empty,_session; job_with_a_newline; task_does_not_resolve,_session; resolver_fails; resolved_task_id_is_not_a_name; valid_job_wins_over_a_bad_session)
--- FAIL: TestCbAuSetWalk (0.01s)
    cb_author_origin_test.go:291: changes.go:266:17: brain.SessionOrigin used in "runRevert"; use hyDrOrigin or cbAuProposeOrigin
    cb_author_origin_test.go:291: command.go:431:17: brain.SessionOrigin used in "Run"; use hyDrOrigin or cbAuProposeOrigin
    cb_author_origin_test.go:291: supersede.go:67:17: brain.SessionOrigin used in "runSupersede"; use hyDrOrigin or cbAuProposeOrigin
FAIL
```

The `revert/` and `supersede/` rows with an empty member pass on base on purpose: those say "as today".

## Notes for the backlog

- The CLI fill (R8) has no committed test, because the brief's Touches names no internal/cli test file. A short `TestCbAu...` in internal/cli would pin the GRID_TASK, GRID_JOB, GRID_SESSION_ID and CLAUDE_SESSION_ID order.
- Through the CLI, `Env.Task` resolves to the task's display id (`T-07`), whether GRID_TASK is the folder or the stable id, so a Job's author is `job:T-07/J-7`, the shape the brief names. That id is unique within a project, not across projects.
- `skills/brain-keeper` and `agents/curator.md` (owned by C-cc) may still say a proposal's author is a random session id; not touched here.

- `runBrain` (internal/cli/brain.go:9) is the only place that builds `braincmd.Env`: `git grep -n 'braincmd\.' -- internal/cli lib cmd ':!*_test.go'` lists that one line.
- The cli harness falls through to `os.Getenv` when `Env.Getenv` is nil, so a cli test that runs `grid brain propose` now reads the runner's GRID_TASK, GRID_JOB and session variables. I found no cli, lib, golden or cmd test that asserts a propose-made origin id (`git grep -n -E '\{24\}|origin_id|Origin\.ID|OriginID|session:[0-9a-f]'` over them prints nothing; the `Origin{Kind: "session", ...}` hits in internal/cli tests are fixtures built by hand, not the result of propose), so none depends on the runner's environment.

## NOT VERIFIED on a live screen

Nothing here has a screen. Not run: a hosted app session that starts a real Job with GRID_TASK and GRID_JOB set and runs `grid brain propose`; the app's own rigs were not run (the brief says none of them asserts an origin).

Verdict: DONE
