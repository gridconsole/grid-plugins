# Plan, rev 1 · 3f9a1c

## Where it stands
Research is done and the plan waits for your approval. Nothing is built yet. Once you approve, step 1 adds the filter to the store.
As of: plan

## How we approach it
Notes already carry tags, so one filter goes in the store, where the list and the coming search both call it.

- **One filter in the store**: it ignores case and is tested on its own.
- **A tag box on the list**: the list narrows as you type.
- **Tests for both**: the filter alone, then the list through it.

| Without | With |
|---|---|
| Every note shows, whatever its tags. | Typing a tag narrows the list. |
| "Home" and "home" are two tags. | Case is ignored. |
| Search would need a filter of its own. | Search reuses the store's filter. |

Figure: views/approach.svg — the list and search both call one filter in the store

## Where things stand now

Notes keep their tags in `src/store.js:3`, but nothing filters by them (research.md, "Store"). The list view renders every note on each keystroke (research.md, "List view").

## Approach and options

- A filter in the store: one place, tested on its own; the list and a later search reuse it.
- A filter in the list view: fewer files, but it can only be tested through the view.

Chosen: the store, because search is the next task and will need the same filter.

## Risks

- medium: tags differ in case, so "Home" misses "home". Retired by step 1.
- low: a long tag list crowds the toolbar. Retired by step 2.

## Steps

- [ ] 1 Add a tag filter function to the store
  The list and search both need it, so it lives in one tested place and ignores case.
  verify: `node --test` passes for the filter
  needs: none
- [ ] 2 Add the tag box to the list view
  People asked to narrow the list as they type; the box calls the store's filter.
  verify: typing narrows the list
  needs: 1
- [ ] 3 Write the release note for the new filter
  Support asked for it; it only needs the decision, not the code.
  verify: the note names the filter and how it treats case
  needs: none
- [ ] 4 Cover both with tests
  verify: `node --test` passes
  needs: 1, 2

Verify: `node --test` passes and the list narrows by tag
