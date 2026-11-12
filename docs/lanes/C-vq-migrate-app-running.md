# C-vq-migrate-app-running: `grid migrate --apply` stops and asks to quit the Grid app when its worker or server runs on the home

Base: plugin main 792b8d6e53bb5a1c84a82c2e5da65bd638babfac. Code commit: 3fb45c60.

On base, as the brief says: `internal/migrate/apply.go:81` "Run offline: the lock serializes migrators, not old application writers." and `internal/cli/migrate.go:84` `res, err := p.Apply()`. Neither `internal/migrate` nor `migrate.go` reads `worker.lock` or `app-url` (`grep -rn "worker.lock\|worker-status\|app-url" internal/migrate` prints nothing; `migrate.go` names `worker.lock` only in `onlyWorkerFiles`, which reads no file). The refusal is a stop on one person's own migrate run; nothing is added for agents.

## What changed

- `internal/cli/vq_migrate_running.go` (new): `vqAppRunning(env Env) bool`. True when `worker.Store{GridHome: env.GridHome}.List()` has an entry with Presence "reported" (another handle holds `worker.lock`), or `appBaseURL(env)` is not "" and `listening(base)` is true. A `List` error does not count as running.
- `internal/cli/migrate.go`: inside `if *apply`, after the plan is printed and just before `p.Apply()`, `vqAppRunning(env)` true prints to stderr `grid migrate: The Grid app is running on <gridhome.Tilde(env.GridHome, env.Home)>. Quit the Grid app, then run grid migrate --apply again.` and returns 1. `--dry-run`, `--rollback` and "nothing to migrate" take no new step. (+6 lines, +1 import.)
- `internal/cli/vq_migrate_running_test.go` (new): eight tests, all `TestVqRun*`, helpers all `vqRun*`.

## Proof table

| Row | Result | Command and result |
|---|---|---|
| 1. held `worker.lock` refuses, tree untouched | HOLDS | `TestVqRunHeldWorkerLockRefusesApply` uses the layout of `TestMigrateInPlaceDefault` (`legacy/projects/example/project.md` in the Grid home, `migrate --apply`, no `--from`) and a `worker.lock` held by a second handle (`fslock.OpenNoFollow` + `fslock.TryLockExclusive`, kept open to Cleanup). It asserts exit 1, stderr equal to the whole refusal line, the plan on stdout, and a snapshot of every path and file byte under the root equal before and after (no backup, no journal, no `.migrate-previous-*`). **Red on base** (`go test -count=1 -v -run TestVqRun ./internal/cli/` on 792b8d6e with only the test file added): `want exit 1 and stderr "grid migrate: The Grid app is running on ~/.grid. Quit the Grid app, then run grid migrate --apply again.\n"`, `exit 0`, stdout `backup: …/home/.grid-backup/20261112T050823Z-f2fa73a4df36` / `migrate: applied`, `--- FAIL: TestVqRunHeldWorkerLockRefusesApply`. **Green on lane**: `--- PASS: TestVqRunHeldWorkerLockRefusesApply (0.00s)`, run exit 0. |
| 2. `app-url` live refuses; dead URL applies | HOLDS | Base: `--- FAIL: TestVqRunListeningAppURLRefusesApply` (`exit 0 … migrate: applied`); `--- PASS: TestVqRunDeadAppURLApplies` (it applies on base too, as it should). Lane: `--- PASS: TestVqRunListeningAppURLRefusesApply` (httptest server, no `worker.lock`; refusal, tree unchanged) and `--- PASS: TestVqRunDeadAppURLApplies` (URL of a closed httptest server; exit 0, `migrate: applied`, `projects/example/project.md` exists). `GRID_APP_URL` is set empty in every test so a value in the process environment cannot stand for the app. |
| 3. free `worker.lock` applies; `--dry-run` with held lock | HOLDS | `--- PASS: TestVqRunStaleWorkerLockApplies` (free `worker.lock` plus `worker-status.json`, Presence "stale": exit 0, `migrate: applied`); `--- PASS: TestVqRunDryRunWithHeldLock` (held lock and a live `app-url`: `migrate --dry-run` and bare `migrate` exit 0, plan printed, stderr empty, tree unchanged). Both pass on base too. |
| 4. worker-files-only home still "nothing to migrate" | HOLDS | `--- PASS: TestVqRunWorkerFilesOnlyHomeHasNothingToMigrate` (held `worker.lock` + `worker-status.json`; `migrate` and `migrate --apply` both exit 0 with `migrate: nothing to migrate`, tree unchanged). `go test -count=1 -run 'TestMigrateFreshHome\|TestMigrateCommand\|TestMigrateInPlaceDefault\|TestMigrateRollbackCommand' ./internal/cli/` -> `ok  …/internal/cli 0.364s`, exit 0; `migrate_fresh_test.go` and `migrate_test.go` unedited. |
| extra: `--from` on a fresh home with the worker running | HOLDS | `--- PASS: TestVqRunFromFolderRefusesWhileWorkerRuns` (red on base: `exit 0 … migrate: applied`). `--from` swaps the home the same way. |
| extra: label is `Tilde(GridHome, Home)` | HOLDS | `--- PASS: TestVqRunRefusalNamesTheHomeByTilde` (Grid home `~/elsewhere/grid`; red on base). |
| 5. `internal/migrate` | HOLDS | `go test -count=1 ./internal/migrate/` -> `ok  …/internal/migrate 18.736s`, exit 0. |
| 5. lint | HOLDS | `make lint` -> `go vet ./...`, exit 0. `gofmt -l internal/cli` prints nothing. |
| 5. cross | HOLDS | `make cross` exit 0: `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile`, every package `ok` or `no test files`, no FAIL. |
| 5. diff names only Touches | HOLDS | `git diff main --stat` at the code commit: `internal/cli/migrate.go \| 7 +`, `internal/cli/vq_migrate_running.go \| 22 +++`, `internal/cli/vq_migrate_running_test.go \| 259 +++`, 3 files, 288 insertions, 0 deletions. This report, `docs/lanes/C-vq-migrate-app-running.md`, is the one further file (the runbook asks for it). |
| tests of changed package and importers | HOLDS | Importers of `internal/cli` from `go list -test` (imports, test imports, external test imports): cmd/grid, contract/formats, internal/account, internal/braincmd, internal/golden, internal/jobgraph, internal/mcp, internal/packaging, internal/question, internal/settings, lib. `go test -count=1 ./internal/cli/ ./cmd/grid ./contract/formats ./internal/account ./internal/braincmd ./internal/golden ./internal/jobgraph ./internal/mcp ./internal/packaging ./internal/question ./internal/settings ./lib` -> all `ok` (`internal/cli` 144.424s, `lib` 60.155s; `cmd/grid` has no test files), exit 0. |
| walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` -> `ok …/internal/cli 0.569s`; `go test -count=1 ./internal/job/lint/` -> `ok …/internal/job/lint 0.451s`. The app-side walk tests (`make webui-test` etc.) do not apply to this repo. |
| new tests fail on base, pass on lane | HOLDS | Base, four red: TestVqRunHeldWorkerLockRefusesApply, TestVqRunListeningAppURLRefusesApply, TestVqRunFromFolderRefusesWhileWorkerRuns, TestVqRunRefusalNamesTheHomeByTilde (each `exit 0 … migrate: applied`). The other four pass on base by design (they pin what must not change). Lane: all eight `--- PASS:`, `ok …/internal/cli 0.514s`. No existing test edited or deleted. |
| *RealPlugin* tests, mismatch proof | n/a | No row names one. |
| cross-repo | HOLDS | `git grep -n -e "--apply" main` in the app repo: one hit, `docs/lanes/A96.md:7`, "runs `project scan <path> --json` (never `--apply`)". The app does not call `grid migrate --apply`. |
| no guard, gate or cap on agents | HOLDS | The one added stop is on `grid migrate --apply`, which a person runs on their own data. No hook, gate, cap or hold on an agent. |
| commits | HOLDS | One-sentence messages, no attribution line of any kind (the owner's rule wins over the reminder). |

## Open items

1. **A home that is already migrated is refused too.** The check sits before `p.Apply()` with no condition, as the brief says, so with the app running and the home at the current format `grid migrate --apply` prints the plan line and then the refusal, though `Apply` would change nothing. Seen (scratch test, removed): `exit 1`, stdout `migrate: already at format 1; no changes`, stderr `grid migrate: The Grid app is running on ~/.grid. Quit the Grid app, then run grid migrate --apply again.` A one-condition fix is `if !p.Current && vqAppRunning(env)` in `migrate.go`; not done because the brief did not ask for it. An interrupted-run recovery (`p.Interrupted`) is refused as well, which is right: recovery finishes or undoes a swap.
2. **The check is a look at one moment.** An app started after `vqAppRunning` returns and before the swap in `Apply` is not seen. Closing that needs a check inside `internal/migrate/apply.go`, outside Touches.
3. **`GRID_APP_URL` wins over the file** (`appBaseURL`, open.go:24). A shell that sets it to a listening server somewhere else makes `--apply` refuse for a home that server does not use. Taken as the brief specifies (`appBaseURL(env)`).
4. **`--rollback` also swaps the Grid home** and takes no check, as the brief says it is unchanged.

## NOT VERIFIED

- A live Grid app. The worker was stood in for by a second handle on `worker.lock` and the server by an `httptest` server; no real `grid-worker` or `grid-server` was started.
- Windows. The tests compile for windows/amd64 (`make cross`) but were not run there. They use `fslock`, which has a Windows implementation; whether the held-lock tests also pass on Windows is not shown.
- No screen is involved.

Verdict: DONE
