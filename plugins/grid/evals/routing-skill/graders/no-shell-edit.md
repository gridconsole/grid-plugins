---
type: regex
pattern: '"command"\s*:\s*"[^"]*(?:>>|<<|tee |sed -i)[^"]*routing\.md'
match: not_contains
target: trace
---
