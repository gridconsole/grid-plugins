# C-sa2-choke-list-row

Stacked on C-sa-brain-agent-render (its proof table: docs/lanes/C-sa-brain-agent-render.md).

| Row | Result |
|---|---|
| Entry `"Store.ModelForWith": "role.go"` next to `Store.ModelFor` | HOLDS internal/cli/settings_choke_test.go:114 |
| `go test -count=1 ./internal/cli` | HOLDS exit 0 (`ok .../internal/cli 59.979s`) |
| C-sa rows 1-4: TestSaBrainAgentRendered, TestSaBrainAgentReplacesEmbedded, TestSaAgentForOrder, TestSaModelForWith (+Layers, +EmptyBrainIsModelFor), TestSaWorkerSpawnModel | HOLDS `go test -count=1 -v -run TestSa ./internal/brain/ ./internal/job/... ./internal/settings/ ./internal/cli/` all `--- PASS:` |
| C-sa row 5 `go test -count=1 ./internal/job/... ./internal/settings/ ./internal/brain/` | HOLDS exit 0 (job, frontmatter, jobcmd, lint, settings, brain all ok) |
| `make lint` | HOLDS exit 0 (go vet clean) |
| `make cross` (Windows/macOS builds) | HOLDS exit 0 |
| `git diff origin/main...HEAD --stat` | HOLDS: C-sa Touches + settings_choke_test.go + docs only (11 files). Plain `git diff main` also lists main's newer commits, since the branch base is behind main. |
| Base-red proof for new tests | Carried from C-sa lane; not re-run here. The only new change is the one-line entry; before it, TestSettingsStoreChokePoint was red with "unreviewed settings export Store.ModelForWith". |
| Repo-wide walk tests (Imports, TestHsIpWalks..., lint) | NOT RUN here beyond `./internal/job/lint` (ok) and `./internal/cli` (ok); the orchestrator runs the rest at landing. |
| No guard/gate/cap added | HOLDS, one test-map entry only |

NOT VERIFIED on a live screen: nothing here is UI.
