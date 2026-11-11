# C-hj-suite-grade: grade a `claude plugin eval` case offline

Head: see the bundle. Base check: `git ls-tree main -- internal/evalcmd/suitegrade.go` printed nothing (exit 0), so the lane was free to build.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. One subtest per grader type, passing and failing fixture each | HOLDS | `go test -count=1 -run '^TestHjgGraders$' -v ./internal/evalcmd/` exit 0. PASS lines: `regex_last_message`, `regex_pattern_in_the_body`, `regex_not_contains`, `regex_count:N_and_N+1`, `regex_file_target`, `regex_files_target`, `tool_used_input_match_with_&&`, `tool_used_max_0`, `tool_used_min_2`, `tool_order`, `file_exists_with_**`, `file_exists_exists_false`, `count:N_fails_on_N+1_and_N-1`, `tool_used_max_0_fails_when_the_tool_was_called`, `tool_used_default_min_is_1`, `last_message_is_the_last_text_of_the_run,_not_of_a_subagent`, `a_missing_file_never_passes_not_contains`, `an_llm_and_a_baseline_grader_are_not_graded`, `one_failing_grader_fails_the_case`, then `--- PASS: TestHjgGraders`. Each fixture row logs its pass reason and its fail reason, e.g. `pass: 1 matches, need exactly 1 \| fail: 2 matches, need exactly 1` (count:N fails on N+1) and `pass: Bash called 0 times, need at least 0 and at most 0 \| fail: Bash called 1 times, ...` (max 0). |
| 2. Refusals | HOLDS | `go test -count=1 -run '^TestHjgRefuses$' -v ./internal/evalcmd/` exit 0. 25 subtests PASS, among them `unknown_type`, `bad_regex`, `lookahead_is_not_RE2`, `no_graders_dir`, `empty_graders_dir`, `empty_transcript`, `blank_transcript`, `transcript_without_a_JSON_line`, `an_llm_grader_alone_is_not_graded_and_does_not_pass` (NotGraded = [judge], Passed false), then `--- PASS: TestHjgRefuses`. Every error row also checks the returned CaseGrade is zero. |
| 3. Nothing else broke | HOLDS | `go test -count=1 ./internal/evalcmd/ ./internal/eval/` ok, exit 0 (evalcmd 2.97s, eval 90.6s). |
| 4. Only the Touches files | HOLDS | `git diff --name-only main` lists internal/evalcmd/suitegrade.go, hjg_suitegrade_test.go and testdata/hjg/{bad.jsonl,good.jsonl,prompt.md,ws-bad/...,ws-good/...}; plus this report under docs/lanes/ as the runbook requires. |
| New tests fail on base | HOLDS | On main `suitegrade.go` does not exist (`git ls-tree` empty), so the test file cannot compile there: `undefined: GradeCase`. No base run was made. |
| No edited or deleted tests | HOLDS | Only new files; no existing test touched, internal/eval/runner_test.go and suite_test.go untouched. |
| No guard, gate, cap or hook on agents | HOLDS | The change is an offline scorer; nothing runs on an agent. |
| Importers tested | HOLDS | `go list` importers of internal/evalcmd (with transitive): cmd/grid, internal/cli, lib. `go test -count=1 ./internal/evalcmd/ ./internal/eval/ ./internal/cli/ ./cmd/grid/ ./lib/` exit 0 (cmd/grid has no test files). |
| Lint | HOLDS | `make lint` (go vet ./...) exit 0; `gofmt -w` run on both Go files. |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0. |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0. |
| No clock, sleep, timeout; temp dirs | HOLDS | Tests use t.TempDir and read testdata only; no expected-fail test. |
| *RealPlugin* tests | n/a | No row names one. |
| No claude, `claude plugin eval` or curl | HOLDS | Not run. |

## What changed

`internal/evalcmd/suitegrade.go`: `GradeCase(caseDir, transcript, files) (CaseGrade, error)`.

- Reads `prompt.md` (front matter must parse; `case.yaml` alone is accepted as a case) and each `graders/*.md` in name order. Front matter via `mdfile`, body is the pattern or rubric.
- Graded types: `regex` (target `last_message`, `trace`, `files`, `{source: file, path}`; match `contains` (default), `not_contains`, `count:N` = exactly N; flags i, m, s apply, g/u/y/d/v are ignored, anything else is an error), `tool_used` (exact tool name, `input_match` regex over the input JSON written with SetEscapeHTML(false), min default 1, max), `tool_order` (`before`/`after` as a tool name or `{tool, input_match}`; first before-call must precede the first after-call; either missing fails), `file_exists` (glob under `files`, `*` and `?` stay in a segment, `**` crosses; `exists` default true).
- `llm` and `baseline` go to `NotGraded` by name and never count. `Passed` is true when at least one grader ran and all that ran passed; a caller that needs the whole case checks `NotGraded` is empty.
- Errors, never passes: unknown grader type, missing type, unknown key for the type (a typo like `maxx`), bad regex or flag, empty pattern, pattern in both front matter and body, unknown target or match, min > max, a `files` need with no or missing folder, a path leaving the folder, no graders dir or no `*.md` in it, empty or JSON-less transcript.
- Missing evidence fails the grader (a missing file or no assistant text fails even `not_contains`).
- `last_message` is the last text item of the run's own assistant messages (subagent messages, which carry a parent_tool_use_id, are skipped). Tool calls come from `eval.DefaultTraceParser`.
- `trace` target is the raw transcript text.

Fixtures: `testdata/hjg/` holds a good and a bad stream-json transcript, a good and a bad workspace and a prompt.md. The grader files are strings in the test (`hjgGraders`), written to a temp case per run.

## Limits to know

- Patterns run on Go RE2. The built-in uses JS regex, so a lookahead, lookbehind or backreference (several old-plugin graders have them, e.g. `used-grid-home.md`) is a "bad regex" error here, not a silent pass. Such a case must be rewritten or checked by the live `claude plugin eval`.
- The grader format was taken from the brief and the old-plugin graders (`regex`, `tool_used`, `file_exists`, `llm` examples). No old grader uses `tool_order`, `count:N` or `baseline`; their syntax (before/after as a name or mapping, `count:N` as exactly N) is my reading of the brief and of `claude plugin eval --help` as quoted there.
- `min` defaults to 1 unless only `max` is set, then 0 (otherwise `max: 0` could never pass).

## NOT VERIFIED

- That the built-in `claude plugin eval` reads the same grader files the same way (tool_order and count:N syntax, glob semantics, `files` target meaning): needs the live command, which this lane may not run.
- No real recorded transcript from Claude Code 2.1.319 was graded; the fixtures are synthetic stream-json in the shape `parseStreamJSON` reads.
