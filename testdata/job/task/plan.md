# Plan, rev 1 · 000000

- [ ] 1 Document the export format
  Document the export format and column order.
  kind: non-code
  jobs:
    - id: J-1
      role: writer
      skills: []
      review: L0
      risk: []
      max_rounds: 2
      intent:
        owner: intent.md#request
      links:
        - {to: brain:project/export-format, rel: uses, why: "Keep the documented column order stable for readers"}
      keep: [brain:project/export-format]
      decisions: []
      read: [task:research.md#format]
      write: [docs/export.md]
      out_of_scope: [runtime/**]
      acceptance:
        - {id: A1, text: "Document the export format and column order.", traces: item}
      verify: {kind: file-exists, path: docs/export.md}
      budget: {turns_estimate: 8, reads_before_first_edit: 3}
      on_blocked: ask-and-continue
      after: []

## Verify

The export documentation lists the column order.
