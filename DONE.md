# L36F completion

The full handoff was moved to [docs/lanes/L36F.md](docs/lanes/L36F.md). This root completion note records the final status.

Built and committed: deduplicated Personal signals for changed-routine timer refusals; launchd stdout/stderr paths and a 60-second StartInterval; UTF-16LE Task Scheduler files; entry-lock help; and schedule-time rejection of Grid home edge whitespace.

All acceptance checks passed with the default TMPDIR:

```sh
make test
make lint
go test -race ./internal/routine/... ./internal/cli
GOOS=windows go build ./...
GOOS=darwin go build ./...
```

Intended differences from the old plugin: refusal signals use the existing Go signal writer and unchanged last_run for deduplication (`old-plugin-map.md:63,115`; `docs/plugin-design.md:460-462`); local OS timers use corrected platform formats and retained routine logs (`docs/plugin-design.md:455,460`; `docs/go-architecture.md:33,386`); help explains the existing entry lock (`docs/plugin-design.md:460`); all schedule backends reject edge whitespace for consistent working directories (`docs/plugin-design.md:455,460`). Reference paths identify the read-only spec and old-plugin map. Inherited L36 differences are recorded in `docs/lanes/L36.md`.

Could not exercise live OS registration or execute macOS tests on this Linux host. Native Windows locks remain unsupported and outside the allowed edit scope; Windows retains previews. Routine log rotation was not added. The supplied L36 commit object was absent, so work continued from the provided checkout, which already contained L36. See the full handoff for details and test coverage.
