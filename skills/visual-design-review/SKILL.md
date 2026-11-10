---
name: visual-design-review
description: Review a built page, screen or component on screenshots of the rendered page before the work is called done. Use after building or changing any UI, or when the person says "review the design", "check how it looks", "is this page right".
---

The review is about what a person sees, so it is done on screenshots of the rendered page, never by rereading the code. Do it after every UI change, before the Job result is posted. How to drive the browser, read-only versus state-changing work, and the NOT VERIFIED rule are in the `browser` skill.

## Steps

1. **Render and capture.** Open the page in a browser. Capture full-page screenshots at widths 390, 1440 and 3840, in each theme the page supports, plus the important states: empty, loading, error, read-only, a dialog or panel open. File each one: `grid proof add <item> <file> --kind screenshot --note "<width, theme, state>" --by <agent> --task <task>`.
2. **Load project rules.** Read the project's design rules: its `DESIGN.md` or design notes, the README of each component the page uses, and `grid brain search <words>` for decisions and conventions. They are checked with the checklist below and win where they conflict.
3. **Load the decided design and the page's job.** Find what is already decided for this page (the app shell, navigation, where a task opens, colour and state rules) and the Job's statement of who uses the page and what they come to do. Write the page's job in one sentence. A page that ignores a decided layout, or does its job less clearly than it could, fails even when every measured check passes.
4. **Measure.** Run `measure.js` (beside this file) inside the rendered page at each width. It returns contrast ratios from computed colours, font families and sizes, text under 11px, sentences in monospace, sideways scroll, boxes that clip or scroll sideways, and, at 390, tap targets under 44 by 44px. Use its numbers instead of estimating from the image. File them: `grid proof add <item> --text "<numbers>" --kind check --by <agent> --task <task>`.
5. **Review with fresh eyes.** Hand the screenshots, the Job (`grid job show <task> <job>`), the project rules, the measurements, the checklist and the output format to a separate subagent that did not build the page. It judges only what it can see.
6. **Fix and recheck.** Fix the high and medium findings, capture again, review once more. Stop after two rounds; list what is left in the Job result, as NOT DONE rows, rather than churning.
7. **Feed back.** A finding this project has had before is proposed to the brain so the next build avoids it: `grid brain propose --topic conventions ...` (see the `brain-keeper` skill, including the link-with-a-why rule).

## Checklist

### Decided design and information
- The page uses the decided shell: navigation, layout and where things open match the rules. Name each rule checked and whether the page meets it.
- Every field on the page names its data source (the served op answer, `frame/model.mjs`, the decision doc). A field whose source cannot be named, or whose source lacks the value, is a finding, never a guess.
- The page's job is done in the first view: what the person came for is first, sized to its weight, and they can act on it there.
- Every piece of information earns its place for that job. Secondary facts (system load, metadata, ids) are smaller or one click away; nothing the person needs is missing or hidden.
- Each fact is in its clearest form: a count as a number, a trend as a chart, a sequence as a timeline, a choice as buttons, a comparison as a table, an explanation as a sentence. A long wall of same-weight cards means the form is wrong.
- Things that differ look different: urgent from routine, own work from other people's, one project from another.
- The page looks designed, not merely arranged: a clear focal point, rhythm and contrast between sections, colour that carries meaning. A correct but flat page is a finding.
- Notes for the designer (open questions, decision ids) do not crowd the product view.

### Scanning and hierarchy
- Squint test: blur the screenshot. Is the most important thing still the most visible, with one focal point?
- Text-heavy areas: key information top-left, headings and labels start with the meaningful words. Sparse layouts: title top-left, primary action along the scan path.
- One primary action per area, visually distinct; secondary actions look secondary.
- Headings tell the takeaway, not just the topic.

### Content and placement
- Each fact, status or request appears once; other places link to it.
- Actions sit next to the thing they act on. Related items are grouped by proximity.
- With only a few options, all are visible without an extra click.
- Labels are true to the current state (nothing says "working" while idle, "0 failed" when something failed).
- Numbers and times agree everywhere. If two numbers measure different things, each is labelled.
- No wasted width: lines of text and their controls are not stretched across a 3840 screen.

### Contrast and readability
- Body text at least 4.5:1 against its background. Large text (24px and up, or 18.66px bold) and essential UI elements and icons at least 3:1.
- Dark themes: light text on dark, not pure #000 behind pure #FFF; muted text still passes the ratios.
- Disabled elements are clearly disabled but legible, and the reason is shown where it is not obvious.

### Typography
- At most two type families. Monospace only for code, paths, ids and tabular numbers.
- A limited type scale; the same role gets the same size and weight.
- Body text at least 14px on desktop and 16px on phone; small labels no smaller than 11 to 12px.
- Reading lines of 45 to 75 characters; line height 1.4 to 1.6 for body text.

### Color
- Each colour has one meaning across the page (for example amber means needs a person). Never reused decoratively.
- A legend matches what is on screen: same colours, same counts.
- Colour is never the only signal; pair it with text, an icon or a shape.
- Accent and status colours are sparing, so they keep their pull.

### Consistency and layout
- The same component looks and behaves the same everywhere. Spacing follows one scale; alignment lines are shared.
- At 390: no horizontal scroll, nothing clipped, tap targets at least 44 by 44px, an order that still makes sense. At 3840 the layout holds its shape.

## Output format

At most 10 findings, most important first. Each has:

- **Severity:** high (misleads or blocks the person), medium (slows them down or looks broken), low (polish)
- **Where:** the area of the page and which screenshot (width, theme, state)
- **What is wrong:** one sentence
- **Why it matters:** one sentence tied to the standard
- **Fix:** one concrete change

End with one or two sentences on what already works, so fixes do not undo it. Do not pad the list.

## Done

Screenshots at 390, 1440 and 3840 and the measurements are filed with `grid proof add`, the fresh-eyes review has run, no high or medium finding is left, and anything not fixed is a NOT DONE row in the Job result.
