# Plan, rev 3 · 000000

## Where it stands

The store filter works. The toolbar and keyboard checks remain.
As of: build

## How we approach it

Use one filter for the list and search so matching stays consistent.
Used: brain:project/conventions/tag-matching

## Risks

Case differences could hide matching notes. Item 1 covers this.

## Steps

- [x] 1 Add a **tag filter** to the store
  Match tags without case sensitivity and reuse the existing index.
  write: [src/store/**, tests/store/**]
  after: []
  review: L1
  risk: [data-format]
  verify: {command: "go test ./internal/store/...", min_tests: 3}
  needs: none
  evidence: checks/R-1/store.txt
- [ ] 2 Add the `tag box` to the list view
  Keep the current selection when the query changes.
  write:
    - src/list/**
    - tests/list/**
  after:
    - J-1
    - {job: J-3, kind: failed, why: "Use the fallback if indexing fails"}
    - gate:Q-7
  review: L2
  risk: [concurrency, cross-platform]
  verify:
    command: go test ./internal/list/...
    min_tests: 2
  needs: 1
- [ ] 4 Write the release note
  Explain the case-insensitive match and keyboard shortcut.
  verify: {command: "go test ./...", min_tests: 1}
  needs: none

Verify: filtering works with the keyboard and preserves the selected note.
