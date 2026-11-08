---
type: post
version: 1
who: any
required: [ref, kind]
fields:
  - ref: what it is about, `task`, `item:i-<n>` (n is the Job `item` number) or `question:Q-<n>`
  - kind: message, pickup, ask-pickup, ship, close or reopen; every kind but message shows as a one-line event
  - text: at most 4000 characters; a message needs it, a close may carry its reason
  - attachments: paths relative to the task folder, each under attachments/posts/<post id>/ (no absolute path, no `..`)
---
A message or event on the task page's thread. Everyone on the task may write one, as role owner, member, guest, planner or agent; the role comes from the session (GRID_ROLE), never from the body. A guest may not post pickup or ship: the Feed refuses both when written and the fold ignores them. The fold lists the posts that count per `ref` in `by_ref`.

Posts are unsigned: the fold's guest rule trusts the role a line declares, so a line written by hand as `member` counts. The real boundary is the roles file the core checks when it writes a post.

Orchestrator: does nothing with it; the planner reads close, reopen, pickup and ship to set an item's state.
