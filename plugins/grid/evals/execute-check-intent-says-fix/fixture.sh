#!/usr/bin/env bash
# A Grid workspace folder ./ws with task T-01 at its links step (kind check), after a finished write step, then an approve step.
# intent.md says what to fix if the check reports dead links in pricing.html, a file this task owns; the project's check.sh
# fails on exactly those two links, so the fix is the rework of the write step, not part of the check.
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
add a line to the pricing page for teams, then check that pricing.html has no dead links. if the check reports them, take those links out of pricing.html. fixing that is part of this task
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
  - {step: write, kind: run}
  - {step: links, kind: check}
  - {step: review, kind: approve}
step: links
status: running
updated: 2026-03-04T10:00
summary: Add a team line to the pricing page and check its links.
next: Run the link check, sh check.sh in the project folder; it must exit 0.
---

- 4 Mar · write · team line added to pricing.html
T
cat > "$t/subagents/01-write.md" <<'B'
---
id: 01-write
role: worker
status: done
verify: self
follow_up: none
---

## Intent
add a line to the pricing page for teams, then check that pricing.html has no dead links. if the check reports them, take those links out of pricing.html. fixing that is part of this task

## Job
1. Add the team line to pricing.html.

## Context
- projects/site/pricing.html

## Out of scope
Any other page.

## Open
Nothing.

## Acceptance
A1. pricing.html has a line for teams.

## Outcome
Team line added to pricing.html.

## Verdict

## Follow-up
None.
B
printf -- '- 2026-03-04T10:00 · created · by sam\n' > "$t/history.md"
