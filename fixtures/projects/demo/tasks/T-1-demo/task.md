---
id: T-1
title: Demo task
project: demo
owner: member
source: idea
size: normal
workflow:
  - {step: plan, kind: run}
  - {step: plan-review, kind: approve}
  - {step: build, kind: run}
  - {step: review, kind: approve}
step: plan-review
status: waiting
updated: 2025-01-02T03:04
summary: Build the demo example
next: Await plan acceptance
---
