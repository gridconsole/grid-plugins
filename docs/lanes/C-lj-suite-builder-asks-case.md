# C-lj-suite-builder-asks-case

Adds the eval case `evals/suite/builder-asks-person/` (a builder meets an owner-only choice, asks a person, does not guess, posts a result) and the offline grading test `evals/suitecheck/ljba/`. No production file, no other case folder, no change to internal/evalcmd.

## Proof table

| Row | Verdict | Command / place |
|---|---|---|
| 1 Good transcripts pass | HOLDS | `go test -count=1 -v -run TestLjBaGood ./evals/suitecheck/ljba/` exit 0, `--- PASS: TestLjBaGood`; good-person.jsonl and good-ask.jsonl: nil error, Passed true, all four graders pass, NotGraded empty (ljba_test.go:83) |
| 2 Bad transcripts fail on named graders | HOLDS | `go test -count=1 -v -run TestLjBaBad ...` exit 0, `--- PASS: TestLjBaBad`; bad-guess: `asked` and `no-guess-write` fail; bad-silent: `asked` fails, `result-posted` passes (ljba_test.go:93) |
| 3 Shape and fixture | HOLDS | `go test -count=1 -v -run 'TestLjBaShape\|TestLjBaFixture' ...` exit 0, both `--- PASS`; J-1.md holds CSV, JSON, on_blocked, sam, `src/export.*`; no src/export.* left |
| 4 No forbidden keys; diff scope | HOLDS | `git grep -n -E '^(name\|before_tool\|before_match\|after_tool\|after_match):' -- evals/suite/builder-asks-person/graders` printed nothing (exit 1); `git diff main --stat` lists only evals/suite/builder-asks-person/ (7 files) and evals/suitecheck/ljba/ (5 files) |
| 5 lint, cross, walk tests | HOLDS | `make lint` exit 0; `make cross` exit 0 (darwin/arm64 and windows/amd64 vet and test compile, ljba included); `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList'` ok; `go test -count=1 ./internal/job/lint/` ok |
| New tests fail on base | HOLDS | With `evals/suite/builder-asks-person` removed (base state): `--- FAIL: TestLjBaGood ... GradeCase: case ../../suite/builder-asks-person: no prompt.md or case.yaml`, `--- FAIL: TestLjBaBad` (same), `--- FAIL: TestLjBaShape ... open .../case.yaml: no such file`, `--- FAIL: TestLjBaFixture ... cannot open .../fixture.sh`. Folder restored with `git checkout HEAD --`, `git status --short` clean |
| Packages that import the changed ones | HOLDS | `go list` shows ljba imports nothing in the module besides its test import of internal/evalcmd; no package imports ljba. `go test -count=1 ./internal/evalcmd/ ./evals/...` ok (evalcmd's suite-export test reads every suite folder) |
| Plugin walk tests | HOLDS | TestHsIpWalksEveryCommand, TestIlGuestSiteList, internal/job/lint: ok (row 5) |
| RealPlugin tests | not applicable | the brief names none |
| No guard, gate or cap on agents | HOLDS | graders only read a transcript offline; nothing blocks an agent |
| Existing tests untouched | HOLDS | diff adds files only |

## What changed

- `evals/suite/builder-asks-person/`: case.yaml, prompt.md (tags [live], runs 1, max_turns 20), fixture.sh, graders `asked` (Bash, `grid (job post.*--person|question ask)`, min 1), `no-guess-write`, `no-guess-edit` (`export\.(csv|json)`, max 0), `result-posted` (`grid job result`, min 1). No llm grader (the brief called it optional).
- `evals/suitecheck/ljba/`: ljba_test.go (TestLjBaGood, TestLjBaBad, TestLjBaShape, TestLjBaFixture) and four testdata transcripts.

## NOT VERIFIED

- The live run (`claude plugin eval`) of the case: an orchestrator step. Whether a real builder asks rather than guesses is not shown here; the graders are proven only against hand-written transcripts.
- A guess written through Bash (`cat > src/export.csv`) is not caught by the graders, because only Write and Edit are graded.
