# C-jw-title-empty-front-matter

Words that open with an empty front matter block (`---` then `---` on the next line) are now treated as front matter by `titleOf`: with nothing after, `grid task new` refuses them with the existing message; with a body, the title comes from the body.

## Proof table

| Row / rule | Result |
|---|---|
| 1. `grid task new` with `---\n---\n` exits non-zero, refusal text, no folder | HOLDS. `go test -count=1 -run 'TestJwTf' ./internal/cli/` exit 0 on the lane (`TestJwTfEmptyBlockIsRefused`, 3 inputs: `---\n---\n`, `---\n---` with no final newline, and blank lines around with a blank body). Exit is 2 through the existing refusal path. Red on base (title.go checked out to base): `task new "---\n---\n": exit 0, want non-zero; stdout "{\"id\":\"T-01\",\"path\":\".../personal/tasks/T-01-task\"}"` and `task folders written: [.../personal/tasks/T-01-task]`. |
| 2. `---\r\n---\r\nFix the login\n` makes a task titled "Fix the login" | NOT DONE for the CR form through `grid task new`. `internal/task/task.go:71` calls `checkTaskText("words", ..., true)`, and `internal/task/text.go:18` treats `\r` as a control character. Any words with `\r` are refused before `titleOf` runs: `grid task new: invalid task edit: words contains invalid UTF-8 or a forbidden control...` (seen on base and on the lane; exit 2). Changing that check is outside Touches. `emptyBlockBody` in title.go does accept `\r\n`, but no test can reach it. The LF form HOLDS: `TestJwTfEmptyBlockTitlesFromBody` makes a task titled "Fix the login" from `---\n---\nFix the login\n`, with extra body lines, with blank lines between, and with blank lines before the block. Red on base: `row2-lf: title = "---", want "Fix the login"`. |
| 3a. `go test -count=1 -run 'TestJwTf\|TestIj\|Title' ./internal/cli/ ./internal/task/` | HOLDS, exit 0: `ok internal/cli 0.165s`, `ok internal/task 0.396s` |
| 3b. `make lint` | HOLDS, exit 0 (`go vet ./...`) |
| 3c. `make cross` | HOLDS, exit 0 (darwin/arm64 and windows/amd64: vet and test compile) |
| 3d. `git diff main --stat` names only Touches | HOLDS: `internal/task/title.go`, `internal/cli/jw_title_front_matter_test.go`, plus this report `docs/lanes/C-jw-title-empty-front-matter.md` |
| Tests of changed packages and their direct importers | HOLDS, exit 0. Packages: task, braincmd, guard, hook, job, job/jobcmd, job/jobtest, land, signal, updates, cli (the direct importers of `internal/task`, from `go list`; jobtest and updates have no test files). `go test -count=1` on these 11 packages: all `ok`. |
| gofmt | HOLDS: `gofmt -l internal/task internal/cli` prints nothing |
| New tests fail on base, pass on lane | HOLDS, see rows 1 and 2 |
| No existing test edited or deleted | HOLDS |
| No guard, gate, cap or hold on agents | HOLDS, none added |
| `git merge main` before last proof run | HOLDS: "Already up to date." |
| No `*RealPlugin*` test needed | Not applicable |

## What changed

- `internal/task/title.go`: `titleOf` first checks `emptyBlockBody` (new). It matches `---`, a line end, `---`, then a line end or the end of the words, and returns what follows. A blank body gives the existing refusal; otherwise the body gives the title. All other words go through the shared parser as before, and the shared parser is untouched.
- `internal/cli/jw_title_front_matter_test.go`: `TestJwTfEmptyBlockIsRefused`, `TestJwTfEmptyBlockTitlesFromBody`, and the helper `jwTfRefused`, set up like `ijTitleNew` and `ijTitleMade`.

## Open item for the owner

Row 2 names `\r\n` words. The text check at `internal/task/text.go:18` refuses a `\r` in words, so CRLF words never reach `titleOf`. Say whether the check should allow `\r` in words (a separate change, not in this brief).

## NOT VERIFIED on a live screen

Nothing here needs a screen. The New task page was not run.
