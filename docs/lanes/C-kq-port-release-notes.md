# C-kq-port-release-notes

Ports the eval case release-notes to the built-in `claude plugin eval` format: `evals/suite/release-notes/` plus the offline test `evals/suitecheck/kqrn/`.

Base: `evals/builtin-fit.md` (merged into this branch) says release-notes is `full` (nothing dropped), so the lane launched. Its answers (c) to (f) agree with the brief: `--eval-dir evals/suite`, `--case release-notes`, `--scaffold`, `--allow-tools Write`, `--trust-plugin`, `--no-publish`. (d) says a testdata folder is no case while nothing under it holds a `prompt.md` or `case.yaml`; kqrn/testdata holds neither. Where builtin-fit.md differs from the brief: nothing found. Its example flags name `Bash Write Edit`, but its row (f) says release-notes needs only Write, so the brief's `--allow-tools Write` stands.

## Proof table

| Row | Result |
|---|---|
| 1 TestKqRnShape | HOLDS. `go test -count=1 -v -run 'TestKqRn' ./evals/suitecheck/kqrn/` exit 0, prints `--- PASS: TestKqRnShape`. On base (stashed): `stat .../evals/suitecheck/kqrn: directory not found` / `FAIL ./evals/suitecheck/kqrn [setup failed]` |
| 2 TestKqRnMap | HOLDS. Same command prints `--- PASS: TestKqRnMap` (graders match the 4 check ids in ../../tasks/release-notes/checks.json; prompt body equals tasks prompt.md; fixture.sh equals setup.sh) |
| 3 TestKqRnGood | HOLDS. `--- PASS: TestKqRnGood`: Passed true, NotGraded empty, 4 graders (kqrn_test.go:117-123) |
| 4 TestKqRnBad | HOLDS. `--- PASS: TestKqRnBad`: files-bad fails only `fix`; files-none fails `notes` (kqrn_test.go:125-148) |
| 5 TestKqRnFixture | HOLDS. `--- PASS: TestKqRnFixture`: `sh fixture.sh` in t.TempDir() exits 0, changes.txt is the two exact lines |
| 6 diff scope | HOLDS with a note. `git diff 9de38d87 --stat` (my base merge commit) shows only 12 files under evals/suite/release-notes and evals/suitecheck/kqrn. `git diff origin/lane-C-kq-eval-builtin-fit --stat` also lists ~45 files from the lane-Pint120 merges already in this branch's base (internal/, docs/lanes, skills/questions); none is mine |
| `make lint` | HOLDS, exit 0 (`go vet ./...`, no output). `gofmt -l evals/` empty. No doc.go needed: vet accepts the test-only package |
| `make cross` | HOLDS, exit 0. Output lists `cross: darwin/arm64 vet`, `test compile`, `cross: windows/amd64 vet`, `test compile` |
| Packages that import the changed ones | None: kqrn is a test-only leaf; `go list -f '{{.ImportPath}} {{.TestImports}}' ./evals/...` lists no importer. It imports internal/evalcmd and internal/mdfile, which I did not change |
| Walk tests, plugin | HOLDS. `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| Walk tests, app (`make webui-test`, plugincli, grid-server, slices) | Not applicable: app rows; this is the plugin repo |
| New tests fail on base | HOLDS, see row 1 (package does not exist on base) |
| No existing test edited or deleted | HOLDS. Only new files |
| No guard, gate, cap or hold on agents | HOLDS. None added |
| *RealPlugin* tests | None in this lane |
| Commit rules | HOLDS. Two one-sentence commits, no attribution lines, no personal paths or hosts |

## What changed

- `evals/suite/release-notes/`: `case.yaml` (schema 1.1, name, scaffold_script), `prompt.md` (front matter name, runs 1, max_turns 10, allowed_tools Read and Write; body is the task prompt), `fixture.sh` (copy of setup.sh), `graders/notes.md` (file_exists RELEASE.md), `graders/heading.md`, `fix.md`, `feature.md` (regex on file RELEASE.md, pattern as `exportGrader` writes it; none needs escaping).
- `evals/suitecheck/kqrn/`: `kqrn_test.go` with TestKqRnShape/Map/Good/Bad/Fixture, and testdata (good.jsonl, files-good, files-bad, files-none/.keep).
- The test keeps its own copy of the type names in graderKeys (suitegrade.go:57-64), because graderKeys is unexported.

## NOT VERIFIED

- Live run of the built-in `claude plugin eval` on this case: not run (the brief reserves it for the orchestrator after landing). So it is unverified that the built-in accepts `max_turns` and `allowed_tools` in prompt.md and reads the grader files; the offline check only runs GradeCase.
- `evals/.gitignore` covers only `/runs/`; a live run without `--output-dir` writes `evals/suite/results/` (use the flags in the brief).

Verdict: DONE
