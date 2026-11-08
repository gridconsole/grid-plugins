# Legacy fixture provenance

L44 recorded recovering `form-b.md`, `form-b-revised.md`, and `form-c.md` from
the pre-skeleton parent `124138c^`. Neither that object nor the requested
`a9aa9a3^` is available in this rework checkout; the inherited historical inputs
remain unchanged. `form-a.md` is the fenced example at
`old-plugin/skills/plan/SKILL.md:22-77` in the reference material.

The expected files follow the 8 November owner decisions:

- B's sentence verification remains prose, including beside structured fields.
  Its original closing sentence starts with lowercase `filtering`. Conversion,
  hash, CLI migration and archive diff tests use the real historical input;
  the hand-edited `form-b-fields` companion has been removed.
- C retains the document title and explicit `item`, `title`, `intent.item`, and
  `intent.goal` fields. The converted Job is compared with the old YAML through
  JSON and LoadPlan. Installation adds a revision heading, archives the original
  unnumbered bytes as revision 1 and writes the upgraded plan as revision 2.
- A and C lack recall citations. Preview preserves that absence; installation
  tests supply `Recall: nothing found` explicitly, as an owner would.

Migrated hash-corpus entries are produced by the reference `plan-hash.mjs`.
