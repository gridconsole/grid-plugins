# C-hs-person-control-answer

Base sha: 91fadeff87525f5859978740e0b66ce51f375e48. On the base the leased control had no "answer" key (person.go:208 body held target, status, reason), so the lane applies.

## Proof table

| Row | Result |
|---|---|
| 1. TestHsCaLeasedControlHasAnswer | HOLDS. `go test -count=1 -v -run TestHsCa ./internal/orchestrator/` exit 0, PASS. On base it failed: `hs_control_answer_test.go:34: control answer = <nil>, want "Call it Lark."` |
| 2. TestHsCaBothPathsSameKeys | HOLDS. PASS on the lane. On base it failed: `hs_control_answer_test.go:67: leased keys [reason status target], unleased keys [answer reason status target]` |
| 3. TestHsCaFieldsDoNotOverwrite | HOLDS. PASS on the lane (control answer is the argument, note is "n"). On base it failed: `hs_control_answer_test.go:92: control answer = <nil>, want the answer argument` (the guarantee against overwrite holds because answerFields does not list "answer", person.go:160) |
| 4a. `git diff main --stat` | HOLDS. Only internal/orchestrator/hs_control_answer_test.go (97 +) and internal/orchestrator/person.go (1 line changed, :208) |
| 4b. tests of changed and importing packages | HOLDS, exit 0. Importers by `go list -deps`: cmd/grid, internal/cli, internal/migrate, lib. `go test -count=1 ./internal/orchestrator/ ./internal/contract/ ./internal/cli/ ./internal/migrate/ ./cmd/grid/ ./lib/ ./contract/...`: all ok (cmd/grid and contract have no test files) |
| 4c. `make lint` | HOLDS, exit 0 (`go vet ./...`); `gofmt -l internal/orchestrator` prints nothing |
| Windows build | HOLDS. `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS. `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| New tests fail on base, pass on lane | HOLDS, lines pasted above |
| No existing test edited or deleted | HOLDS. The diff only adds a file and changes one line in person.go |
| No guard, gate, cap, hold or blocking hook | HOLDS. None added |
| RealPlugin tests | Not applicable: no row names one |
| Commits | One commit, one sentence, no attribution lines |

## What changed

- person.go:208: the body of the accepting control written by a task-lease writer now has `"answer": answer` (the trimmed text), before the answerFields loop. Both paths now write target, status, reason and answer.
- New file internal/orchestrator/hs_control_answer_test.go with the three tests.

## NOT VERIFIED on a live screen

The app's task page (taskview) showing the answer for a leased answer. That needs the app and a live screen. C-hs-answered-text reads body.answer from this control.

DONE
