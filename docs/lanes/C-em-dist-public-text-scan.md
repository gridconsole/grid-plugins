# C-em-dist-public-text-scan

Base: bf4b8e210aa356c0bc6c2f3ff1bfeac173558c53 (main side d528178a content). Read dist.go:60-82 (stage copies skills, agents, agent-templates, LICENSE) and publish_set_test.go:1-60 (folder names only). `grep -rn 'Users/\|@gmail\|/home/' internal/packaging --include=*_test.go` printed nothing, so no existing test scans file text.

| Row | Result |
|---|---|
| 1. TestEmScan on the real staged tree passes on base | HOLDS. `go test -count=1 -v -run TestEmScan ./internal/packaging/` exit 0, `--- PASS: TestEmScan`, zero hits. |
| 2. Planted hits reported exactly, allowed lines not | HOLDS. `TestEmScanFindsPlantedHits` PASS: reports lines 2 `/Users/ana`, 4 `/home/ana`, 6 `C:\Users\ana`, 8 `/srv/`, 10 `ana@gmail.com`, 12 `10.1.2.3`; not `~/.claude/agents`, `sam@example.com`, `127.0.0.1`, `192.0.2.7`, `git@github.com`, `1.2.3`. Also proves a non-UTF-8 file is skipped. |
| 3. Coverage per top-level entry, fails on zero | HOLDS. em_public_text_scan_test.go:131-143 logs counts and errors on zero for skills (13), agents (4), agent-templates (12), LICENSE (1), hooks (1), .mcp.json (1), .claude-plugin (2). Also read: .codex-plugin 1, bin 2. |
| 4a. diff names only the new file | HOLDS for production and tests: `git diff main --stat` lists only internal/packaging/em_public_text_scan_test.go, plus this report under docs/lanes (required by the runbook). |
| 4b. `go test -count=1 -run TestEmScan ./internal/packaging/` | HOLDS, exit 0. |
| 4c. `make lint` | HOLDS, exit 0 (runs `go vet ./...`). |
| 4d. `GOOS=windows go vet ./internal/packaging/` | HOLDS, exit 0. `GOOS=darwin` likewise exit 0. |
| Importers tested | HOLDS. `go list` gives internal/packaging and internal/packaging/cmd; `go test -count=1` on both: ok. |
| New test fails on base | N/A: the lane adds only a test, and the real tree is clean, so TestEmScan passes on base by design (row 1). The planted test fails if any pattern is removed. |
| No guard on agents; no existing test edited or deleted; no attribution lines; no personal paths | HOLDS. |

Names: helpers `emScan*`, tests `TestEmScan*`; reuses `moduleRoot` and `writeTestFile` from the package.

Changed: one new test file. No production, skill or template text touched.

NOT VERIFIED on a live screen: nothing here is UI.

DONE
