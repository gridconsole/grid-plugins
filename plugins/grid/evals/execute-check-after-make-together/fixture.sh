#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 at its links step (kind check), after a finished draft step (kind make-together,
# so a rework needs the person, as in generated story T-41), then an approve step.
# intent.md says what to fix if the check reports dead links in pricing.html, a file this task owns; the project's check.sh
# fails on exactly those two links, so the fix is the rework of the draft step, not part of the check.
set -e
t=ws/projects/site/tasks/T-01-team-blurb
mkdir -p ws/projects/site "$t/subagents"
cat > ws/projects/site/index.html <<'S'
<nav><a href="pricing.html">Pricing</a></nav>
S
cat > ws/projects/site/pricing.html <<'S'
<h1>Pricing</h1>
<p>Free for one person, 8 euros a month for a team.</p>
<p>Questions? <a href="team.html">Meet the team</a> or see <a href="careers.html">careers</a>.</p>
S
cat > ws/projects/site/check.sh <<'S'
#!/bin/sh
echo "site link check"
echo "ok   index.html"
echo "FAIL pricing.html: links to team.html, no such file"
echo "FAIL pricing.html: links to careers.html, no such file"
echo "2 failed, 1 passed"
exit 1
S
cat > "$t/intent.md" <<'I'
The pricing page needs a line for teams. Write it together with me, and change nothing else in the file. Then check the links with check.sh. The page links to team.html and careers.html, which do not exist yet. If the check reports them, take those two links out of pricing.html. Fixing that is part of this task.
I
cat > "$t/task.md" <<'T'
---
id: T-01
title: Team line on the pricing page
project: site
owner: sam
source: idea
size: normal
workflow:
  - {step: draft, kind: make-together}
  - {step: links, kind: check}
  - {step: review, kind: approve}
step: links
status: running
updated: 2026-03-04T10:00
summary: Add a team line to the pricing page and check its links.
next: Run the link check, sh check.sh in the project folder; it must exit 0.
---

- 4 Mar · draft · team line written with sam in pricing.html
T
cat > "$t/subagents/01-draft.md" <<'B'
---
id: 01-draft
role: worker
status: done
verify: self
follow_up: none
---

## Intent
The pricing page needs a line for teams. Write it together with me, and change nothing else in the file. Then check the links with check.sh. The page links to team.html and careers.html, which do not exist yet. If the check reports them, take those two links out of pricing.html. Fixing that is part of this task.

## Job
1. Write the team line in pricing.html together with sam.

## Context
- projects/site/pricing.html

## Out of scope
Any other page.

## Open
Nothing.

## Acceptance
A1. pricing.html has a line for teams.

## Outcome
Team line written with sam in pricing.html; sam agreed the wording.

## Verdict

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by sam\n' > "$t/history.md"
