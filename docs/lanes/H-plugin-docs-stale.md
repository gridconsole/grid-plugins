# H-plugin-docs-stale

## Proof table

| Row | Status | Proof |
| --- | --- | --- |
| 1. README section renamed "Commands", one line per group, pointer to `grid help` | HOLDS | README.md "## Commands". `git grep -n "not built yet" -- internal` (exit 0) still finds cli.go:98,201,274 and cli_test.go:320; the nil-run verbs in internal/cli/cli.go:110,116,124,127,146 are views, ask, machine, deploy, sync, and the README names exactly those. They match `"built": false` in contract/commands.json at lines 650, 2477, 3077, 3159, 5022. |
| 2. settings.md says writes take the Windows lock (fslock_windows.go) and keeps the read-only sentence | HOLDS | docs/settings.md:95-98. `GOOS=windows go vet ./internal/fslock/...` exit 0, no output. `GOOS=darwin go vet ./internal/fslock/...` exit 0. |
| 3. Every `grid <verb>` named in both edits is in commands.json | HOLDS | name line in contract/commands.json: version 5, where 50, open 80, doctor 102, hook 137, task 170, plan 486, job 655, lease 1765, feed 1912, mcp 2299, question 2313, answer 2482, approve 2506, deliver 2538, proof 2586, history 2652, land 2679, brain 2706, env 3082, ports 3107, person 3164, routine 3250, worker 3406, agent 3450, claude 3500, workflow 3546, skill 3604, session 3664, intent 3984, usage 4159, account 4357, project 4490, project-md 4581, setup 4621, signal 4651, artifact 4781, eval 4876, migrate 4981; not built: views 649, ask 2476, machine 3076, deploy 3158, sync 5021. settings.md names `agent set`, `skill enable|disable`, `hook level`, `setup`: agent 3450, skill 3604, hook 137, setup 4621. |
| 4. make lint | HOLDS | `make lint` ran `go vet ./...`, exit 0, no output. |
| Tests of changed packages | HOLDS | Docs only; no Go package changed, so no package imports a changed package. |
| Windows and macOS builds | HOLDS | The vet above covers the only package the docs cite; no code changed. |
| No guard, gate, cap, hold, hook added | HOLDS | Docs only. |
| No test edited or deleted | HOLDS | Docs only. |
| New tests fail on base | N/A | No new tests; docs change. |

## What changed
- README.md: "Commands working today" becomes "Commands", with one line per group from contract/commands.json, a pointer to `grid help`, and the five verbs that still print "not built yet".
- docs/settings.md:95-98: the stale Windows-cannot-write paragraph now says settings writes take the lock, on Windows in `internal/fslock/fslock_windows.go`; the read-only sentence stays.

## NOT VERIFIED on a live screen
Nothing here needs a live screen.
