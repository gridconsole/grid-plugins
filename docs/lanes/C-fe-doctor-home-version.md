# C-fe-doctor-home-version

Base sha: 06047047ac70ebf6bba163d448a121c627a363a1

| Row | Result |
|---|---|
| A1 hooks-only home: `fresh home: run grid setup`, fix "Run grid setup" | HOLDS: `go test -count=1 ./internal/doctor/ -run TestFreshHomeVersion` exit 0 (A1 subtest, fe_home_version_test.go) |
| A2 sessions+projects, no VERSION: `cannot read` | HOLDS: same test, subtest A2 |
| A3 sessions, VERSION "x": `cannot read` | HOLDS: same test, subtest A3 |
| A4 empty home: `cannot read` | HOLDS: same test, subtest A4; doctor_test.go:26-27 unedited |
| A5 `go test -count=1 ./internal/doctor/ ./internal/cli/` | HOLDS: exit 0 (doctor ok, cli ok). `make lint` (go vet ./...) exit 0. Diff names the three files plus this note. |
| A6 A1 test fails on base | HOLDS: on base 06047047 (test file only): `fe_home_version_test.go:39: format version: want a failure containing "fresh home: run grid setup", got {... Detail:cannot read ~/.grid/VERSION Fix:Run grid migrate; if VERSION is missing or invalid, restore it from a known backup.}` and `fix = "Run grid migrate; ...", want prefix "Run grid setup"`; FAIL exit 1 |
| Packages importing doctor | go list: internal/cli only (tested, ok) |
| Windows build `GOOS=windows CGO_ENABLED=0 go build ./...` | HOLDS exit 0 |
| macOS build `GOOS=darwin CGO_ENABLED=0 go build ./...` | HOLDS exit 0 |
| gofmt -l internal/doctor | HOLDS, no output |
| No guard/gate/cap on agents; no existing test edited or deleted | HOLDS |
| *RealPlugin* tests | not applicable: none touched |

## What changed
- doctor.go: checkFormat adds a case for a missing VERSION in a fresh home; new helper `freshHome` (sessions/ is a dir, projects/ absent). The helper is a small addition beside checkFormat.
- installation.go: fixHint "format version" returns "Run grid setup" for details starting "fresh home".
- fe_home_version_test.go: new test with A1-A4.

## NOT VERIFIED on a live screen
Real `grid doctor` output on a hooks-only account. The rebase onto C-ex-doctor-deny-block is the orchestrator's or the later lane's step; hunks do not overlap.
