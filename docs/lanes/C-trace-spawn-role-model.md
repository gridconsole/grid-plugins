# C-trace-spawn-role-model: every spawned Job runs on its role's model and effort

Trace rows TR-11 and TR-12. Base: plugin main `55c46cbd`.

## What changed

`grid worker next --json` now prints two more keys on a spawn line, `model` and `effort`
(`omitempty`, after `task_epoch`). They are the keys the app worker already decodes
(app `internal/kernel/worker/job.go` `Model` and `Effort`) and puts on the claude command as
`--model` and `--effort`. No app change, no flag, `contract/` does not move.

- `internal/settings/role.go`: `AgentForRole(role)` (role normalized with `contract.NormalizeRole`;
  `grid-<role>` when the catalog has it, `orchestrator` answers `grid-planner`, anything else false) and
  `Store.ModelFor(role)` (that agent's model and effort from `Effective()`: catalog, then personal, then project;
  two empty strings and no settings read for a role with no catalog agent).
- `internal/orchestrator/stream.go`: `Stream.Model func(taskDir, role string) (model, effort string, err error)`;
  `spawnLine` gains `Model` and `Effort`. A nil `Model`, or empty answers, print neither key. An error is one Diag line
  naming the task, the Job and the error; the spawn line is still printed without the keys and `sent` is set.
  `Spawn` (store.go) gains `Role`, filled at tick.go from `j.Role`.
- `internal/cli/worker.go`: `Model` is `workerSpawnModel`, which takes the project folder name from the task folder
  (as the `Configure` callback does) and reads through `settingsStore(env, name, true)`. A project folder with no
  record (`where.ErrProjectUnregistered`) reads the personal store.
- `agents/curator.md` front matter `model: sonnet` to `model: opus`, so the file agrees with the catalog.
- `internal/cli/settings_choke_test.go`: two entries added to its reviewed-exports list (`AgentForRole`,
  `Store.ModelFor`, file `role.go`). See the deviation below.

A person's `grid agent set` wins over the catalog. The values are defaults the owner may change, not limits.

## Deviations from the brief (for the reviewer)

1. The brief says worker.go calls `settings.Store{Home: env.GridHome, Project: <name>}.ModelFor(role)`.
   `TestSettingsStoreChokePoint` (internal/cli/settings_choke_test.go) forbids any production `settings.Store`
   outside `settingsStore`, so worker.go goes through `settingsStore(env, name, true)` (the read form) instead.
   `settingsStore` answers `ErrProjectUnregistered` for an explicit unregistered project name, so worker.go
   retries with `gridhome.Personal`, which is the personal store the brief asks for.
2. The same test keeps a reviewed list of exported settings functions; `role.go` is a new file the brief asks for,
   so its two exports are added to that list. No assertion was changed or weakened.

## Proof table

| Row | State | Evidence |
|---|---|---|
| 1 | HOLDS | `go test ./internal/cli -run WorkerNextSpawnCarriesTheCatalogModel` exit 0 (`worker_next_model_test.go`, child process `worker next --json`, fixture role `builder`). On base (worker.go and curator.md checked out from `55c46cbd`): `worker_next_model_test.go:108: builder spawn model/effort = ""/"", want sonnet/medium` |
| 2 | HOLDS | `TestStreamSpawnCarriesTheRoleModel` (spawn_model_test.go) passes: builder and grid-builder sonnet/medium, checker sonnet/low, researcher opus/medium, orchestrator opus/medium. On base the file does not compile (`Stream.Model`, `ModelFor` do not exist), so there is no assertion line to paste |
| 3 | HOLDS | `TestWorkerNextSpawnFollowsPersonalAndProjectSettings` passes: personal opus + catalog medium; project `reg` haiku; registered `other` and unregistered `example` opus; stderr has no settings error. On base: `unregistered project folder: model/effort = ""/"", want opus/medium`, `project with its own model: ... want haiku/medium`, `another registered project: ... want opus/medium` |
| 4 | HOLDS | `TestAgentForRole` (role_test.go) false for `person`, `Person`, empty, `no-such-role`; `TestStreamSpawnWithoutAModelKeepsTheNineKeys` nil `Model` and empty answers print exactly the nine keys |
| 5 | HOLDS | `TestStreamSpawnSurvivesBrokenSettings`: spawn still printed, nine keys, one Diag line `task <id>: job J-1 assigned, no model or effort for role "builder": settings.json: invalid character ...`, second pass prints nothing new |
| 6 | HOLDS | `workerReads` in worker_next_model_test.go decodes the spawn object with the two tags `json:"model"` and `json:"effort"`; no app edit |
| 7 | HOLDS | `go test ./internal/orchestrator ./internal/settings ./internal/cli .` exit 0 (`.` has no test files); importers `./internal/doctor ./internal/guard ./internal/hook ./internal/job` ok; `make lint` exit 0 |
| 8 | HOLDS | `TestAgentFilesMatchTheCatalog` passes (builder, reviewer, curator, orchestrator). On base curator.md: `role_test.go:91: agents/curator.md says sonnet/medium, the catalog says opus/medium` |
| Builds | HOLDS | `GOOS=windows CGO_ENABLED=0 go build ./...` exit 0; `GOOS=darwin CGO_ENABLED=0 go build ./...` exit 0 |
| No guard | HOLDS | nothing blocks, holds or caps a Job; an error in `Model` costs only the two keys |
| New tests fail on base | HOLDS | rows 1, 3, 8 above; rows 2, 4, 5 need the new `Stream.Model` field and do not compile on base |
| Existing tests | HOLDS | no existing assertion edited; only the reviewed-exports list in settings_choke_test.go gained two entries (deviation 2) |
| Public repo | HOLDS | no personal path, host or email in the diff |

## Packages that import the changed ones

`go list` of importers of `internal/orchestrator`, `internal/settings` and the root package: `internal/cli`,
`internal/doctor`, `internal/guard`, `internal/hook`, `internal/job`. All tested. The whole-module test and the
cross-repo set are the orchestrator's at landing.

## NOT VERIFIED on a live screen

A real worker starting claude with `--model` and `--effort` from these keys (no claude run here); the app side is
read from the brief's quote of app main `35c29baf`, not run.
