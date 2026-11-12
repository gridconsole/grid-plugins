# C-sa-brain-agent-render: a Job runs the agent the brain holds for its role

Base: main 2b47f62181522cb3036a3e38e186ec6d9e779eb8. Lane commits: 1e5accd6 (tests), 4e1ac5f5 (code), aa0abeac (reviewed-exports entry), then this report.
Commit identity on every lane commit: `Grid lane`, no attribution lines.

## Base check

None of the lines read knowledge/agents on base 2b47f621:
- internal/job/render.go:339-342 `gridplugins.AgentFor(contract.NormalizeRole(j.Role))` (embedded agents/*.md only).
- internal/settings/role.go:28-39 `ModelFor` answers `s.Effective()` for the catalog agent.
- internal/cli/worker.go:206-219 `workerSpawnModel` returns `st.ModelFor(role)`.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1 TestSaBrainAgentRendered red on base, green here | HOLDS | Green: `go test -count=1 -v -run TestSa ./internal/job/` prints `--- PASS: TestSaBrainAgentRendered (0.02s)`; it finds `### Instructions\n\n<body>\n\n` once, after `## Job\n`, from an Agent read by `Store.AgentFor` out of a temp home holding knowledge/agents/grid-researcher.md (internal/job/sa_brain_agent_render_test.go:41-61). Red on base: tests-only commit 1e5accd6 (`git switch --detach HEAD~1` from the code commit) fails to build: `internal/job/sa_brain_agent_render_test.go:37:82: unknown field Agent in struct literal of type job.RenderOptions`, `FAIL github.com/gridconsole/grid-plugins/internal/job [build failed]`. The base line that a researcher gets no section: on 2b47f621 `go test -count=1 -v -run TestRoleWithoutAgentFileGetsNoSection ./internal/job/` prints `--- PASS: TestRoleWithoutAgentFileGetsNoSection (0.03s)` (render_role_test.go:66-77). |
| 2 TestSaAgentForOrder, and a brain grid-builder.md replaces agents/builder.md | HOLDS | `--- PASS: TestSaAgentForOrder (0.01s)` with the sub-tests project beats home, grid-role beats role, names come before levels, orchestrator finds grid-planner, a key outside the agent fields skips the file (`hooks:`; next level returned, ok false when none), front matter that does not parse, nothing to find, line endings. Red on base: `internal/brain/sa_agent_for_test.go:34:21: s.AgentFor undefined`. Builder: `--- PASS: TestSaBrainAgentReplacesEmbedded (0.03s)` compares with `agentFileBody(t, "builder")` and checks inputs.json is unchanged but for brief_bytes. |
| 3 TestSaModelForWith | HOLDS | `--- PASS: TestSaModelForWith (0.00s)`: fable/high with nothing set, sonnet/high after `SetAgent("grid-researcher", Agent{Model: "sonnet"})`, project haiku/low beats both. Also `TestSaModelForWithLayers` (a personal value equal to the catalog's still beats the brain; empty brain field inherits; no catalog agent answers the brain values) and `TestSaModelForWithEmptyBrainIsModelFor` (14 roles x 2 stores equal ModelFor). Red on base: `internal/settings/sa_model_for_with_test.go:7:18: st.ModelForWith undefined`. |
| 4 TestSaWorkerSpawnModel | HOLDS | `--- PASS: TestSaWorkerSpawnModel (0.02s)`: no file opus/medium; `model: fable` in `<gridHome>/projects/example/knowledge/agents/grid-researcher.md` fable/medium; with `effort: high` fable/high; a `hooks:` file is skipped; a non-string model is no value; a personal `agent set --model haiku` beats the brain; role person answers the home brain file's values or empty. Red on base (assertion, not build): `sa_worker_spawn_model_test.go:27: workerSpawnModel("researcher") = "opus"/"medium", <nil>; want "fable"/"medium"`. The test sets `env.Cwd` to the fixture's registered folder: with `h.env.Cwd` empty the settings resolver answers "no folder to resolve" on base too. |
| 5 TestSa run, packages green, role tests unchanged | HOLDS | `go test -count=1 -v -run 'TestSa' ./internal/brain/ ./internal/job/... ./internal/settings/ ./internal/cli/` printed `--- PASS:` for TestSaAgentForOrder, TestSaBrainAgentRendered, TestSaBrainAgentReplacesEmbedded, TestSaModelForWith, TestSaModelForWithLayers, TestSaModelForWithEmptyBrainIsModelFor (the cli line was fixed by the Cwd line above and re-run: PASS). `go test -count=1 ./internal/cli ./internal/job/... ./internal/settings/ ./internal/brain/` exit 0 on aa0abeac. render_role_test.go and role_test.go are untouched (`git diff main --stat` lists neither). |
| 6 `make lint`, `make cross`, diff names only Touches | HOLDS with one named extra | `make lint` (go vet ./...) exit 0. `make cross` (darwin/arm64 and windows/amd64 vet and test compile) exit 0. `git diff main --stat` names the Touches files plus two: internal/cli/settings_choke_test.go (one entry, see below) and this report. |
| Rule: importers of changed packages | HOLDS | `go list -f '{{.ImportPath}} {{.Deps}}' ./...` filtered for brain, job, job/jobcmd, settings, cli gave 35 packages (cmd/grid, lib, account, approvefix, brain, braincmd, cli, doctor, env, feed, guard, hook, job, job/jobcmd, job/jobtest, job/lint, jobgraph, land, lease, mcp, migrate, orchestrator, packaging, packaging/cmd, people, ports, projectenv, question, routine, settings, signal, task, updates, usage, workflow, lib). `go test -count=1` over all of them: every package ok except internal/cli, which failed on TestSettingsStoreChokePoint only (see below); after the fix `go test -count=1 ./internal/cli` ok (34.150s). brain now imports settings; no cycle (settings' deps do not include brain). |
| Rule: repo-wide walk tests (plugin) | HOLDS | `go test -count=1 -v ./internal/cli -run 'TestSettingsStoreChokePoint\|TestSettingsPackageChokeMutations\|TestHsIpWalksEveryCommand\|TestIlGuestSiteList\|TestSaWorkerSpawnModel'` exit 0: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`, `--- PASS: TestSettingsStoreChokePoint`. `go test -count=1 ./internal/job/lint/` ok (0.199s). The app-side walk tests (make webui-test and the others) belong to the app repo and do not apply to this plugin lane. |
| Rule: tests fail on base, pass on lane | HOLDS | See rows 1-4; the test-only commit 1e5accd6 is red, the code commit 4e1ac5f5 is green. |
| Rule: no existing test edited, no test deleted | HOLDS with one named extra | No test deleted; no assertion changed. One entry added to the reviewed-exports map of internal/cli/settings_choke_test.go:114. |
| Rule: no guard, gate, cap or hold on agents | HOLDS | Nothing is refused or limited. A brain file that `brain check` flags is skipped by the reader, as the brief says; no new rule. |
| Rule: RealPlugin tests, no claude/curl, no background server | HOLDS | No row names a RealPlugin test; none run. |
| Rule: public repo, no personal paths, hosts or emails | HOLDS | Only temp paths and the project name "example" in tests. |

## What changed

- internal/brain/sa_agent_for.go (new): `Store.AgentFor(role) (md, level, ok)`. Names: the catalog agent (`settings.AgentForRole`, grid-planner for orchestrator), `grid-<role>`, `<role>`; for each name the levels in `Store.levels()` order (project, then home). A file is taken only if `ReadConfig` reads it and `frontKeys` parses it and every key is in `agentFields`; otherwise it is skipped. CRLF comes back as LF so `agentBody` finds the end of the front matter. brain imports settings for the one mapping: no cycle, so nothing is repeated.
- internal/job/render.go: `RenderOptions.Agent`; at the Instructions section the brain text is used when not empty, else the embedded agent. Same section text; inputs.json unchanged.
- internal/job/jobcmd/command.go: fills `Agent` from `w.store.AgentFor(j.Role)`.
- internal/settings/role.go: `ModelForWith(role, brain Agent)`: catalog, brain, personal, project; `ModelFor` calls it with an empty Agent. A role with no catalog agent answers the brain values and reads no settings file. It reads the two settings layers one at a time (the same directories `Effective` reads) so the brain's values sit under them.
- internal/cli/worker.go: `workerSpawnModel` reads the brain agent of the task's project folder (`brain.Store{GridHome: env.GridHome, Project: <folder>}`), takes `model:` and `effort:` as written, and calls `ModelForWith`.
- internal/cli/settings_choke_test.go:114: `"Store.ModelForWith": "role.go"` added to the reviewed read-only exports. TestSettingsStoreChokePoint was red without it (`settings_choke_test.go:37: ../../internal/settings/role.go: unreviewed settings export Store.ModelForWith`). The file is outside the Touches list but inside the cli package the brief names, and the brief's exported `ModelForWith` cannot pass that test otherwise. If the reviewer reads the Touches list as closed, this lane is STUCK on that one line.

## Decisions where the brief was silent

- Order of search is names first, levels second: a project `researcher.md` does not beat a home `grid-researcher.md`. Pinned in TestSaAgentForOrder "names come before levels". The other reading is one swap of the two loops.
- A brain file with front matter and an empty body is used as the Agent (the brief says "not empty"), so it renders an empty Instructions section rather than falling back to the embedded file.
- A `model:` or `effort:` that is not a plain string is read as no value (inherits).

## Notes for the reviewer (trust)

- The brain agent's body goes into the Job brief as written, unfenced, like the embedded agents. A body with its own `## Acceptance` style heading is not checked; the brief says AgentFor adds no new rule. Not tested here.
- `model` and `effort` reach the worker's `--model` and `--effort` as written (owner question 3); settings does not check them against the catalog lists.
- For a Personal task (folder not under projects/) the project name is the folder two levels up, as `settingsStore` already uses; an invalid name makes the project level fail and the search goes on to home.

## NOT VERIFIED on a live screen

Nothing here is UI. Not run: a real worker spawning claude with a brain agent's `model:` (needs the built plugin and claude; not allowed in this sandbox).
