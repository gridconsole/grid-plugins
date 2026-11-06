---
name: visual-design-review
description: Review a built page, screen or component on screenshots of the rendered page before the work is called done. Use after building or changing any UI, a plan page, a task view or a prototype, or when the person says "review the design", "check how it looks", "is this page right".
---

<!-- Decision source: owner proposal, after several delivered designs repeated the same visual mistakes. -->

# Visual design review

Use this after building or changing any page, screen or component, before calling the work done. The review is about what a person sees, so it is done on screenshots of the rendered page, never by rereading the code.

## Steps

1. **Render and capture.** Open the page with whatever browser or screenshot tool is available (Playwright, headless Chrome over CDP). Capture full-page screenshots at three widths: wide desktop (about 1920px), laptop (about 1280px) and phone (about 390px), in both themes the page supports. Capture the important states too: empty, loading, error, read-only, a dialog or panel open.
2. **Load project rules.** Read the project's design rules: in this repo `design-system/README.md` and the README of each component the page uses; in any project a `DESIGN.md`, `design-notes.md` or similar file. Their rules and known recurring issues are checked as well as the checklist below, and win where they conflict.
3. **Load the decided design and the page's job.** Find what the product has already decided for this page: the plan's layout rules (the app shell, navigation, where a task opens), the rules for colour and state, and the brief's statement of who uses the page and what they come to do or learn. Write the page's job in one sentence before judging it. A page that is tidy but ignores a decided layout, or does its job less clearly than it could, fails the review even when every measured check passes.
4. **Review with fresh eyes.** Hand the screenshots, the original brief and the project rules to a separate subagent that did not build the page. Give it the page's job, the decided layout rules, the checklist below and the output format. It judges only what it can see.
5. **Measure what can be measured.** Run `measure.js` (beside this file) inside the rendered page at each width. It returns contrast ratios from computed colours, font families and sizes, text under 11px, sentences set in monospace, sideways scroll, boxes that clip or scroll their content sideways (a table in its own scroll box, which a page-level check misses) and, at phone width, tap targets under 44 by 44px. Use its numbers instead of estimating from the image, and hand them to the reviewer with the screenshots.
6. **Fix and recheck.** Fix the high and medium findings, capture again, and review once more. Stop after two rounds; list anything left over for the person rather than churning.
7. **Feed back.** If a finding is one this project has had before, add it to the project's design notes file (create `design-notes.md` beside the design system if there is none) so the next build avoids it.

## Checklist

### Decided design and information
- The page uses the decided shell: the navigation, the layout and where things open match the plan's rules. Name each rule checked and whether the page meets it.
- The page's job is done in the first view: what the person came for is the first thing they see, at a size that matches its weight, and they can act on it there.
- Every piece of information on the page earns its place for that job. Secondary facts (system load, metadata, ids) are smaller or one click away; nothing the person needs for the job is missing or hidden.
- Each fact is shown in its clearest form: a count as a number, a trend as a chart, a sequence as a timeline, a choice as buttons, a comparison as a table, an explanation as a sentence. Long walls of same-weight cards are a sign the form is wrong.
- Things that differ look different: urgent from routine, own work from other people's, one project from another. If every item has the same weight and colour, the person has to read everything to find anything.
- The page looks designed, not merely arranged: a clear focal point, deliberate rhythm and contrast between sections, and colour used to carry meaning. A correct but flat page is a finding.
- Notes for the designer (open questions, decision ids) do not crowd the product view.

### Scanning and hierarchy
- Squint test: blur the screenshot. Is the most important thing still the most visible? Is there one clear focal point, not several competing ones?
- Text-heavy areas follow the F-pattern: key information top-left, headings and labels front-loaded with the meaningful words.
- Sparse layouts (landing pages, empty states) follow a Z-pattern: title top-left, primary action reachable along the scan path.
- The primary action is visually distinct and there is only one per area. Secondary actions look secondary.
- Headings tell the reader the takeaway, not just the topic.

### Content and placement
- Each fact, status or request appears once. Other places link to it rather than restating it.
- Actions sit next to the thing they act on, not across the screen from it.
- Related items are grouped by proximity; unrelated items are separated by space.
- When there are only a few options, they are visible without an extra click.
- Labels and titles are true to the current state (nothing says "working" while idle, "0 failed" when something failed).
- Numbers and times agree everywhere they appear: legends, cards, badges, summaries, the clock. If two numbers measure different things, each is labelled.
- No wasted width: lines of text, and the controls that go with them, are not stretched across very wide screens.

### Contrast and readability
- Body text meets WCAG AA: at least 4.5:1 against its background. Large text (24px and up, or 18.66px bold) and essential UI elements and icons: at least 3:1.
- Dark themes: light text on a dark background, avoiding pure #000 behind pure #FFF; muted and secondary text still pass the ratios above.
- Disabled elements are clearly disabled but still legible, and the reason is shown where it is not obvious.

### Typography
- At most two type families. Monospace is used only for code, paths, ids and tabular numbers, never for sentences.
- A clear, limited type scale; the same role always gets the same size and weight. In this repo that is the six steps `--t-page` to `--t-micro`.
- Body text at least 14px on desktop and 16px on phone; small labels no smaller than 11 to 12px.
- Line length for reading text roughly 45 to 75 characters; line height about 1.4 to 1.6 for body text.

### Color
- Each color has one meaning across the whole page (for example amber = needs you). It is not reused decoratively for something else.
- Any legend matches what is on screen: same colors, same counts.
- Color is never the only signal; pair it with text, an icon or a shape.
- Accent and status colors are used sparingly so they keep their pull.

### Consistency and layout
- The same component looks and behaves the same everywhere (buttons, cards, chips, spacing).
- Spacing follows a consistent scale; alignment lines are shared across sections.
- At phone width: no horizontal scroll, nothing clipped, tap targets at least 44 by 44px, the order still makes sense.

## Output format

Return at most 10 findings, most important first. For each:

- **Severity:** high (misleads or blocks the person), medium (slows them down or looks broken), low (polish)
- **Where:** the area of the page and which screenshot
- **What is wrong:** one sentence
- **Why it matters:** one sentence tied to the standard
- **Fix:** one concrete change

End with one or two sentences on what already works well, so fixes do not undo it. Do not pad the list; if there are only three real issues, report three.
