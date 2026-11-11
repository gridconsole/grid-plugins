# C-gg-projectmd-set-concurrent: two processes setting different project.md keys keep both

Base: `b87fc364` (clone's `main`; it contains `4e736654` named in the brief). Production code is not changed; the lane adds one test file.

Result: rows 1, 2, 3 and 5 HOLD. Row 4 is NOT DONE because the answer differs: a relink drops the keys (see below). Its red test is not handed in.

## Proof table

| # | Row / rule | Result | Proof |
|---|---|---|---|
| 1 | `TestGgProjectmdConcurrentGoroutines`: 12 goroutines set `k01`..`k12`; no error; each reads back; old keys and body byte equal | HOLDS | `internal/projectmd/gg_set_concurrent_test.go:252`. `go test -count=3 -race -v -run TestGgProjectmdConcurrent ./internal/projectmd/` exit 0: `--- PASS: TestGgProjectmdConcurrentGoroutines (0.28s)`, `(0.15s)`, `(0.26s)`. `ggCheckFile` (:92) holds the file to `---\n` + the fixture's own 3 front-matter lines + exactly the 12 `kNN: value-NN` lines + `---\n` + the body, byte for byte. |
| 2 | `TestGgProjectmdConcurrentProcesses`: 4 children x 5 distinct keys; all exit 0; 20 keys read back; no temp file beside project.md | HOLDS | `:292`. Same run: `--- PASS: TestGgProjectmdConcurrentProcesses (1.54s)`, `(1.38s)`, `(1.75s)`. `ggRunChildren` (:173) fails on any non-zero exit and prints that child's output; `ggNothingBeside` (:120) requires the folder to hold only `project.md` (the atomic writer's temp is `.project.md.*`). The test asserts 20 keys (`len(lines) != 20`). |
| 3 | Same key, two children, two values: both exit 0, reads back as one of them, file parses | HOLDS | `:312`. Same run: `--- PASS: TestGgProjectmdConcurrentSameKey (1.26s)`, `(1.14s)`, `(1.08s)`. Each child sets `shared` 5 times; the file must hold exactly one `shared:` line with `from-a` or `from-b`, and `Key` read returns no error. |
| 4 | Beside a relink: 2 children set keys while the parent runs a `.projects.lock` writer; every set key is present | NOT DONE | The answer differs, see "Row 4" below. Relink writer called: `projects.Store.Relink` (`internal/projects/projects.go:401`, lock taken in `change` at :409). No import cycle: `internal/projects` imports atomicfile, fspath, gitx, gridhome, localdata, project and where, none of which imports projectmd; the test compiled and ran in package projectmd. |
| 5a | `git diff main --stat` names only the new file | HOLDS | `git diff main --stat` exit 0: `internal/projectmd/gg_set_concurrent_test.go \| 324 +++`, `1 file changed, 324 insertions(+)`. |
| 5b | `go test -count=3 -race -run TestGgProjectmdConcurrent ./internal/projectmd/`: 0 fail | HOLDS | Exit 0, `ok  github.com/gridconsole/grid-plugins/internal/projectmd  9.919s` for three counts under `-race`: 0 failed, 9 passed, 3 skipped (the child helper, which skips without its marker). |
| 5c | `make lint` exit 0 | HOLDS | `make lint` exit 0 (`go vet ./...`, then `gofmt -l .` printed nothing). `gofmt -l internal/projectmd/` printed nothing. |
| R1 | Small commits, one-sentence messages, no attribution lines | HOLDS | One test commit with a one-sentence message and no Co-Authored-By line (the owner's rule wins over the reminder); this table is the second commit. |
| R2 | New tests fail on base and pass on the lane | NOT SHOWN | This lane changes no production code, so base and lane run the same Key and the three tests pass on both (`b87fc364`). A mutation proof (remove the lock in a scratch copy and show the tests go red) was not run: creating a scratch folder outside the clone was refused by the shell and production code may not be edited in this lane. The reviewer can do it in the review clone by changing `if value != nil {` at `internal/projectmd/projectmd.go:79` to `if false {` and reverting. |
| R3 | No existing test edited or deleted | HOLDS | `git diff main --stat` shows one added file, no deletions. |
| R4 | No guard, gate, cap, hold or blocking hook on agents | HOLDS | Test-only change. |
| R5 | Tests of the changed package and of every importer; lint; Windows and macOS builds | HOLDS | Importers of `internal/projectmd` from `go list -f '{{.ImportPath}} {{join .Imports " "}} {{join .TestImports " "}} {{join .XTestImports " "}}' ./...` filtered on that import: `internal/cli` only. `go test -count=1 ./internal/projectmd/ ./internal/cli/` exit 0: `ok .../internal/projectmd 0.303s`, `ok .../internal/cli 66.314s`. `make cross` exit 0 (darwin/arm64 and windows/amd64: `go vet ./...` and test compile of every package; the binaries are compiled, not run). |
| R6 | No test reads or writes a real home; four variables under `t.TempDir()`; `t.Fatal` when HOME is outside the root; children get the same four | HOLDS | `ggSetup` (:46) sets `HOME`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME` and `GRID_HOME` to folders under one `t.TempDir()` and `t.Fatalf`s when `HOME` is not under it (:56-58); the Grid home the resolver gets is the `GRID_HOME` folder. `ggRunChildren` (:173) appends the same four, from `ggEnvKeys` (:42), to every `cmd.Env`. The "harness `Env.Getenv`" of the brief does not exist in this package: `Key` takes a `where.Resolver{GridHome: home}` and no Env. Logs name keys and values like `k01`, never an email or token. |
| R7 | Children block on stdin; parent releases only after all started; no sleep, no timing assertion; each child prints only its own result line | HOLDS | `ggRunChildren` starts every child with `cmd.StdinPipe()` (:188) and closes every pipe (:200) only after the start loop; the goroutine test uses one channel closed after all goroutines are started. The file does not import `time`. The child prints one line `ggresult <name> set <n> keys`; Go's own `PASS` line also reaches stdout, so the parent keeps only lines with the `ggresult ` prefix and requires exactly the expected one. Waiting is on `cmd.Wait` exit codes. |
| R8 | Whole file under 20 s on a loaded machine | HOLDS on this run | 9.919 s for three counts under `-race` (about 3.3 s per count). The VM load at that moment was not measured. |
| R9 | `*RealPlugin*` tests, UI rows | N/A | None named. |

## Row 4: a relink drops every front-matter key that is not a project.Record field

Request: the test of row 4 (2 children x 5 keys `k01`..`k10`; the parent calls `store.Relink("ggproj", <temp>/linked)` 5 times between releasing the children and waiting on them; then reads every key back) and, to rule out a concurrency cause, one sequential run: `Key set k01=value-01`, then one `Relink`, then `Key read k01`. Both were run on the lane head and both fail with no production change.

Response, concurrent run (`go test -count=1 -v -run TestGgProjectmdConcurrentBesideRelink`, exit 1):

```
k01 reads <nil>, front-matter key "k01" is not set; want "value-01"
... (k02 .. k09 the same)
k10 reads <nil>, front-matter key "k10" is not set; want "value-10"
--- FAIL: TestGgProjectmdConcurrentBesideRelink
```

All 10 keys are missing. Response, sequential run (no concurrency at all), `project.md` after one set and one relink:

```
---
id: gg-project
folders:
- machine: <machine id>
  path: <temp>/linked
keys:
- key: folder:<temp>/linked
  machine: <machine id>
---
# Gg
...body, byte equal...
```

and `Key read k01` returns `front-matter key "k01" is not set`. The `lands` and `owner` keys that were in the file before are gone too.

Cause: `projects.Store.write` (`internal/projects/projects.go:468-500`) renders the front matter with `project.Write` (`internal/project/project.go:171`, `Format` at :159-168) from a `project.Record`, which has only `id`, `folders`, `repos` and `keys` (`internal/project/project.go:36-41`). It keeps the body after the front matter (`projects.go:490-496`) and nothing else. So a `Relink`, `Remove`, `Add` or `Apply` of an existing project drops every other front-matter key, whatever the lock does. The comment at `internal/projectmd/projectmd.go:80` ("so a relink cannot lose this edit") holds for the interleaving only: the lock orders the two writers, and the relink that runs second then drops the key.

Not changed: production code is out of scope for this lane. A fix belongs in a separate lane (keep the keys outside the Record the way `projectmd.Key` does with a `yaml.Node`, or an owner decision that those keys do not belong in project.md). When it lands, the row-4 test is the one described above and should go green with no change to the helpers in this file (`ggRunChildren` takes the children; the parent runs the relink between `ggRunChildren`'s start and its wait, which needs a callback parameter that was removed from the file because nothing used it).

## What changed

- New: `internal/projectmd/gg_set_concurrent_test.go` with `TestGgProjectmdConcurrentGoroutines`, `TestGgProjectmdConcurrentProcesses`, `TestGgProjectmdConcurrentSameKey`, the child helper `TestGgProjectmdConcurrentChild` (skips without its env marker) and the `gg` helpers.
- This proof table.
- Nothing else; no production file, no existing test.

## NOT VERIFIED

- Row 4 (above): NOT DONE.
- That the tests go red when the lock is removed (R2 above).
- Windows and macOS: compiled with `make cross`, not run. The child-process tests have not run on those systems.
- No UI in this lane; nothing needs a live screen.
