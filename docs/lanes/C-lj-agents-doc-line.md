# C-lj-agents-doc-line

docs/agents.md line 5 now says what SessionStart puts in the context: no agent name; session lines, Job lines, brain index.

| Row | Result |
|---|---|
| 1. `git grep -n 'names these bare names' -- docs/agents.md` | HOLDS, exit 1, no output |
| 2. grep `TestSessionStartNamesNoAgents\|names no agent`; grep `sessionContext\|jobContext\|brain index` | HOLDS, exit 0, docs/agents.md:5 |
| 3. `go test -count=1 -v -run TestSessionStartNamesNoAgents ./internal/hook/` | HOLDS, exit 0, `--- PASS: TestSessionStartNamesNoAgents` |
| 4. diff names only docs/agents.md (plus this report); `make lint` | HOLDS, exit 0; `make cross` HOLDS, exit 0 (darwin/arm64, windows/amd64) |
| 5. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` | HOLDS, exit 0, ok |
| 5. `go test -count=1 ./internal/job/lint/` | HOLDS, exit 0, ok |
| Packages importing changed ones | none: no Go file changed |
| No guard/gate/cap added; nothing removed | HOLDS: one sentence pair replaced, rest word for word |
| New tests | none: doc-only; row 3 test pre-exists and pins the claim |

Doc points at `assembleContext`, `sessionContext`, `jobContext` by name, not line number.

NOT VERIFIED on a live screen: nothing UI. I did not re-read hook.go function bodies; the claim rests on the brief and the passing test.
