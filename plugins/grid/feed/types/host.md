---
type: host
version: 1
who: host
required: []
fields:
  - online: true while the Host takes work; a host post older than 3 minutes counts offline
  - grant: true when the Host lets this task use it
  - repos: object of repo name to the head sha the Host has
  - slots: { total, free } Worker slots
  - usage_left: how much of the subscription window is left, as a fraction from 0 to 1
---
A Host announces itself. The latest host post of each Host is the one that counts.

Orchestrator: picks a Host for a Job from these posts (hosts.mjs): online, granted and has the Job's repo, then the one that already has the base commit, then most free slots, then most usage left, then name order. With none eligible the Job stays ready, with reason host_offline, or worker_cap when every eligible slot is full.
