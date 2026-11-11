# G-brain-skills-setup

Stopped at acceptance row 0. No code was changed.

## Proof table

| Row | Result | Evidence |
|---|---|---|
| 0. D-brain-wire is on main | NOT DONE | `git log --oneline main \| grep -i brain-wire` printed nothing (exit 1). `git merge-base --is-ancestor origin/lane-D-brain-wire main` exit 1, and the same check against `origin/main` and `HEAD` exit 1. main, origin/main and HEAD are all d8f1a550. The branch `origin/lane-D-brain-wire` exists with head 898b13b4 ("Add the D-brain-wire proof table.") but is not merged. `docs/lanes/` has no D-brain-wire note. |
| 1. Quote design rows and list matching skills | NOT DONE | Brief says to stop when row 0 fails. |
| 2. packaging.Write grid-brain entry plus layout test | NOT DONE | Same. |
| 3. Version/metadata JSON reports both plugin names | NOT DONE | Same. |
| 4. go test ./internal/packaging/... and make lint | NOT DONE | Nothing changed, so nothing to run. |
| New tests fail on base | n/a | No tests added. |
| No guard, gate, cap, hold or blocking hook | HOLDS | Nothing added. |
| Packages that import the changed ones, Windows and macOS builds | n/a | No package changed. |

## What changed

Only this note. No Go code, skills or metadata.

## Open items

- Row 0 needs `lane-D-brain-wire` (head 898b13b4) merged into main. Its commits include e4595cc8 (record proposals in `grid brain`) and a28f2993 (topics loader).
- Once it is merged, rerun this lane from rows 1 to 4. The brief fits in about 50 calls.
- Row 1 also needs `pages/grid-design onboarding.html:1440-1455`. I did not read it, because the stop came first.

## NOT VERIFIED on a live screen

Nothing was built, so there is nothing to see.
