---
items:
  - id: J-1
    item: 1
    title: Document the export format
    role: writer
    skills: []
    review: L0
    risk: []
    max_rounds: 2
    intent:
      owner: intent.md#request
      item: plan.md#step-1
      goal: plan.md#verify
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
---
# Export documentation

## Step 1 {#step-1}

Document the export format and column order.

## Verify {#verify}

The export documentation lists the column order.
