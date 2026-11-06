---
type: regex
pattern: '"name":\s*"(?:Write|Edit|MultiEdit)"[^\n]*artifacts/|"name":\s*"Bash"[^\n]*(?:\b(?:cp|mv|tee|touch|install)\b[^\n]*|>>?\s*(?:\\?")?[^\s|;&"]*)artifacts/'
match: not_contains
target: trace
---
