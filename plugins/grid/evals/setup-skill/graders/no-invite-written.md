---
type: regex
pattern: '"name"\s*:\s*"(?:Write|Edit|MultiEdit)"\s*,\s*"input"\s*:\s*\{\s*"file_path"\s*:\s*"(?:[^"\\]|\\.)*(?:project-roles|invite)|"name"\s*:\s*"Bash"\s*,\s*"input"\s*:\s*\{\s*"command"\s*:\s*"(?=(?:[^"\\]|\\.)*(?:project-roles|invite))(?:[^"\\]|\\.)*(?:(?<!>)>>?(?![>&])(?!\s*/dev/null)|\btee\b|\bcp\b|\bmv\b|\brm\b|\btouch\b|\bsed\s+-i)'
match: not_contains
target: trace
---
