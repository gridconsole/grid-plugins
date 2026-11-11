# A-win-artifact-doctor-open

Windows parity for artifact source opening and support-bundle logs, plus CheckImport tests. Repo: plugin. Branch: lane-A-win-artifact-doctor-open, from main.

## Proof table

| Row / rule | Result | Proof |
|---|---|---|
| 1. CheckImport, any OS: file under limit returns nil and creates nothing | HOLDS | `go test ./internal/artifact/ -count=1` exit 0. internal/artifact/artifact_checkimport_test.go:45-72 lists the task folder before and after (no artifacts folder, then with an artifacts folder and a record) and compares with reflect.DeepEqual (:54, :69). |
| 1. folder returns "source must be a regular file" | HOLDS | artifact_checkimport_test.go:74-79 (exact text match). Runs on this Linux sandbox. On Windows the same text comes from open_windows.go:30-31, see row 3. |
| 1. one byte over MaxBytes returns the too-large error | HOLDS | artifact_checkimport_test.go:81-92: 10 bytes at MaxBytes 10 returns nil; 11 bytes returns `errTooLarge(10).Error()` (:87). |
| 1. MaxBytes 0 falls back to DefaultMaxImportBytes | HOLDS | artifact_checkimport_test.go:95-102 passes a 1-byte file with MaxBytes 0 and -1. The fallback is artifact.go:114-117; a limit of 0 would refuse the file as too large. The comment at :92-94 names those lines. |
| 1. missing path satisfies errors.Is(err, fs.ErrNotExist) | HOLDS | artifact_checkimport_test.go:104-109. |
| 2. `go list` GOOS=windows shows open_windows.go and bundle_open_windows.go, not the _other files | HOLDS | `GOOS=windows go list -f '{{.ImportPath}} {{.GoFiles}}' ./internal/artifact/ ./internal/doctor/` exit 0: `artifact [artifact.go open_windows.go]`, `doctor [bundle.go bundle_open_windows.go doctor.go installation.go redact.go]`. |
| 2. GOOS=linux and GOOS=darwin lists unchanged from main | HOLDS | Both exit 0 and both give `artifact [artifact.go open_unix.go]`, `doctor [bundle.go bundle_open_unix.go doctor.go installation.go redact.go]`. Extra: `GOOS=plan9` still selects `open_other.go` and `bundle_open_other.go`. |
| 3. Windows artifact test: file symlink, junction, plain folder, plain file | HOLDS (compiled, file:line) | internal/artifact/artifact_windows_test.go: symlink :37-44 (skip with reason :41); junction :46-57 (made with `cmd /c mklink /J` :52, skip with reason :54); plain folder :59-61 ("source must be a regular file"); plain file :63-79 (CheckImport and Import succeed, file present). Import and CheckImport assertions are in refusedWith :26-33 (exact text match, no artifacts folder written). `GOOS=windows go test -c -o /dev/null ./internal/artifact/` exit 0. |
| 4. Windows doctor test: one-link log, hard-linked log, symlink open | HOLDS (compiled, file:line) | internal/doctor/bundle_windows_test.go: one-link log listed by bundleFiles and collected by readBundleTail :26-37; hard-linked log absent under both names and readBundleTail refused with "log is not a single-link regular file" :39-56 (skip with reason :45); openBundleLog (and readBundleTail) on a symlink to a file outside logs returns an error :58-77 (skip with reason :67). `GOOS=windows go test -c -o /dev/null ./internal/doctor/` exit 0. |
| 5. `go test ./internal/artifact/ ./internal/doctor/ -count=1`: 0 fail, no existing test edited | HOLDS | exit 0: `ok .../internal/artifact 0.052s`, `ok .../internal/doctor 2.124s`. `git diff origin/main --stat` lists only new test files (no existing test file changed); TestBundleSkipsHardLinks (bundle_test.go:184) passes unedited. |
| 5. CheckImport above 0% | HOLDS | `go test -cover ./internal/artifact/` 86.1% (main: 77.1%). `go tool cover -func`: `internal/artifact/artifact.go:101:	CheckImport	90.0%`. |
| 6. `GOOS=windows go vet` for both packages | HOLDS | exit 0, no output. |
| 6. `GOOS=windows/darwin go test -c -o /dev/null` for both packages | HOLDS | all four commands exit 0, no output. |
| 6. `make lint` | HOLDS | exit 0; the target is `go vet ./...` and printed only that line. |
| Tests of packages that import the changed ones | HOLDS | `go list` importers of artifact and doctor: internal/cli, internal/packaging, internal/task. `go test ./internal/artifact/ ./internal/doctor/ ./internal/cli/ ./internal/packaging/ ./internal/task/ -count=1` exit 0 (artifact 0.087s, doctor 3.514s, cli 30.756s, packaging 1.723s, task 1.713s). `GOOS=windows` and `GOOS=darwin go vet` of those three packages exit 0. |
| Windows and macOS builds | HOLDS | `GOOS=windows go vet` and the four `go test -c` above, exit 0. |
| New tests fail on base and pass on the lane | NOT DONE for the Windows tests; characterization only for CheckImport | The CheckImport tests cover code that already works on main (a coverage gap), so they pass on base as well; there is no failing line to paste. The Windows tests need a Windows host; the Linux sandbox only compiles them. On base a Windows host would run the symlink import through os.Open and drop every log; I did not observe that here. |
| No guard, gate, cap, hold or blocking hook on agents | HOLDS | Nothing added that limits an agent: the change only opens a file without following a link and reads a link count. Nothing was removed. |
| Commits: small, one sentence, no attribution | HOLDS | Three commits, plus the one for this file; no Co-Authored-By line. |
| Plugin repo public: no personal path, host or email | HOLDS | `git diff origin/main -- . ':!docs' | grep -nE "/home/|/srv/|@[a-z]+\.|gridwork|Co-Authored"` printed nothing (the diff is 9 files, 356 insertions, 6 deletions). |
| Brief constraints: artifact.go and fslock unchanged; bundle.go only at the two call sites | HOLDS | `git diff origin/main --stat` shows artifact.go and fslock untouched; bundle.go has exactly two changed lines: :186 `singleLinkBundleEntry(path, info)` and :235 `singleLinkBundleLog(f, st)`. |

## What changed

- `internal/artifact/open_windows.go` (new, `//go:build windows`): `openSource` calls `fslock.OpenNoFollow(path, os.O_RDONLY, 0)`. `errors.Is(err, syscall.ELOOP)` (:24) returns the unix text "source must be a regular file, not a symlink" (:25). When the open failed and `os.Lstat(path)` says a directory (:30), it returns "source must be a regular file" (:31), the answer a folder gets today.
- `internal/artifact/open_other.go`: tag narrowed with `|| windows`, so it stays for plan9, js and wasip1.
- `internal/doctor/bundle_open_windows.go` (new): `openBundleLog` is `fslock.OpenNoFollow(path, os.O_RDONLY, 0)` (:15). `singleLinkBundleEntry(path, info)` opens the path with `openBundleLog` and reads NumberOfLinks from the handle (:25); any error returns false. `singleLinkBundleLog(f, info)` reads NumberOfLinks from f's handle.
- `internal/doctor/bundle_open_other.go`: tag narrowed with `|| windows`; both functions return false as before.
- `internal/doctor/bundle_open_unix.go`: `singleLinkBundleLog(info)` is now two functions with the same Nlink == 1 check on info, so unix behaviour is unchanged.
- `internal/doctor/bundle.go`: the two call sites only (:186, :235).
- New tests: `artifact_checkimport_test.go`, `artifact_windows_test.go`, `bundle_windows_test.go`.

### The fslock line quoted for the brief

fslock_windows.go refuses a reparse point as the final component in two places, both with `syscall.ELOOP`:

- :152-155, the failed open of a junction: `if errors.Is(err, syscall.ERROR_ACCESS_DENIED) && mode != syscall.CREATE_NEW { if a, e := syscall.GetFileAttributes(name); e == nil && a&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 { err = syscall.ELOOP } }`
- :167-168, the opened handle: `if info.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0 { return closeFail(syscall.ELOOP) }`

open_windows.go:24 maps exactly that error (`errors.Is(err, syscall.ELOOP)`) to the "not a symlink" message.

## NOT VERIFIED on a live screen / host

- Nothing here is a UI row. Everything Windows-specific is compiled, not run: the symlink and junction refusal, the plain-folder answer through the Lstat fallback, NumberOfLinks from the handle, and the one-link and hard-link log cases. A Windows host has to run them.
- Windows host run: for the orchestrator: `go test ./internal/artifact/ ./internal/doctor/ -count=1 -v -run 'Windows|Symlink|Junction|Bundle'`
- Not run: a full `Bundle()` on Windows with logs. The existing bundle tests skip on Windows with "fslock support is Unix-only" (bundle_test.go:59, :185 and others), which is stale now that fslock has a Windows file; I did not edit them. A Windows bundle test that checks `logs/` in the archive would cover the end to end case.

## Notes

- I wrote one scratch coverage profile to /tmp (outside /srv/gridwork) during the cover run; nothing else outside the clone.
