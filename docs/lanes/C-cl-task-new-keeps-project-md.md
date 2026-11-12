# C-cl-task-new-keeps-project-md: `grid task new` keeps the other keys and the body of project.md

Head: see the bundle (lane-C-cl-task-new-keeps-project-md). Base: main 23867517 (origin/main merged, already up to date).

## Proof table

| Row | Result |
|---|---|
| 1. `go test ./internal/task/ -run 'RegisterKeeps' -count=1 -v` PASS per case | HOLDS, exit 0. Last lines: `--- PASS: TestRegisterKeepsProjectMd (0.05s)` with PASS for `other_keys_and_body`, `other_keys,_no_body`, `body,_no_other_keys`, `body_with_a_rule_line`, `no_front_matter`, `new_project`; `ok  .../internal/task 0.059s` |
| 1b. Same test fails on main (before the fix, test file only) | HOLDS. Failing lines: `register_keeps_project_md_test.go:69: other keys lost, changed or reordered:` (output shows `---\nid: 01HZ…\nfolders: …\nrepos: …\nkeys: …\n---` with no `environments:`/`note:`), `:72: body lost or changed:`, `:79: other keys lost or text added:`, `:87: body lost or changed:`, `:94: body lost or changed:`; `--- FAIL` for the first four cases; `FAIL github.com/gridconsole/grid-plugins/internal/task 0.059s` |
| 2. `go test ./internal/task/ ./internal/projects/ ./internal/project/ -count=1` | HOLDS, exit 0: `ok …/task 2.414s`, `ok …/projects 0.287s`, `ok …/project 0.004s` |
| Importers of the changed packages | HOLDS: `go test ./... -count=1` exit 0 with no failing package (covers every importer: braincmd, cli, env, guard, hook, job, jobcmd, land, migrate, projectmd, settings, signal, updates, where, workflow) |
| 3. `make lint` | HOLDS, exit 0 (`go vet ./...`); `gofmt -l internal/` prints nothing |
| 4. `make cross` | HOLDS, exit 0: darwin/arm64 and windows/amd64 vet and test compile, all `ok` |
| Walk: `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS: `ok internal/cli` |
| Walk: `go test -count=1 ./internal/job/lint/` | HOLDS: `ok internal/job/lint` |
| Walk tests of the app repo (webui, grid-server, slices) | not run, they belong to the app repo, not this plugin lane |
| `git fetch origin && git merge origin/main` before the last run | HOLDS: `Already up to date.` |
| No guard/gate/hold added | HOLDS: no hook, no refusal; `register` only reads the old file |
| No existing test edited or deleted | HOLDS: one new test file only |

## What changed

- `internal/project/project.go`: new `KeepOldText(canonical, old)`: `KeepOtherKeys` plus the bytes after the closing `---` line. A text whose first line is not `---` adds no body.
- `internal/projects/projects.go`: `Store.write` calls `KeepOldText` instead of its own body loop (same result; the `bytes` import went).
- `internal/task/create.go`: `Store.register` reads the existing project.md and passes the formatted record through `KeepOldText` before `atomicWrite`. A missing file is no old text.
- `internal/task/register_keeps_project_md_test.go`: the new test (task new through `Store.New`, a git repo with a remote so repos and keys change; asserts id, folders, repos, keys, the `environments:` block and the extra key in order, and the body byte for byte).
- Project.md format and the Record type are unchanged.

## Notes

- "No front matter": `project.Load` already refuses such a project.md, so `task new` fails and does not rewrite the file; the test pins that the file is left as it was. This is unchanged behaviour on main, not new.
- Text after the front matter with its own `---` line is kept (case `body_with_a_rule_line`).

## NOT VERIFIED on a live screen

Nothing here is UI. Not run against the built real plugin (no `grid task new` binary run); the test goes through `Store.New`, as the brief asks.
