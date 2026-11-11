# C-lv-signal-sender-flags

`grid signal add` takes `--sender` and `--recipient` and the signal note records them as `sender:` and `recipient:` front matter keys.

Base: `3ab3b0ce` (plugin main as cloned). Commits on the lane: `f4b27d9e` (signal package), `ca57e069` (CLI flags, role policy, contract), `4f9dd754` (skill text), then this report.

## Proof table

| Row | Verdict | Proof |
|---|---|---|
| 1. Red on base, green on lane | HOLDS | Base, new CLI test run before any production edit: `go test -count=1 -run TestLvSig ./internal/cli/` printed `--- FAIL: TestLvSigAddRecordsSenderAndRecipient` / `lv_signal_sender_cli_test.go:48: exit 2: flag provided but not defined: -sender`. The signal package test did not compile on base: `go vet ./internal/signal/` printed `lv_signal_sender_test.go:49:72: unknown field Sender in struct literal of type Input`. Lane: `go test -count=1 -run 'TestLvSig' -v ./internal/signal/ ./internal/cli/` prints `--- PASS: TestLvSigAddRecordsSenderAndRecipient` (the front matter holds `sender: Marieke` and `recipient: the vertex address`, the add answer is `written`). The same command `signal add --source mail --source-id m-1 --title T --sender Marieke --recipient "the vertex address" --body -` is the one the test runs in a `newHarness` project. I did not run `grid signal add --sender x` as a standalone binary: a command starting with `GOFLAGS=` was refused, so the base line above comes from the harness test (`internal/cli/signal.go:41-46` on base has no such flag). |
| 2. No flags, same bytes as base | HOLDS | `go test -count=1 -run TestSignalsGolden ./internal/cli/` passes (`--- PASS: TestSignalsGolden`); `git diff main --stat -- testdata/golden` prints nothing. `TestLvSigAbsentFieldsWriteNoKey` pins the key set and order `id source source_id title status captured updated tier body_kind`; `TestLvSigAddWithoutFlagsWritesNoKeys` checks the CLI note. |
| 3. Newline refused, over 512 refused | HOLDS | `TestLvSigAddRefusals` (4 subtests: sender/recipient newline, sender/recipient 513 bytes): stderr holds `sender must be one line`, `recipient must be one line`, `sender must be UTF-8 text of at most 512 bytes`, `recipient must be UTF-8 text of at most 512 bytes`; the exit code equals the one a bad `--source-id` (newline) gets in the same run, which is 1 (`failure`, `internal/cli/projects.go:47-51`; exit code unchanged); the notes folder holds no file after the refusal. `TestLvSigValidate` adds CR, U+2028, a header-injection value (`x\nstatus: done`), invalid UTF-8, and checks that exactly 512 bytes pass. |
| 4. Keys survive act, snooze, wake, move | HOLDS | `TestLvSigKeysSurviveActs` runs snooze, wake, note, reopen, add (T-7), reopen, dismiss through `Apply` and checks both lines in the file after each; `TestLvSigKeysSurviveMove` moves the note to a second project (`Move` patches the status scalar in the original header) and snoozes it there. Both pass. Act works because `Note` now carries the fields (`internal/signal/signal.go` Note struct), since `Apply` re-encodes from the struct (`act.go:195`). |
| 5. roleFlag skips the new values | HOLDS | `TestLvSigRoleFlagSkipsSenderAndRecipient`, 8 cases (values starting with `-`, `--act=turn` as a sender or recipient value, both together, single-dash spelling, `=` form, fence). On base 5 of the 8 fail, for example `roleFlag(["signal" "act" "S-1" "--act" "dismiss" "--sender" "--act=turn"]) = "turn", want "dismiss"`; on the lane all 8 pass. Change: `internal/cli/role_policy.go:167`. |
| 6. Named test run, lint, cross | HOLDS | `go test -count=1 -run 'TestLvSig\|TestSignal\|TestContractCommandsParserInventory\|TestContractCommandsInventory' -v ./internal/signal/ ./internal/cli/`: `ok internal/signal`, and in `internal/cli` all `TestLvSig*`, `TestSignal*`, `--- PASS: TestContractCommandsInventory`, `--- PASS: TestContractCommandsParserInventory`. `make lint` (`go vet ./...`): completed, no error. `make cross` (darwin/arm64 and windows/amd64 vet and test compile of every package): completed, no error. `gofmt -l internal/signal internal/cli`: no output. |

| Rule | Verdict | Proof |
|---|---|---|
| Contract file | HOLDS | Edited by hand: `--recipient` and `--sender` objects in the `signal add` entry, after `--project` and before `--source` (`contract/commands.json`); `TestContractCommandsParserInventory` compares the whole file to the parsers and passes, so the bytes are the encoder's. `git log HEAD..main` is empty, so there was nothing to merge. |
| Packages that import the changed ones | HOLDS | `go list -f '{{.ImportPath}} {{join .Deps " "}}' ./...` filtered on `internal/signal` and `internal/cli` gives: `cmd/grid`, `internal/cli`, `internal/signal`, `lib`. `go test -count=1 ./cmd/grid ./internal/cli ./internal/signal ./lib`: `cmd/grid` no test files, `internal/cli` ok 43.5s, `internal/signal` ok, `lib` ok. |
| Walk tests (plugin) | HOLDS | `go test -count=1 ./internal/cli -run 'TestHsIpWalksEveryCommand\|TestIlGuestSiteList' -v`: `--- PASS: TestHsIpWalksEveryCommand`, `--- PASS: TestIlGuestSiteList`. `go test -count=1 ./internal/job/lint/`: `ok`. The app walk tests (`make webui-test` and the others) belong to the app repo and were not run here. |
| New tests fail on base, pass on lane | HOLDS | Row 1 and row 5 lines above. Tests are in the two new files only, named `TestLvSig*`, helpers `lvSig*`. No existing test edited or deleted. |
| No guard, gate, cap or hold on agents | HOLDS | The 512-byte limit is a field-format check on the same footing as source id, in the brief's Touches list; nothing is added to an agent's path. |
| Files touched | HOLDS | `internal/signal/signal.go`, `internal/cli/signal.go`, `internal/cli/role_policy.go`, `contract/commands.json`, `skills/signals/SKILL.md`, the two new test files, this report. Nothing else. |
| `*RealPlugin*` tests | NOT NEEDED | The brief names none; no row depends on a built plugin. |
| UI rows | NOT NEEDED | Plugin-only lane; the app read is C-lv-inbox-sender-origin. |

## What changed

- `internal/signal/signal.go`: `Input` and `Note` gain `Sender` and `Recipient` (`sender,omitempty`, `recipient,omitempty`, placed after `title`); `Validate` checks both with `textField` at 512 bytes; `Add` copies them. `Input` JSON tags are `sender,omitempty` and `recipient,omitempty`.
- `internal/cli/signal.go`: `add` flags `--sender` ("who sent it, as the source names them") and `--recipient` ("the address or channel it came to").
- `internal/cli/role_policy.go:167`: `sender` and `recipient` join the value-taking flags `roleFlag` skips.
- `contract/commands.json`: the two flag objects.
- `skills/signals/SKILL.md`: the add command shows both flags, and one sentence says when to give them.

Behaviour worth knowing, pinned by tests:

- A second `add` with the same source and source id is a `duplicate` and keeps the first note's sender and recipient (`TestLvSigDuplicateKeepsFirstSender`).
- Values the YAML dump could read as another type (`true`, `123`, `null`, `~`, `yes`, `1e3`, `0x1F`, `a: b`, `# c`, leading and trailing space, quotes, brackets, emoji) read back as the same text (`TestLvSigRoundTripsLookalikes`).
- A sender of only spaces is accepted, as a source id of only spaces is today; `textField` is the only check, as the brief says.

## NOT VERIFIED on a live screen

- Nothing here draws a screen. The inbox line "Email from Marieke to the vertex address." is the app's read (C-lv-inbox-sender-origin) and the app's real-plugin rigs (`ez_inbox_signal_add_realplugin_test.go`, `gd_inbox_act_realplugin_test.go`) were not run in this lane; the orchestrator's cross-repo set covers them.

## Verdict

DONE
