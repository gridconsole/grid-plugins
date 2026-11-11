# C-win-plugin-gitattributes

Base: 55c46cbdeab67b5afcfcb42806c49caf2a12386e (main). Change commit: b60d7d87.

## Proof table

| Row | Verdict | Evidence |
|---|---|---|
| 1. Fails on main in a core.autocrlf=true clone | HOLDS | Clone of the base sha with `-c core.autocrlf=true`. `git ls-files --eol agents/builder.md` prints `i/lf w/crlf`. `grep -c -P '\r' agents/builder.md` prints 22. `go test ./internal/packaging/ -run 'TestGeneratedLayout\|TestShippedAgentsAndSkillsAreWellFormed'` exits 1. Failing lines: `plugin_files_test.go:249: agents/builder.md: front matter: does not open with a --- line` and `layout_test.go:31: bin/grid.cmd is stale; run make generate` (also .mcp.json and .claude-plugin/marketplace.json stale). |
| 2. Same clone from the lane branch | HOLDS | `git grep -l -I -P '\r'` in the autocrlf lane clone prints only `bin/grid.cmd`. `go test ./internal/packaging/ ./internal/routine/ ./internal/job/` exit 0 (packaging 4.4s, routine 0.6s, job 5.3s ok), including both row 1 tests. I used `git grep` instead of `grep -rlI`: the shell refused that form. Untracked files cannot hold CR in a fresh clone. |
| 3. `git add --renormalize .` changes no stored file | HOLDS | `git add --renormalize .` exit 0, then `git status --porcelain` printed only `?? .gitattributes`. |
| 4. Attributes and stored eol | HOLDS, one difference | `git check-attr`: agents/builder.md `text: auto`, `eol: lf`. The three .cmd files: `text: unset`. They also print `eol: lf`; the rule `* eol=lf` still applies, but is inert under `-text`. `git ls-files --eol bin/grid.cmd` prints `i/crlf w/crlf attr/-text`. `git ls-files --eol internal/packaging/grid.cmd` prints `i/lf w/lf attr/-text`. |
| 5. Tests and lint in the normal clone | HOLDS | `go test ./internal/packaging/... ./internal/routine/... ./internal/job/...` exit 0, 0 fail. `make lint` exit 0 (runs `go vet ./...`). |
| Windows build | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0 |
| macOS build | HOLDS | `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| Importers of changed packages | HOLDS | No Go package changed. I ran the three named packages and their subpackages (packaging/cmd, job/frontmatter, job/jobcmd, job/lint, job/jobtest). All ok. The `go list` import graph also shows the other importers of job and routine (cli, hook, lease, orchestrator, jobgraph, ...). The orchestrator runs the whole-module test at landing. |
| New tests fail on base, pass on lane | HOLDS by existing tests | The brief forbids Go files and testdata. The tests of row 1 are the failing tests: they fail on base (above) and pass on the lane (row 2). |
| No test edited or deleted | HOLDS | The diff is `.gitattributes` and this report only. |
| No guard, gate or cap on agents | HOLDS | None added. |
| No personal paths, hosts or emails | HOLDS | The files hold none. |
| No attribution lines in commits | HOLDS | One-sentence messages, no trailers. |

## What changed

- New `.gitattributes`: `* text=auto eol=lf`, `*.cmd -text`, `*.bat -text`, each with a one-line comment. No file's stored bytes changed (row 3).
- New `docs/lanes/C-win-plugin-gitattributes.md` (this report).

## NOT VERIFIED on a live screen

- Not run on a real Git for Windows machine. The autocrlf clone on Linux (`-c core.autocrlf=true`) stands in for it.
- Checkouts already made on Windows keep their CRLF files until the next `git add --renormalize .` or a fresh clone. This is how Git works and is not fixed by the file.

DONE
