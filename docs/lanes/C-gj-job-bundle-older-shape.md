# C-gj-job-bundle-older-shape

A Job bundle saved by an earlier plugin build still verifies and traces with the
current build. Base: `b8324c53a776fefe1faadb5b7094d3a4e6f7598a` (main merge, lane-Pint92).

Result: no row failed on base, so **no production file changed**. The readers
(`loadVerifyJob`, `traceScanTask`) already read the older shape. The three new
tests are regression guards: they pass on base and on the lane, and they fail
only if a later change breaks the older shape or recaptures its fixtures.

## Base check

I read verify.go:18-86, trace.go:236-296 and save.go:30-80 on the base (save.go
writes job.yaml as indented JSON of the Job, so a JSON job.yaml is the real
shape). No test on the base
verifies or traces a bundle whose job.yaml carries `max_rounds`:
`feedjob_save_test.go:142` and `feedjob_pick_test.go:39` only set or mention it in a
feed body; `testdata/job/task/plan.md:12` carries it in a plan (the plan-fallback
path of TestVerifyCLI, not a saved bundle); `trace_test.go:251` reads
`testdata/trace-old-inputs.json`, a hand-written inputs.json with no job.yaml
`max_rounds`. So the lane proceeds.

## Proof table

| # | Row | Result | Proof |
|---|-----|--------|-------|
| 1 | TestGjOlderBundleVerifyReadsRetiredFields | HOLDS | `go test -count=1 -v -run TestGjOlderBundle ./internal/job/jobcmd/` exit 0, `--- PASS: TestGjOlderBundleVerifyReadsRetiredFields`. Test at gj_older_bundle_test.go:102: job.yaml is the `body` of the 73025e9e^ fixture (still holds `max_rounds` and `review`, checked at :87), verify replaced by `{"kind":"reviewer-check"}` (:82); `Run(["verify","J-1","--task",task])` exits 3 with "reviewer must check" in stdout (:106), stderr empty (:109); `loadVerifyJob` gives Review "L1", Title "Build example feature" (:112-118). |
| 2 | TestGjOlderBundleTraceReadsOldInputs | HOLDS | Same run, `--- PASS: TestGjOlderBundleTraceReadsOldInputs`. gj_older_bundle_test.go:121: `Run(["trace","brain:project/export-format"])` exits 0, no "invalid inputs.json" and no "problem:" in the output (:125-131); the `--json` run (:132) has one row, J-1, the pointer, sha256 2d82e2c5…e27b and tier person-team, and no problems. |
| 3 | TestGjOlderBundleFixturesKeepTheOldShape | HOLDS | Same run, `--- PASS`. :151: body has `max_rounds` and `review`; inputs.json (top level and each source) has no `shown`, `brief_bytes` or `pasted_chars`; tiers include person-team and external; README.md names `73025e9e^` and `919c2b9a^`. |
| 4 | Home rule | HOLDS | gj_older_bundle_test.go:26-48: `t.Setenv` of HOME, CLAUDE_CONFIG_DIR, CODEX_HOME, GRID_HOME under one `t.TempDir()` (:40) before anything runs; `t.Fatalf` when HOME is not under that root (:44); Env gets `Home` and `GridHome` from the same root (the Env struct has no field for the other two keys). `grep -n "Sleep\|time.After"` on the file: no match, exit 1. |
| 5a | `go test -count=1 -run 'TestGjOlderBundle\|TestVerifyCLI\|TestTrace\|TestOWTrace' ./internal/job/...` | HOLDS | exit 0: `ok .../internal/job 0.329s [no tests to run]`, `ok .../job/frontmatter 0.174s [no tests to run]`, `ok .../job/jobcmd 0.621s`, `? .../job/jobtest [no test files]`, `ok .../job/lint 0.174s [no tests to run]` |
| 5b | `make lint` | HOLDS | exit 0 (`go vet ./...`, no findings); `gofmt -l internal/job/jobcmd` prints nothing |
| 5c | `git diff main --stat` names only Touches | HOLDS | Before this report: gj_older_bundle_test.go, testdata/older-bundle/{README.md,inputs.json,job-record.json}, 334 insertions, 0 deletions. This report file (docs/lanes) is the one addition the runbook asks for. |

## Rules

| Rule | Result | Proof |
|------|--------|-------|
| Small commits, one-sentence messages, no attribution lines | HOLDS | 3 commits on the lane (testdata, test, this report); messages are one sentence each, no trailer lines |
| Public repo: no personal paths, hosts or emails | HOLDS | the new files hold only commit ids, fixture ids and `t.TempDir()` paths |
| New tests fail on base and pass on the lane | NOT APPLICABLE, stated plainly | No row failed on base. The readers already tolerate the older shape, so the tests pass on base too. My first run of row 1 failed on my own setup (the saved Job names worktree `example-worktree`; verify looks the folder up before it reads the kind): `exit 1 ... reason: stat .../projects/alpha/worktrees/example-worktree: no such file or directory`. I fixed that in the test helper (gj_older_bundle_test.go:90-94), not in a reader. |
| No existing test edited or deleted | HOLDS | `git diff main --stat` shows only added files |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | no production file touched |
| Tests of changed packages and their importers | HOLDS | `go list -f '{{.ImportPath}}: {{join .Imports " "}}' ./... \| grep internal/job/jobcmd` shows jobcmd is imported by `internal/cli` only. `go test -count=1 ./internal/job/jobcmd/ ./internal/cli/` exit 0: `ok jobcmd 3.730s`, `ok cli 65.316s` |
| Windows and macOS builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| *RealPlugin* tests | NOT APPLICABLE | no row names one; the lane needs no built plugin |
| UI rows | NOT APPLICABLE | no UI |

## What changed

- `internal/job/jobcmd/testdata/older-bundle/job-record.json`: byte copy of `contract/fixtures/job/valid/basic.json` at `73025e9e^` (git show, redirected, not hand-written).
- `internal/job/jobcmd/testdata/older-bundle/inputs.json`: byte copy of `testdata/golden/job-inputs.golden` at `919c2b9a^`.
- `internal/job/jobcmd/testdata/older-bundle/README.md`: names the two commits.
- `internal/job/jobcmd/gj_older_bundle_test.go`: three tests, helpers `gjOldEnv`, `gjOldRead`, `gjOldBody`, `gjOldBundle`.

The test builds job.yaml from the fixture's `body` as raw JSON members, so
`max_rounds`, `review` and every other older member stay as the older build wrote
them; only `verify` is replaced (reviewer-check, so no shell runs). The fixture is
kept as the whole 73025e9e^ record, not a pre-cut job.yaml, so it stays byte
identical to git.

## NOT VERIFIED on a live screen

Nothing here needs a screen. Not run: the whole-module test and the cross-repo
set (the orchestrator runs them at landing). Not covered: a bundle saved by a
build older than 73025e9e that lacks other fields, and `brief.md` content (the
readers do not parse it; the test writes a one-line brief).
