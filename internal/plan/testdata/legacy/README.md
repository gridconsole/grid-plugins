# Legacy fixture provenance

The requested `a9aa9a3` object is absent from this checkout. The available
pre-skeleton parent, `124138c^`, supplies the unchanged `form-b.md`,
`form-b-revised.md`, and `form-c.md` inputs. `form-a.md` is the fenced example
at `old-plugin/skills/plan/SKILL.md:22-77` in the reference material.

There are substantive differences between these available historical inputs
and the requested hand-converted outputs:

- `form-b.md` mixes Job fields with an A-style sentence verification in item 4.
  It is an error fixture. `form-b-fields.md` is an explicitly adjusted companion
  using the current fixture's Job verification record in that one item.
- `form-b.expected.md` is an unchanged copy of today's parser fixture, for
  current-format no-op coverage. The pure B and revised B expected files preserve
  the lowercase first word of the original closing verification sentence.
- C's expected output retains the document title and the explicit `item`,
  `title`, `intent.item`, and `intent.goal` fields. The hand conversion removed
  those fields and replaced the title with a placeholder revision heading.
  The converted Job is compared with the old YAML through JSON and LoadPlan.

These differences are recorded rather than silently fabricating historical
fixtures or dropping source content. Exact equality to the hand-converted
fixtures remains unresolved. New hash-corpus entries pin the preserved outputs.
