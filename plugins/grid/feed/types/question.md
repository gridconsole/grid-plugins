---
type: question
version: 1
who: any
required: [question]
fields:
  - question: the question, in plain words
  - blocking: true when the Job cannot go on without the answer
  - who: `owner` when the person to ask is the Owner of the task, as for a wait Job that needs a manual approval
  - options: the answers the asker can already see, one per line
  - who: for the Orchestrator's question, the person it is for (owner)
  - why: for the Orchestrator's question, why the Job failed
  - job: the Job the question is about
---
A question to a person or to the Orchestrator. It stays open until an answer post names it in `re`, or a control post closes it.

Orchestrator: for a blocking question parks the Job and asks the Owner; for another question answers it from the brain when it can, else asks the Owner. Never leaves a question open without saying who has it.

The Orchestrator's own question to the Owner (a Job failed twice) has who: owner, job and why, and the options Retry it, Change the brief, Drop it; an owner or member answer acts on the parked Job, once (an answer with a worker id does not count; "drop" or "don't retry" wins over "retry"). A Job parked because a Worker's blocking question is open goes ready again, as a counted attempt, when that question is answered.
