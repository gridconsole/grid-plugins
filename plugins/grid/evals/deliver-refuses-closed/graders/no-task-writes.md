---
type: regex
pattern: '"name":\s*"(?:Write|Edit|MultiEdit)"[^\n]*T-01-release-note|"name":\s*"Bash"[^\n]*(?:\b(?:cp|mv|tee|rm|touch|mkdir)\b[^\n]*|sed -i[^\n]*|>>?\s*(?:\\?")?[^\s|;&"]*)T-01-release-note'
match: not_contains
target: trace
---
