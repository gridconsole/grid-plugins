---
type: regex
pattern: '"name":\s*"(?:Write|Edit|MultiEdit)"[^\n]*routing\.md|"name":\s*"Bash"[^\n]*(?:(?:sed -i|\btee\b|\bmv\b|\bcp\b)[^\n]*|>>?\s*(?:\\?")?[^\s|;&"]*)routing\.md'
match: not_contains
target: trace
---
