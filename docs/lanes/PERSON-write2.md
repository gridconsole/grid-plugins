# PERSON-write2: a person's answer is never lost quietly
Base: lane-PERSON-write (8c1e2000). Head: see the report.

Changed:
- Row 1, `exclusive` (internal/orchestrator/store.go): checks the task folder exists and is a folder (it never makes the task folder), runs `feed.CheckFilesystem`, makes the Feed folder with mode 0700, then opens `.orchestrator.lock`. A person's first Job post on a fresh task no longer fails with "no such file or directory".
- Row 2, `personAnswers` (internal/orchestrator/person.go): the earliest answer by At/ID is still kept. Each other eligible answer that reached the Job before the pass gets one `post` message on the Feed: `The answer of <who> to <job> at <at> (record <id>) was not used: <first> answered first, at <at>. It said: <text>`. The quote is cut at 3000 characters (a message holds 4000; the answer record keeps the whole text). The notices are written after the kept answer's message and before the accepting control, so a pass that stops between them repeats them rather than losing them; once the control lands the Job is closed and later passes write nothing. The second person reads it in `feed read`.
- Row 3, `job result` (internal/cli/orchestrator.go): on the unleased path it prints `Saved. The answer will be taken up on the next pass of the task's orchestrator.` on stderr and exits 0; `--json` writes the record with `"pending": true` added. A leased answer prints neither. An unleased Job post is unchanged (the fold counts it at once).
No guard, gate or cap was added on agents; the unleased path still admits only persons of the team.

New tests (each fails on base, passes on the lane). "Base" is the three source files at 8c1e2000 with the new test files in place.
- TestPersonFirstJobOfFreshTask (orchestrator): `open .../feed/.orchestrator.lock: no such file or directory` (person_lost_test.go:46).
- TestPersonFirstJobOnFreshTask (cli): `1 open .../T-07-example/feed/.orchestrator.lock: no such file or directory` (person_lost_test.go:28).
- TestSecondPersonAnswerIsNamedNotUsed (orchestrator): `want one message for the second answer, got []` (person_lost_test.go:109).
- TestSecondPersonSeesTheUnusedAnswerInFeedRead (cli): `feed read does not tell sam the answer was not used` (person_lost_test.go:72); the dump shows only "dana answered J-2 ..." and no notice.
- TestUnleasedAnswerSaysItIsPending (cli): `stderr does not say the answer is pending: ""` (person_lost_test.go:91).
Guards that pass on base and on the lane, so they are not counted above: TestExclusiveDoesNotMakeTheTaskFolder (the fix makes no task folder), TestSingleAnswerWritesNoNotice (a lone answer leaves no notice). Both second-answer tests sleep 1.1 s so the two answers get different At seconds.

Checks (exit code):
- go list -f imports ./...: importers of internal/orchestrator are internal/cli; of internal/cli, cmd/grid. 0
- go test ./internal/orchestrator ./internal/cli ./cmd/... -count=1: ok, ok, no test files. 0
- go test ./internal/... ./cmd/... -count=1: one FAIL, internal/eval TestFailuresAreSaved/timeout ("timeout did not stop the command", a 100 ms timeout hit during setup at 130 ms). Exit 1. Not mine: internal/eval imports none of the changed packages, and `go test ./internal/eval -run TestFailuresAreSaved -count=1` passes alone (ok, 0.120s). Every other package ok.
- gofmt -l internal/orchestrator internal/cli cmd: empty. 0
- go vet ./internal/orchestrator ./internal/cli ./cmd/...: 0
- GOOS=windows CGO_ENABLED=0 go build ./...: 0
- GOOS=darwin CGO_ENABLED=0 go build ./...: 0
- make lint: not run, as briefed.

Row 4: `go test ./...` was not run (the orchestrator does that at landing); `./internal/... ./cmd/...` above is the nearest, with the one unrelated timing failure. Goldens to regenerate: none. No file under contract/, no ops.yaml (api/ does not exist in this clone) and no generated file mentions `job result` or was touched.

NOT VERIFIED: a real `grid` process on a real task (a person at a shell or the app answering while a worker holds the lease, and the stderr text there); the UI's view of the new message; Windows and macOS test runs (builds only); the Feed folder's 0700 mode on Windows (the tests skip that assertion there); two answers landing in the same second (the tests keep them apart; the tick then orders by record ID, as before).
Not done: the second person is told only after the tick. Nothing is said at write time that another answer is already pending, because AnswerPerson would need a new return value and existing tests call it as it is.
