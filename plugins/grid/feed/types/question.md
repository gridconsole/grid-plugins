---
type: question
version: 1
who: any
required: [question]
fields:
  - question: the question, in plain words
  - blocking: true when the Job cannot go on without the answer
  - options: the answers the asker can already see, one per line
---
A question to a person or to the Orchestrator. It stays open until an answer post names it in `re`, or a control post closes it.

Orchestrator: for a blocking question parks the Job and asks the Owner; for another question answers it from the brain when it can, else asks the Owner. Never leaves a question open without saying who has it.
