---
type: heartbeat
version: 1
who: worker
required: []
fields:
  - note: what the Worker is doing, a few words
  - session: the Worker's agent session id
  - model: the model the Worker runs
  - tail: the last 8 lines of the Worker's session, each cut to 200 characters; the beat hook hides the value after KEY=, TOKEN=, SECRET= or PASSWORD=, Bearer tokens and ghp_, gho_, sk- and xox tokens as ***
---
A Worker is alive. Hooks post it after tool calls and when the Worker stops; the post carries `job` and `worker`.

Orchestrator: the fold extends the lease of the Job on each beat from its assigned Worker. A Job whose lease ran out shows as stale: the loop marks it stale once, retries it once as attempt 2 with a new Worker (the old attempt's posts do not count), and if attempt 2 also goes stale posts one question to the Owner and parks the Job. A Worker that exits with no result, or a result with no submit, takes the same path.
