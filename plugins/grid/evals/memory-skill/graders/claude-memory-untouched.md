---
type: regex
pattern: '"(?:file_path|path)"\s*:\s*"(?:[^"\\]|\\.)*(?:MEMORY\.md|\.claude/|CLAUDE\.md)|"command"\s*:\s*"(?=(?:[^"\\]|\\.)*(?:MEMORY\.md|CLAUDE\.md|\.claude/))(?:[^"\\]|\\.)*(?:(?<!>)>>?(?![>&])(?!\s*/dev/null)|\btee\b|\bcp\b|\bmv\b|\brm\b|\btouch\b|\bsed\s+-i)'
match: not_contains
target: trace
---
