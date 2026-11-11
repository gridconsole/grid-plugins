# C-ij-task-title-front-matter: `grid task new` takes the title from the request's front matter

Lane: C-ij-task-title-front-matter, repo plugin. Base sha: ee8a96c1 (main = origin/main). Base check holds: on ee8a96c1, `internal/task/create.go:93` was `title := strings.TrimSpace(strings.SplitN(strings.TrimSpace(o.Words), "\n", 2)[0])`, the first line of the words.
Code head: 7e656046. The commit that adds this report sits on top of it.

## Base failures (new test on ee8a96c1, before the fix)

`go test -count=1 -run TestIjTitle ./internal/cli` exit 1:

```
--- FAIL: TestIjTitleFromFrontMatter (0.04s)
    --- FAIL: TestIjTitleFromFrontMatter/row1-front-matter-title (0.01s)
        ij_task_title_test.go:91: row1-front-matter-title: title = "---", want "Fix: the login page"
        ij_task_title_test.go:94: row1-front-matter-title: folder = "T-01-task", want "T-01-fix-the-login-page"
    --- FAIL: TestIjTitleFromFrontMatter/row1-no-title-uses-body (0.01s)
        ij_task_title_test.go:91: row1-no-title-uses-body: title = "---", want "Users cannot sign in"
        ij_task_title_test.go:94: row1-no-title-uses-body: folder = "T-01-task", want "T-01-users-cannot-sign-in"
    --- FAIL: TestIjTitleFromFrontMatter/row2-blank-lines-before-block (0.00s)
        ij_task_title_test.go:91: row2-blank-lines-before-block: title = "---", want "Real title"
        ij_task_title_test.go:94: row2-blank-lines-before-block: folder = "T-01-task", want "T-01-real-title"
    --- FAIL: TestIjTitleFromFrontMatter/row3-title-and-blank-body (0.02s)
        ij_task_title_test.go:91: row3-title-and-blank-body: title = "---", want "Fix login"
        ij_task_title_test.go:94: row3-title-and-blank-body: folder = "T-01-task", want "T-01-fix-login"
    --- FAIL: TestIjTitleFromFrontMatter/row4-long-body-line (0.00s)
        ij_task_title_test.go:91: row4-long-body-line: title = "---", want "aaaa…(120 a)"
    --- FAIL: TestIjTitleFromFrontMatter/row4-long-title (0.00s)
        ij_task_title_test.go:91: row4-long-title: title = "---", want "bbbb…(120 b)"
--- FAIL: TestIjTitleOnlyFrontMatterIsRefused (0.00s)
    ij_task_title_test.go:104: row3-only-front-matter: exit = 0, want 2; stdout "{\"id\":\"T-01\",\"path\":\"…/home/.grid/personal/tasks/T-01-task\"}\n"
    ij_task_title_test.go:107: row3-only-front-matter: stderr = "grid task new: T-01 has no first Job: …", want it to hold "task words hold only front matter and no title"
    ij_task_title_test.go:110: row3-only-front-matter: task folders written: […/home/.grid/personal/tasks/T-01-task]
--- FAIL: TestIjTitleExplicitKeepsWordsAndOldTasks (0.00s)
    ij_task_title_test.go:119: row5-explicit-title: title = "---", want "Fix login"
    ij_task_title_test.go:122: row5-explicit-title: folder = "T-01-task", want it to end -fix-login
FAIL	github.com/gridconsole/grid-plugins/internal/cli	0.151s
```

(The pasted `aaaa…`/`bbbb…` and `…` paths are shortened here; the run printed the full strings.)

## Proof table

| # | Row | Result |
|---|-----|--------|
| 1 | Page shape: `title: "Fix: the login page"` gives that title and `T-01-fix-the-login-page`; without `title:` gives `Users cannot sign in` and `T-01-users-cannot-sign-in` | HOLDS. `go test -count=1 -run TestIjTitle -v ./internal/cli` exit 0: `--- PASS: TestIjTitleFromFrontMatter/row1-front-matter-title`, `--- PASS: TestIjTitleFromFrontMatter/row1-no-title-uses-body`. Base: FAIL above (`---`, `T-01-task`). |
| 2 | Blank lines before the block give `Real title` | HOLDS. `--- PASS: TestIjTitleFromFrontMatter/row2-blank-lines-before-block` (same run). Base: FAIL `---`. |
| 3 | Title plus a whitespace body gives `Fix login`; a block with no title and a blank body exits 2 with `task words hold only front matter and no title` and makes no `T-01-*` folder | HOLDS. `--- PASS: TestIjTitleFromFrontMatter/row3-title-and-blank-body`, `--- PASS: TestIjTitleOnlyFrontMatterIsRefused` (same run; the test walks the whole Grid home for a `T-01*` folder). Refusal: `internal/task/title.go:25`, called by Validate `internal/task/task.go:85`, after the unchanged `task words must not be empty` check (task.go:82-84). Base: FAIL above. |
| 4 | 300 `a` after the block gives 120 `a`; `title: ` + 300 `b` gives 120 `b` | HOLDS. `--- PASS: TestIjTitleFromFrontMatter/row4-long-body-line`, `--- PASS: TestIjTitleFromFrontMatter/row4-long-title` (same run). Base: FAIL `---`. |
| 5 | `title: "  Fix    login  "` gives `Fix login` and a folder ending `-fix-login`; intent.md is `IntentHead` + the words byte for byte; `Plain words\nsecond line` gives `Plain words`; the first task's task.md and folder are unchanged after more `task new` calls | HOLDS. `--- PASS: TestIjTitleExplicitKeepsWordsAndOldTasks` (same run). intent.md still written as `IntentHead+o.Words` (create.go:113, untouched). |
| 6a | `go test -count=1 ./internal/task ./internal/cli`: 0 fail | HOLDS. Both `ok` in the importer run below (internal/task 77.2s, internal/cli 303.5s); no FAIL line. |
| 6b | `make golden`: 0 fail, no golden changes | HOLDS. `make golden` ends `PASS` / `ok github.com/gridconsole/grid-plugins/internal/golden`; `make golden 2>&1 \| grep -cE -- "--- FAIL\|^FAIL"` prints `0`. No file under testdata/golden changed (`git diff main --stat` below). |
| 6c | `go vet ./internal/task ./internal/cli` exit 0; `gofmt -l internal/task internal/cli` prints nothing | HOLDS. `gofmt -l internal/task internal/cli && go vet ./internal/task ./internal/cli` printed nothing, and the `&&` chain went on to `git status`. `make lint` (`go vet ./...` plus `gofmt -l .`) exit 0. |
| 6d | `git diff main --stat` names only the Touches files | HOLDS. On code head 7e656046: `contract/README.md`, `internal/task/create.go`, `internal/task/task.go`, `internal/task/title.go`, `internal/cli/ij_task_title_test.go`. The report commit adds only `docs/lanes/C-ij-task-title-front-matter.md`, the report the runbook asks for. No change to internal/cli/task.go, its usage text, testdata/golden or the `--json` shape `{id, path}`. |
| R1 | Small commits, one-sentence messages, no attribution lines | HOLDS. `a75af240` (rule, Validate, test), `7e656046` (contract), then the report commit. Each has a one-sentence message and no attribution lines, because the owner's rule wins over the reminder. |
| R2 | New tests fail on base and pass on the lane; no existing test edited or deleted | HOLDS. FAIL lines above (base ee8a96c1), PASS lines in rows 1-5. The only test file is the new `internal/cli/ij_task_title_test.go` with the `ijTitle` helper prefix. |
| R3 | No guard, gate, cap, hold or blocking hook on agents | HOLDS. The only new refusal is the one the brief names (row 3), and it checks a person's request words in `task new`. It is not a check on agents. The 120-rune cut was already there (create.go:94-96 on base). |
| R4 | Tests of changed packages and every importer | HOLDS. Command: `go list -f '{{.ImportPath}}: {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./... \| grep -E "internal/(task\|cli)( \|$)"`. It lists cmd/grid, contract/formats, internal/account, braincmd, cli, golden, guard, hook, job, job/jobcmd, job/jobtest, jobgraph, land, mcp, migrate, orchestrator, packaging, question, settings, signal, workflow and lib. `go test -count=1 ./internal/task` plus all of these: every package `ok` (cmd/grid and job/jobtest have no test files), no FAIL. |
| R5 | Lint, Windows and macOS builds | HOLDS. `make lint` exit 0. `make cross` runs vet and test-compile for darwin/arm64 and windows/amd64 and exits on the first failure. Its last lines are windows `ok` lines such as `ok github.com/gridconsole/grid-plugins/lib`, so both targets passed. |
| R6 | RealPlugin tests | Not applicable: no row names one. |
| R7 | Bundle and report written | HOLDS. `git bundle create /srv/gridwork/tmp/hand/C-ij-task-title-front-matter.bundle origin/main..lane-C-ij-task-title-front-matter`; this file is copied to `/srv/gridwork/tmp/hand/C-ij-task-title-front-matter-report.md`. |

## What changed

- `internal/task/title.go` (new): `titleOf(words) (string, error)` is the rule in one function. It trims leading spaces, tabs and newlines, then calls `taskfile.ParseFrontmatter`. A `title` that is a string and not blank after `strings.Join(strings.Fields(..), " ")` wins. Otherwise the block's Body gives its first nonblank line. Words with no block use the old expression unchanged. A block with no usable title and a blank Body returns `task words hold only front matter and no title`. Every title is cut at 120 runes. The words themselves are never changed.
- `internal/task/create.go:93-96`: the inline title lines are replaced by `titleOf(o.Words)`. The slug (`slug(title)`) and intent.md (`IntentHead+o.Words`) are unchanged.
- `internal/task/task.go:85`: Validate calls `titleOf` after the empty-words check, so the new refusal exits 2 before anything is written (cli/task.go:129-132 prints `grid task new: <err>`).
- `contract/README.md:521`: the title sentence is now the four steps. The slug sentence and "These transformations never change `intent.md`" are untouched.
- `internal/cli/ij_task_title_test.go` (new): rows 1-5 through `grid task new --json -- <words>` on the cli harness.

## Notes and open items (not rows)

- A double-quoted `title` is JSON-decoded, so `title: "\u0007"` or `"‮"` can put a control or bidi character in the title. The whitespace fold does not remove these. `task rename` rejects such titles through `validTaskText` (rename.go:41-43), but `task new` takes them. This is outside Touches, so I added no check. Backlog item: decide whether such a title counts as "not usable".
- The leading trim is exactly spaces, tabs and newlines, as the brief says. With a Unicode space such as U+00A0 before `---`, there is no block, so the old rule applies and the title is `---`. Words cannot hold `\r`, `\v` or `\f` (checkTaskText).
- Requests that start with a Markdown horizontal rule now read as front matter. For example, `---\nIntro\n---\nRest` is now titled `Rest` (it was `---`), because ParseFrontmatter reads any `---` … `---` pair as a block. That is the rule as briefed. An empty block (`---\n---\n…`) is not a block for ParseFrontmatter, so it keeps the old rule.
- The app mirrors this rule (app internal/slices/tasks/newtask.go:189-198) and should follow the contract sentence at README.md:521.
- On the base, the harness printed `T-01 has no first Job: "grid@01LAPTOP" is not a valid <member>@<machine ULID> writer id` on stderr with exit 0. That message comes from the harness's machine id, not from this lane.

## NOT VERIFIED on a live screen

- Creating a task from the app's New task page through the app route (`grid task new --json -- <words>`), then seeing the title and folder in the app. This needs lane C-hw-newtask-page-body landed and a running app. This lane has no UI change.

DONE
