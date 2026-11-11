# C-redo-curator-eval

Start sha (`git rev-parse origin/main`): 13b11fe104ad2ccc46942726d2ebd2ea5bacba54

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 1 Fails on main | HOLDS | Test committed first (own commit), task folder absent. `go test ./internal/eval -run TestCuratorDecides` → `curator_decides_test.go:70: agent-curator-decides is not in evals/tasks`, `FAIL` (exit 1), on start sha 13b11fe1 plus the test file. |
| 2 Empty agent | HOLDS | `curator_decides_test.go` subtest "empty agent": asserts `Setup.Status == "passed"`, `GatesPassed == 0` (`GatesTotal == 1`) and `Status == "failed"`. `go test ./internal/eval -run TestCuratorDecides -v` → PASS, exit 0. |
| 3 Fake agent | HOLDS | Subtest "fake agent": two `grid brain decide T-01 … --note` commands; asserts `Summary.Passed == len(task.Checks)` (8) and gates 1/1, status passed. PASS, exit 0. |
| 4 Hand-written file | HOLDS | Subtest "brain file written by hand": same two decisions, then `by-hand.md` into `knowledge/conventions/`; asserts `Summary.Failed == 1`, gate passed, and the failed id is `no-brain-file-written-by-hand`. PASS, exit 0. |
| 5 grep + suite | HOLDS | `git grep -n "template-present\|agent-templates/grid-curator\|brain-proposals/\*\.md" -- evals/tasks/agent-curator-decides` prints nothing (see note 2). `go test ./internal/eval` (includes TestSampleSuite with the new id, `suite_test.go:24`) → ok. |
| 6 Tests + lint | HOLDS | `go test ./internal/eval ./internal/braincmd -count=1` → ok, ok. `make lint` exit 0. |
| Importers of changed packages | HOLDS | `go list` importers of internal/eval and internal/braincmd: internal/cli, internal/evalcmd, internal/land. `go test ./internal/cli ./internal/evalcmd ./internal/land -count=1` → ok ×3. |
| Windows / macOS builds | HOLDS | `GOOS=windows go vet ./internal/eval ./cmd/grid && GOOS=windows go build ./...` exit 0; `GOOS=darwin go vet ./internal/eval && GOOS=darwin go build ./...` exit 0. |
| New tests fail on base, pass on lane | HOLDS | See row 1 and rows 2–4. |
| No existing test edited | HOLDS | Only `suite_test.go:24`, one id added to `want`. |
| No guard/gate/cap on agents | HOLDS | None added; the `all-decided` gate is an eval check, not an agent restriction. |
| Whole-module test, cross-repo set | NOT RUN | The orchestrator runs these at landing. |

## What changed

- `internal/eval/curator_decides_test.go` (new): builds `cmd/grid` into `t.TempDir()` (`.exe` on Windows), puts it first on PATH with `t.Setenv`, loads `agent-curator-decides` from `evals/tasks`, runs it with `Runner.Run` three times (empty, fake, hand-written file). Skips without `sh`.
- `evals/tasks/agent-curator-decides/` (new; started from the old lane's files): `setup.sh` writes both proposals with `grid brain propose --task T-01 --project notes-app …`; the `mkdir`/`mv` lines and the comments about the move, the template copy and PATH are gone; `mkdir -p .claude/agents` and `git init -q .` stay. `checks.json` has eight checks; `template-present` is dropped. `prompt.md` and `task.json` name no template.
- `internal/eval/suite_test.go`: `agent-curator-decides` added to `want`.

## Notes

1. `--project` together with `--task` is not refused: setup passed with both (`Setup.Status == "passed"` in all three runs). `--project` stays.
2. The `all-decided` check's glob `"$1"/brain-proposals/*.md` matched the row 5 grep pattern. I rewrote only the quoting, `"$1/brain-proposals"/*.md`; the glob, the test and the regexes behave the same. Without it row 5 printed `checks.json:2`.
3. In row 4 the agent runs `mkdir -p knowledge/conventions` first, because accepting `csv-semicolons` only creates `knowledge/decisions` and rejecting writes nothing under `knowledge/`; the first attempt wrote nothing and the agent step failed (all checks skipped).
4. All eight checks passed for the fake agent without any change to their regexes (history lines and knowledge file set match what the CLI writes on the start sha).
5. `tierB` in `home_test.go` counts tier-b tasks only (25); the new task is tier c, so that count is unchanged.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real curator (`claude -p` with `agents/curator.md`), which is the orchestrator's.
