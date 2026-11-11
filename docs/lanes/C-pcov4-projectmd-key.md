# C-pcov4-projectmd-key: tests for the refusals and role branches of projectmd Key

Head: see the report line "head sha" (the commit that adds this file).

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1 Coverage before and after | HOLDS | before (untouched clone): `ok .../internal/projectmd coverage: 76.6% of statements`; after: `coverage: 84.1% of statements` (`go tool cover -func`: `total: (statements) 84.1%`, Key 81.9%, jsonValue 100.0%). 84.1 > 76.6. The profile went to a folder under /srv/gridwork/tmp because `mktemp` is outside the shell allow list. |
| 2 Per role: owner, member, empty Caller | HOLDS | `TestKeyPerRoleNonGuestReadsAndSets` (projectmd_key_test.go:44) subtests `owner`, `member`, `empty_caller`: read returns `reviewed`, set returns `shipped`, project.md equals the fixture with `lands: shipped`. Exit 0. |
| 2 Per role: guest, external, grid-guest | HOLDS | `TestKeyPerRoleGuestReadsAndCannotSet` (:76): read returns `reviewed` from the invited project (project-roles.json people entry `guest-1`); set returns exactly `guests cannot write project.md` (projectmd.go:66-68); project.md byte for byte equal to the fixture. |
| 3 Not found per role | HOLDS | `TestKeyNotFoundPerRole` (:99): owner and member get `project "nowhere" not found` (projectmd.go:62); guest, external and grid-guest get exactly `project not found` (:60) for an unknown name and for the existing `other` they are not invited to; a guest not on the people list also gets it for `sample`, on a set. |
| 4 Set refusals, file unchanged | HOLDS | `TestKeySetRefusesValuesJSONCannotCarry` (:126): `.nan` gives `value cannot be represented as JSON: json: unsupported value: NaN`, `.inf` gives `... +Inf` (:128), `a\n---\nb` gives an error containing `expected single document`; project.md equals the fixture after each. The code matched the brief. |
| 5 Read refusal | HOLDS | `TestKeyReadRefusesNonStringMappingKeys` (:148): `extra: {1: a}` and `extra:` holding a list with `{1: a}`; read gives exactly `value needs string mapping keys` (projectmd.go:166-168); a set of `lands` still succeeds. **Difference from the brief:** the writer re-indents a block list, so after the set the list case holds `extra:\n- {1: a}` (no two-space indent before the dash); the test asserts the whole file as written today. The mapping case keeps `extra: {1: a}` as it was. |
| 6 jsonValue direct | HOLDS | `TestProjectmdKeyJSONValue` (:182): `[]any{map[any]any{}}` false, `map[string]any{"a": []any{map[any]any{}}}` false, `map[string]any{"a": []any{"x"}}` true, scalar true (also a bare `map[any]any` false and nil true). jsonValue 100.0%. |
| 7 go test | HOLDS | `go test -count=1 ./internal/projectmd/` exit 0, ok. |
| 7 windows vet | HOLDS | `GOOS=windows go vet ./...` exit 0. |
| 7 darwin vet | HOLDS | `GOOS=darwin go vet ./internal/projectmd/` exit 0. |
| 7 windows test compile | HOLDS | `GOOS=windows go test -c -o /dev/null ./internal/projectmd/` exit 0. Compiled only; no Windows run. |
| 7 lint | HOLDS | `make lint` (runs `go vet ./...`) exit 0; `gofmt -l internal/projectmd` prints nothing. |
| Rule: importers of the package | HOLDS | `go list` of non-test imports of internal/projectmd gives `internal/cli` only. `go test -count=1 ./internal/cli/ ./internal/projectmd/` exit 0 (both ok). |
| Rule: tests only, no non-test file, no existing test edited | HOLDS | `git diff --stat origin/main..lane` shows internal/projectmd/projectmd_key_test.go and this file under docs/lanes. |
| Rule: new tests fail on base | NOT APPLICABLE | The lane adds no production code; the tests pin behaviour that already exists, so they pass on base too. Before the file exists none of the new test names exist. |
| Rule: no guard, gate, cap or hold added | HOLDS | Nothing outside test code and this report changed. |
| Rule: no personal paths, hosts or emails | HOLDS | The new test uses only `t.TempDir()` and made-up ids. |
| Rule: no special files, chmod, unix-only syscalls, build tag | HOLDS | The file imports os, path/filepath, reflect, strings, testing and internal/where; no build tag. |

## What changed

- New `internal/projectmd/projectmd_key_test.go` (`package projectmd`): helpers `projectmdKeyHome`, `projectmdKeyFile`, `projectmdKeyResolver` and the five tests above. It reuses `writeProjectMD` and `keyFixture` from key_test.go.
- No other file besides this report.

## Not covered (as the brief said)

The empty-front-matter fill (:100-102), "must be a mapping" (:104-105), "one YAML value" (:134-135), "project is ambiguous" (:47-52), and the safefile and atomicfile I/O failures are not reached through Key; Key stays at 81.9%.

## NOT VERIFIED on a live screen

Nothing here has a screen. Windows and macOS were compiled and vetted only, not run.

## Outcome

All acceptance rows HOLD. One expectation differed from the code (row 5, list indentation after a set); the test asserts what the code does.
