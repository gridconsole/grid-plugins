# C-hj-read-before-edit

A run-job eval case: the worker reads a file in this session before it edits it (skills/run-job/SKILL.md:12).

## Proof table

| Row | Result |
|---|---|
| 1. TestHjoBad: `read-first` fails on bad.jsonl, `edited` passes, bad.jsonl has tool calls | HOLDS: `go test -count=1 -run '^TestHjo' -v ./internal/evalcmd/` exit 0; `read-first: graded=true pass=false (before at call 1, after at call 0)`, `edited: pass=true (Edit called 2 times)`, `--- PASS: TestHjoBad` |
| 2. TestHjoGood: every code grader passes on good.jsonl; llm graders log "not graded" and are never counted | HOLDS: same run, exit 0; `read-first pass=true (before at call 0, after at call 1)`, `edited pass=true`, `cites pass=true`, `--- PASS: TestHjoGood`. The llm branch is `hjo_case_test.go` hjoGrade (`not graded`); the case holds no llm grader |
| 3. TestHjoShape: name, `tags: [live]`, runs 1, schema_version "1.1", known grader types | HOLDS: same run, exit 0, `--- PASS: TestHjoShape` |
| 4. git grep for emails, names, keys and home paths prints nothing; `go test -count=1 ./internal/evalcmd/` exit 0 | HOLDS: `git grep --cached -nE '...' -- evals/suite/read-before-edit internal/evalcmd/testdata/hjo` printed nothing; `go test -count=1 ./internal/evalcmd/ ./internal/cli/` exit 0 |
| 5. `git diff --name-only main` lists only Touches files | HOLDS: see the list below, plus this report in docs/lanes/ as the runbook requires |
| Tests of changed package and importers | HOLDS: `go list` shows internal/cli imports internal/evalcmd; both packages `ok`, exit 0. No non-test code changed |
| Lint | HOLDS: `make lint` exit 0 (go vet ./..., gofmt -l empty) |
| Windows and macOS builds | HOLDS: `GOOS=windows CGO_ENABLED=0 go vet ./internal/evalcmd/` exit 0; `GOOS=darwin CGO_ENABLED=0 go vet ./internal/evalcmd/` exit 0 (vet compiles the tests) |
| New tests fail on base | Not run on base. The new tests need evals/suite/read-before-edit and testdata/hjo, which `git ls-tree main` lacks, so on main they would stop at `no graders found`; I did not run that |
| No guard, gate, cap or hold on agents | HOLDS: only an eval case and a test were added |
| No existing test edited or deleted | HOLDS: only new files |
| No clock, sleep or timeout in the test; temp dirs only | HOLDS: TestHjoFixtureScript uses t.TempDir; no time import |
| No `*RealPlugin*` test needed | n/a |

## What changed

- `evals/suite/read-before-edit/`: prompt.md, case.yaml, fixture.sh (builds `job.md` and `src/retry.go` in the run dir), graders `read-first.md` (tool_order Read before Edit), `edited.md` (tool_used Edit, min 1), `cites.md` (regex over last_message).
- `internal/evalcmd/testdata/hjo/`: bad.jsonl (Edit first, the file-not-read error, then Read and Edit; 8 lines) and good.jsonl (Read, Edit, cites `src/retry.go:9`; 7 lines).
- `internal/evalcmd/hjo_case_test.go`: TestHjoBad, TestHjoGood, TestHjoShape, and TestHjoFixtureScript, which runs fixture.sh in a temp dir and checks line 9 of the file is the line good.jsonl cites. It skips on Windows (no POSIX sh).
- The grader front matter keys (`type`, `before`, `after`, `tool`, `min`, `target`, `match`) follow the brief; the test's own parser reads them, not Claude Code's.

## NOT VERIFIED

- The live run: `claude plugin eval ... --case read-before-edit` (the orchestrator runs it by hand). Whether Claude Code 2.1.319 accepts these front matter key names is unchecked.
- Whether a live sonnet worker reads before it edits.
