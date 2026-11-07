---
type: host
version: 1
who: host
required: []
fields:
  - online: true while the Host takes work
  - repos: the repos on the Host and their head shas
  - slots: free Worker slots
  - usage_left: how much of the subscription window is left, as a share
  - grants: what the Host lets this task use
---
A Host announces itself. The latest host post of each Host is the one that counts.

Orchestrator: picks a Host for a Job from these posts: online, granted and has the repo first, then the one that already has the base commit, then free slots, then usage left.
