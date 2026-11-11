# C-ga-workflow-show-personal

Base sha: b87fc36458332a9e2bd680ff9e3ed5a896638900 (lane branch, from main).

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. Binary repro on a scratch GRID_HOME (lane build exits 0 with workspace file; plugin-dist 0.1.1979 exits 1) | NOT DONE | `go build -o /srv/gridwork/tmp/ga-personal-scratch/grid ./cmd/grid` exit 0, but running a built binary is refused in this sandbox (the allow list has no way to run it, and `env`/`GRID_HOME=` prefixes are refused), so I ran neither the lane binary nor the plugin-dist one. The same behaviour is covered in-process by row 2: `Read(r, "personal")` is what `grid workflow show --project personal` calls (internal/cli/workflow.go:59, no other project check before it). |
| 2. `go test -count=1 -v -run TestGaPersonalShow ./internal/workflow/` | HOLDS, exit 0 | PASS for owner/personal, owner/Personal, owner without workflows.md, guest, owner without Personal access, unknown name. The owner cases check one file with scope workspace, Project nil, the default template text, Rev set, no errors. Guest and no-access refusals are exactly `project is missing or ambiguous`; `nosuch` is `project "nosuch" is missing or ambiguous`. |
| 2. Fails on base | HOLDS | With files.go stashed to base: `ga_personal_show_test.go:25: Read("personal") error = project "personal" is missing or ambiguous`, same for `"Personal"`, and `ga_personal_show_test.go:38: Read = {Files:[] ...}, project "personal" is missing or ambiguous`; `--- FAIL: TestGaPersonalShow`. Guest, no-access and unknown-name subtests pass on base too: they pin the refusals that must not change. |
| 3. `go test -count=1 -run 'TestReadPreservesPersonalPolicy\|TestWritePersonalCallerPolicy' ./internal/workflow/` | HOLDS, exit 0 | `ok .../internal/workflow 0.038s`. The `git diff <base> --stat` lists files.go and the new test only, so personal_test.go and write_test.go are untouched. |
| 4. `go test -count=1 ./internal/workflow/ ./internal/cli/ -run Workflow` | HOLDS, exit 0 | both packages `ok`. `git diff b87fc364 --stat`: internal/workflow/files.go (13 lines), internal/workflow/ga_personal_show_test.go (67 lines), nothing else. |
| Packages that import workflow | HOLDS, exit 0 | direct importers from `go list`: internal/cli, internal/feed, internal/task. `go test -count=1 ./internal/workflow/ ./internal/feed/ ./internal/task/ ./internal/cli/` all `ok`. |
| Lint | HOLDS, exit 0 | `make lint` (`go vet ./...`) |
| Windows and macOS builds | HOLDS, exit 0 | `GOOS=windows CGO_ENABLED=0 go build -o /dev/null ./cmd/grid`; `GOOS=darwin CGO_ENABLED=0 go build -o /dev/null ./cmd/grid` |
| No guard/gate/cap added | HOLDS | The change widens one read; no hook, no limit. |
| No existing test edited or deleted | HOLDS | diff shows only files.go and the new test file. |
| New tests in the one new file | HOLDS | internal/workflow/ga_personal_show_test.go |
| No attribution lines, no personal paths | HOLDS | one-sentence commit message, none in the files. |

## What changed

internal/workflow/files.go, `readFiles`: when no registered project matches the selector, the selector equals "personal" by `strings.EqualFold` with `where.Personal`, and the caller is not a guest and `r.PersonalOK()`, the entries list is emptied and only the workspace `workflows.md` is read, as `scopeTemplates` does. Every other unmatched selector still returns `projectSelectionError`, so the guest, no-access and unknown-name texts are unchanged. A registered project folder named personal still wins, because `selectProject` runs first. `ReadPolicy` shares `readFiles`, so `--policy` works the same way.

The refusal check uses `!r.Caller.IsGuest() && r.PersonalOK()`, the same two conditions `projectSelectionError` uses, so a guest is refused even if a custom policy says yes (tested).

## NOT VERIFIED

- Row 1 binary run (lane binary exit 0, plugin-dist exit 1, with a default template) was not run: the sandbox refuses to run a built binary. A checker or the orchestrator can run it with a fresh scratch GRID_HOME.
- The app Judge end to end (judge.go:404) was not run.

NOT DONE: row 1 as written (binary run). Rows 2 to 4 and the rules hold.
