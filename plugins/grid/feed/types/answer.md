---
type: answer
version: 1
who: any
required: [answer]
fields:
  - answer: the answer, in plain words
---
The answer to a question; `re` is the question's post id.

Orchestrator: reads it, closes the question and moves the parked Job on with a control post (running, or ready for a new Worker).
