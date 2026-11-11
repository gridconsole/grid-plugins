# C-fix-signal-crlf-notes

A signal note saved with Windows line endings is read, acted on and moved like any other.

Base: plugin `cd84f424` (newer than the brief's `3a3d99ea`). What moved: nothing in the cited causes. `internal/signal/signal.go` `read` still had `HasPrefix(data, "---\n")` (then :181-182) and `Cut(data[4:], "\n---\n")` (then :184), so every Why line still shows its cause.

## Decision

Only the front matter is read with either line ending; the body is never changed. A file that opens with `---\r\n` is a CRLF note. Its closing line is `\r\n---\r\n`. The header between them has `\r\n` turned into `\n` before yaml reads it. The bytes after the closing line are the body, exactly as they are in the file. A file that opens with `---\n` takes the old path. A CRLF opening with no `\r\n---\r\n` closing line is refused with "signal has no closing front matter". A lone `\r` and a BOM stay refused.

## Proof table

| Row | Result | Proof |
|---|---|---|
| 1. Dismiss of a CRLF note: no error, "dismissed", file opens with `---\n` | HOLDS | `go test ./internal/signal/ -run CRLF -count=1 -v` exit 0, `TestCRLFNoteDismiss` PASS. On base (read change stashed, tests kept): `crlf_notes_test.go:40: signal has no front matter` / `--- FAIL: TestCRLFNoteDismiss`. Base sha `cd84f424`. |
| 2. Add on the CRLF note answers "duplicate", same id | HOLDS | `TestCRLFNoteAddIsDuplicate` PASS. Base: `crlf_notes_test.go:60: signal has no front matter`. |
| 3. Move of a CRLF note: same title/source/source_id, body byte for byte | HOLDS, with a change outside Touches (see below) | `TestCRLFNoteMove` PASS. Base: `crlf_notes_test.go:78: signal has no front matter`. With only the `read` change it still failed with `signal has no closing front matter` from `relocateStatus` (move.go). |
| 3b. Body "a\r\nb\r\n" with LF front matter survives Dismiss | HOLDS | `TestBodyCRLFIsData` PASS on lane and on base, as the brief says. |
| 4. Still refused: wrong tier, file name not `<id>.md` ("signal metadata is invalid"); no closing line ("signal has no closing front matter"); over MaxBytes | HOLDS | `TestCRLFNoteStillRefused` PASS (5 subtests, including a CRLF opening closed only by an LF `---` line, which is refused). The over-MaxBytes case passes on base too: the Lstat check at signal.go:174 is unchanged and runs on the raw size, before the read. |
| 5a. `go test ./internal/signal/ -count=1` | HOLDS | `ok ... 0.209s`, exit 0 |
| 5b. `go test ./internal/cli/ -run Signal -count=1` | HOLDS | I ran the whole package: `go test ./internal/cli/ -count=1`, `ok ... 29.182s`, exit 0 |
| 5c. `make lint` | HOLDS | `go vet ./...`, exit 0 |
| 5d. `GOOS=windows go vet ./internal/signal/` | HOLDS | exit 0. I also vetted `./internal/cli/` for windows: exit 0 |
| Windows and macOS builds | HOLDS | `GOOS=windows go build ./...` exit 0; `GOOS=darwin go build ./...` exit 0 |
| Packages that import the changed package | HOLDS | `go list` (imports and test imports) gives `internal/cli` and `internal/signal`; both tested above |
| gofmt | HOLDS | `gofmt -l internal/signal` prints nothing |
| New tests fail on base | HOLDS | Failing lines in rows 1-3. Row 4's subtests (except over-MaxBytes) fail on base with `signal has no front matter`. |
| No existing test edited; none deleted | HOLDS | `git diff origin/main..lane --stat` shows only `signal.go`, `move.go` and the new `crlf_notes_test.go` plus this doc |
| No guard, gate, cap or limit added on agents | HOLDS | Nothing of the kind in the diff. |
| No attribution lines in commits | HOLDS | Two commits with one-sentence messages. |

## What changed

1. `internal/signal/signal.go` `read` (the brief's change): picks the opening and closing delimiter from the first line, turns `\r\n` into `\n` in a CRLF header only, and takes the body as the raw bytes after the closing line. The metadata checks and MaxBytes check are untouched.
2. `internal/signal/move.go` `relocateStatus` (**outside the brief's Touches line**): `Move` does not use `encode`, as the brief assumed. It patches the status in the raw bytes with `relocateStatus`, which looked only for an LF closing line. A CRLF note whose status changes on move (`new` becomes `placed`, the normal case) failed with "signal has no closing front matter", so row 3 could not hold. The fix turns a `---\r\n` note into LF front matter plus the unchanged body at the top of the function, then the existing offset logic runs unchanged. It is a separate commit, so the orchestrator can drop it if the owner wants move left alone. Row 3 then fails for a CRLF note with status `new`. A move that leaves the status unchanged still returns the original bytes, so such a CRLF note stays CRLF on disk and `read` handles it.
3. `internal/signal/crlf_notes_test.go`: new tests for rows 1-4.

Act and `Apply` need no change. They call `read` and then `encode`, which writes LF front matter plus the body bytes.

## NOT VERIFIED on a live screen

Nothing on a screen here. The app's own notes reader is out of scope and was not checked.

## Open items

- The `move.go` change above is outside the brief's Touches line and needs the orchestrator's yes.
